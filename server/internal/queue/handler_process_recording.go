// TODO oncoming feature
package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"

	"github.com/livekit/protocol/auth"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// resolveDownloadURL generates a short-lived LK JWT with RoomAdmin grant for embedded
// LK file downloads. The room-scoped grant ensures only this room's files can be accessed.
// Cloud/external LK URLs (pre-signed S3) are returned as-is.
//
// roomName is required for the RoomAdmin grant; pass "" for cloud URLs (no JWT generated).
func resolveDownloadURL(fileURL, lkHost, lkInternalHost, apiKey, apiSecret, roomName string) (string, error) {
	parsed, err := url.Parse(fileURL)
	if err != nil {
		return fileURL, fmt.Errorf("parse file URL: %w", err)
	}

	// Collect LK hosts to match against
	lkHosts := []string{lkHost}
	if lkInternalHost != "" && lkInternalHost != lkHost {
		lkHosts = append(lkHosts, lkInternalHost)
	}

	isEmbedded := false
	for _, host := range lkHosts {
		if host != "" && parsed.Host == host {
			isEmbedded = true
			break
		}
	}

	if !isEmbedded || apiKey == "" || apiSecret == "" {
		// Cloud/external LK or missing creds — URL is likely pre-signed
		return fileURL, nil
	}

	// Generate short-lived JWT with RoomAdmin grant scoped to this room.
	// RoomAdmin is required for LK file server download in v1.45.x.
	// Room-scoping prevents token from accessing other rooms' files.
	// 5-minute TTL limits the damage if token leaked.
	at := auth.NewAccessToken(apiKey, apiSecret)
	at.AddGrant(&auth.VideoGrant{RoomAdmin: true, Room: roomName}) //nolint:staticcheck // AddGrant deprecated, only API
	at.SetIdentity("bedrud-download").
		SetValidFor(5 * time.Minute)

	token, err := at.ToJWT()
	if err != nil {
		return fileURL, fmt.Errorf("generate download JWT: %w", err)
	}

	// Append token as query parameter (LK file server convention)
	q := parsed.Query()
	q.Set("access_token", token)
	parsed.RawQuery = q.Encode()

	return parsed.String(), nil
}

// NewProcessRecordingHandler creates a handler that downloads, stores, and
// finalizes a LiveKit Egress recording.
//
// Flow:
//  1. Idempotency check — only process if status is "processing"
//  2. Download file from LK egress URL (with auth header for embedded LK)
//  3. Store via storage.Store (S3 or disk)
//  4. Mark recording as completed with file URL + metadata
//  5. Broadcast "recording ready" system message via LK data channel
//  6. (step 8) Dispatch recording.completed webhook if subscribers exist
func NewProcessRecordingHandler(
	recordingRepo *repository.RecordingRepository,
	webhookRepo *repository.WebhookRepository,
	lkHost, lkInternalHost, lkAPIKey, lkAPISecret string,
	recStore storage.RecordingStore,
) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload ProcessRecordingPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			log.Warn().Err(err).Str("jobID", job.ID).Msg("recording: failed to parse payload")
			return nil
		}

		// 1. Idempotency: only process if current status is "processing"
		rec, err := recordingRepo.GetByEgressID(payload.EgressID)
		if err != nil || rec == nil {
			log.Warn().Str("egressID", payload.EgressID).Msg("recording: not found")
			return nil
		}
		if rec.Status != models.RecordingProcessing {
			log.Warn().Str("egressID", payload.EgressID).Str("status", string(rec.Status)).
				Msg("recording: already processed or failed, skipping")
			return nil
		}

		// 2. Download file from LK egress URL with retries
		tmpFile, err := os.CreateTemp("", "recording-*")
		if err != nil {
			_ = recordingRepo.UpdateError(rec.ID, fmt.Sprintf("temp file creation failed: %v", err))
			return fmt.Errorf("create temp file: %w", err)
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		dlCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()

		// For embedded LK, resolve download URL with short-lived JWT.
		// Cloud/external LK (pre-signed S3 URLs) returned as-is.
		downloadURL, err := resolveDownloadURL(payload.FileURL, lkHost, lkInternalHost, lkAPIKey, lkAPISecret, payload.RoomName)
		if err != nil {
			log.Warn().Err(err).Str("egressID", payload.EgressID).Msg("recording: resolveDownloadURL failed, using original URL")
			downloadURL = payload.FileURL
		}

		// Retry download with square backoff (3 attempts, 1s/4s/9s)
		var written int64
		var downloadOK bool
		for attempt := 0; attempt < 3; attempt++ {
			if attempt > 0 {
				backoff := time.Duration((attempt+1)*(attempt+1)) * time.Second
				log.Info().Str("egressID", payload.EgressID).Int("attempt", attempt+1).Dur("backoff", backoff).
					Msg("recording: retrying download")
				select {
				case <-dlCtx.Done():
					_ = recordingRepo.UpdateError(rec.ID, "download cancelled")
					return dlCtx.Err()
				case <-time.After(backoff):
				}
			}

			req, reqErr := http.NewRequestWithContext(dlCtx, http.MethodGet, downloadURL, nil)
			if reqErr != nil {
				continue
			}

			resp, reqErr := http.DefaultClient.Do(req)
			if reqErr != nil {
				continue
			}

			if resp.StatusCode == http.StatusOK {
				tmpFile.Seek(0, 0)
				tmpFile.Truncate(0)
				written, reqErr = io.Copy(tmpFile, resp.Body)
				resp.Body.Close()
				if reqErr == nil {
					downloadOK = true
					break
				}
			} else {
				resp.Body.Close()
			}
		}

		if !downloadOK {
			_ = recordingRepo.UpdateError(rec.ID, "download failed after 3 retries")
			return fmt.Errorf("download failed after 3 retries")
		}

		tmpFile.Close()

		// 3. Build per-user storage key with timestamps
		var started, completed string
		if rec.StartedAt != nil {
			started = rec.StartedAt.Format("20060102T150405Z")
		}
		if started == "" {
			started = rec.CreatedAt.Format("20060102T150405Z")
		}
		completed = time.Now().Format("20060102T150405Z")
		key := fmt.Sprintf("recordings/%s/%s/%s-%s-%s.mp4",
			rec.CreatedBy, rec.RoomID, rec.ID, started, completed)
		f, err := os.Open(tmpPath)
		if err != nil {
			_ = recordingRepo.UpdateError(rec.ID, fmt.Sprintf("open temp file: %v", err))
			return fmt.Errorf("open temp file: %w", err)
		}
		defer f.Close()

		attachment, err := recStore.Store(ctx, key, f, written)
		if err != nil {
			_ = recordingRepo.UpdateError(rec.ID, fmt.Sprintf("storage write failed: %v", err))
			return fmt.Errorf("storage write failed: %w", err)
		}

		// 5. Update recording as completed
		now := time.Now()
		if err := recordingRepo.UpdateCompleted(rec.ID, attachment.URL, written, payload.DurationMs, now); err != nil {
			log.Warn().Err(err).Str("recordingID", rec.ID).Msg("recording: failed to update completed status")
			return fmt.Errorf("update completed: %w", err)
		}

		// 6. (step 8) Dispatch recording.completed webhook if subscribers exist
		if webhookRepo != nil {
			active, listErr := webhookRepo.ListActive(models.EventRecordingCompleted)
			if listErr != nil {
				log.Warn().Err(listErr).Msg("recording: failed to list active webhooks")
			} else if len(active) > 0 {
				for _, wh := range active {
					whPayload := WebhookPayload{
						URL:    wh.URL,
						Event:  models.EventRecordingCompleted,
						Secret: wh.Secret,
						Body: map[string]any{
							"recordingId":   rec.ID,
							"roomId":        rec.RoomID,
							"roomName":      rec.RoomName,
							"fileURL":       attachment.URL,
							"fileSizeBytes": written,
							"durationMs":    payload.DurationMs,
							"status":        string(rec.Status),
						},
					}
					if enqErr := Enqueue(ctx, db, "dispatch_webhook", whPayload); enqErr != nil {
						log.Warn().Err(enqErr).Str("webhookID", wh.ID).Msg("recording: failed to enqueue webhook dispatch")
					}
				}
			}
		}

		log.Info().Str("egressID", payload.EgressID).Int64("size", written).Int64("durationMs", payload.DurationMs).
			Msg("recording: processed and stored")
		return nil
	}
}

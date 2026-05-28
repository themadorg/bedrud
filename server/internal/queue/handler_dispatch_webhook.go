package queue

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"bedrud/internal/models"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// NewDispatchWebhookHandler creates a handler that delivers outbound webhooks
// via HTTP POST with HMAC-SHA256 signed payload.
//
// Key design decisions:
//   - Single attempt, no retry. Webhooks are advisory — room lifecycle operations
//     must not fail because a webhook endpoint is down.
//   - All failures (network, DNS, timeout, non-2xx) are soft: logged + return nil.
//   - Nil Body is replaced with {} in the envelope to avoid JSON null.
//   - HMAC-SHA256 signature sent in X-Bedrud-Signature header.
func NewDispatchWebhookHandler() Handler {
	httpClient := &http.Client{Timeout: 10 * time.Second}

	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload WebhookPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			log.Warn().Err(err).Str("jobID", job.ID).Msg("webhook: failed to parse payload")
			return nil
		}

		// Build envelope body
		timestamp := time.Now().UTC().Format(time.RFC3339)
		data := payload.Body
		if data == nil {
			data = map[string]any{}
		}
		envelope := map[string]any{
			"event":     payload.Event,
			"timestamp": timestamp,
			"data":      data,
		}
		body, err := json.Marshal(envelope)
		if err != nil {
			log.Warn().Err(err).Str("url", payload.URL).Msg("webhook: failed to marshal envelope")
			return nil
		}

		// HMAC-SHA256 signature
		mac := hmac.New(sha256.New, []byte(payload.Secret))
		mac.Write(body)
		sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		// Build request
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, payload.URL, bytes.NewReader(body))
		if err != nil {
			// Malformed URL — don't retry
			log.Warn().Err(err).Str("url", payload.URL).Str("event", payload.Event).
				Msg("webhook: invalid URL in payload")
			return nil
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Bedrud-Signature", sig)
		req.Header.Set("X-Bedrud-Event", payload.Event)
		req.Header.Set("X-Bedrud-Timestamp", timestamp)

		// Delivery
		resp, err := httpClient.Do(req)
		if err != nil {
			log.Warn().Err(err).Str("url", payload.URL).Str("event", payload.Event).
				Msg("webhook: delivery failed (network error)")
			return nil
		}
		resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			log.Warn().Int("status", resp.StatusCode).Str("url", payload.URL).
				Str("event", payload.Event).Msg("webhook: non-2xx response")
			return nil
		}

		log.Info().Int("status", resp.StatusCode).Str("url", payload.URL).
			Str("event", payload.Event).Msg("webhook: delivered")
		return nil
	}
}

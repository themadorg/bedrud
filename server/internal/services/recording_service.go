// TODO oncoming feature
package services

import (
	"bedrud/config"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
)

var (
	ErrRecordingsDisabled    = errors.New("recordings are disabled on this server")
	ErrRecordingsNotAllowed  = errors.New("recordings are not allowed for this room")
	ErrActiveRecordingExists = errors.New("a recording is already active for this room")
	ErrRoomNotFound          = errors.New("room not found")
	ErrNoActiveRecording     = errors.New("no active recording for this room")
	ErrEgressClientNotReady  = errors.New("egress client not available")
	ErrMaxRecordingsPerRoom  = errors.New("maximum recordings reached for this room")
)

// RecordingService encapsulates all recording business logic.
// Gates: system-level (RecordingsEnabled) → room-level (RecordingsAllowed) →
// idempotency (no active recording) → LK Egress call.
type RecordingService struct {
	settingsRepo  *repository.SettingsRepository
	recordingRepo *repository.RecordingRepository
	roomRepo      *repository.RoomRepository
	egressClient  livekit.Egress
	apiKey        string
	apiSecret     string
}

func NewRecordingService(
	settingsRepo *repository.SettingsRepository,
	recordingRepo *repository.RecordingRepository,
	roomRepo *repository.RoomRepository,
	egressClient livekit.Egress,
	apiKey, apiSecret string,
) *RecordingService {
	return &RecordingService{
		settingsRepo:  settingsRepo,
		recordingRepo: recordingRepo,
		roomRepo:      roomRepo,
		egressClient:  egressClient,
		apiKey:        apiKey,
		apiSecret:     apiSecret,
	}
}

// gateSystem checks if recordings are enabled at the system level.
// This is the belt alongside the middleware's suspenders.
func (s *RecordingService) gateSystem() error {
	settings, err := s.settingsRepo.GetSettings()
	if err != nil {
		return fmt.Errorf("get settings: %w", err)
	}
	if !settings.RecordingsEnabled {
		return ErrRecordingsDisabled
	}
	return nil
}

// gateRoom checks if recordings are allowed for the specific room.
func (s *RecordingService) gateRoom(room *models.Room) error {
	if !room.Settings.RecordingsAllowed {
		return ErrRecordingsNotAllowed
	}
	return nil
}

// egressAuthContext wraps a context with LiveKit API key/secret JWT auth.
// Required for all egress client calls or LiveKit returns "permissions denied".
// RoomRecord + Room-scoped grant is needed to start/stop egress in LiveKit.
// Without Room set, the LiveKit server cannot verify room-level permission.
func (s *RecordingService) egressAuthContext(ctx context.Context, roomName string) (context.Context, error) {
	return lkutil.AuthContext(ctx, s.apiKey, s.apiSecret, &lkauth.VideoGrant{
		RoomRecord: true,
		RoomJoin:   true,
		Room:       roomName,
	})
}

// StartRecording runs the full gate chain and starts a recording.
// Returns the recording ID and any error.
func (s *RecordingService) StartRecording(ctx context.Context, roomID, createdBy string) (string, error) {
	// Gate 1: system-level
	if err := s.gateSystem(); err != nil {
		return "", err
	}

	// Gate 2: room exists
	room, err := s.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return "", ErrRoomNotFound
	}

	// Gate 3: room-level permission
	if err := s.gateRoom(room); err != nil {
		return "", err
	}

	// Gate 4: recording limit per room (non-persistent rooms)
	if !room.Settings.IsPersistent {
		limit := config.Get().Recording.MaxRecordingsPerRoom
		if limit > 0 {
			count, err := s.recordingRepo.CountByRoom(roomID)
			if err != nil {
				return "", fmt.Errorf("count recordings: %w", err)
			}
			if count >= int64(limit) {
				return "", ErrMaxRecordingsPerRoom
			}
		}
	}

	// Gate 5: idempotency (no active recording)
	active, err := s.recordingRepo.HasActiveRecording(roomID)
	if err != nil {
		return "", fmt.Errorf("check active recording: %w", err)
	}
	if active {
		return "", ErrActiveRecordingExists
	}

	// Create pending record
	now := time.Now()
	recordingID := uuid.New().String()
	rec := &models.Recording{
		ID:            recordingID,
		RoomID:        roomID,
		RoomName:      room.Name,
		RecordingType: models.RecordingTypeComposite,
		Status:        models.RecordingPending,
		CreatedBy:     createdBy,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := s.recordingRepo.Create(rec); err != nil {
		return "", fmt.Errorf("create recording: %w", err)
	}

	// Check egress client
	if s.egressClient == nil {
		_ = s.recordingRepo.DeleteRecording(recordingID)
		return "", ErrEgressClientNotReady
	}

	// Call LK Egress API with auth context
	egressCtx, err := s.egressAuthContext(ctx, room.Name)
	if err != nil {
		_ = s.recordingRepo.DeleteRecording(recordingID)
		return "", fmt.Errorf("egress auth: %w", err)
	}
	egressReq := &livekit.RoomCompositeEgressRequest{
		RoomName: room.Name,
		Output: &livekit.RoomCompositeEgressRequest_File{
			File: &livekit.EncodedFileOutput{
				FileType: livekit.EncodedFileType_MP4,
				Filepath: fmt.Sprintf("recordings/%s/%s/%s", createdBy, roomID, recordingID),
			},
		},
		// AudioOnly=true → mixed room audio output. Correct for voice-only rooms.
		// Don't set layout/custom_base_url with AudioOnly — those force video pipeline.
		// Future: detect room tracks, stop egress + restart with AudioOnly=false when
		// cameras appear (one egress can't toggle mid-run).
		AudioOnly: true,
	}

	egressResp, err := s.egressClient.StartRoomCompositeEgress(egressCtx, egressReq)
	if err != nil {
		log.Error().Err(err).Str("roomID", roomID).Msg("RecordingService: LK Egress start failed")
		_ = s.recordingRepo.DeleteRecording(recordingID)
		return "", fmt.Errorf("egress start: %w", err)
	}

	log.Info().Str("egressID", egressResp.EgressId).Str("roomID", roomID).Str("roomName", room.Name).Str("createdBy", createdBy).Msg("RecordingService: LK Egress started")

	// Optimistic lock: only transitions from pending
	if err := s.recordingRepo.UpdateEgressID(recordingID, egressResp.EgressId, models.RecordingStarted); err != nil {
		log.Error().Err(err).Str("recordingID", recordingID).Msg("RecordingService: failed to update egress ID")
		// Recording is running in LK but we couldn't update DB — orphan risk.
		// Best effort: try to stop the egress.
		if stopErr := s.stopEgress(ctx, egressResp.EgressId); stopErr != nil {
			log.Warn().Err(stopErr).Str("egressID", egressResp.EgressId).Msg("RecordingService: failed to stop orphan egress")
		}
		return "", fmt.Errorf("update egress ID: %w", err)
	}

	return recordingID, nil
}

// StopRecording stops the active recording for a room.
// Only calls StopEgress on LK — the egress_ended webhook handler handles
// status transition (started → processing → completed/failed) and enqueues
// the process_recording job. This avoids a race where StopRecording's
// status update blocks the webhook from processing the recording.
func (s *RecordingService) StopRecording(ctx context.Context, roomID string) (string, error) {
	// Gate 1: system-level
	if err := s.gateSystem(); err != nil {
		return "", err
	}

	// Gate 2: room exists
	room, err := s.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return "", ErrRoomNotFound
	}

	// Find active recording
	active, err := s.recordingRepo.GetActiveByRoom(roomID)
	if err != nil {
		return "", fmt.Errorf("find active recording: %w", err)
	}
	if active == nil {
		return "", ErrNoActiveRecording
	}
	if active.Status != models.RecordingStarted {
		return "", fmt.Errorf("recording is in %s state, cannot stop", active.Status)
	}
	if active.EgressID == "" {
		_ = s.recordingRepo.UpdateError(active.ID, "no egress ID, cannot stop")
		return "", fmt.Errorf("recording has no egress ID")
	}

	if s.egressClient == nil {
		return "", ErrEgressClientNotReady
	}

	// Stop egress with auth context.
	//
	// If StopEgress succeeds: leave status as-is — the egress_ended webhook
	// from LiveKit handles the full lifecycle (started → processing,
	// error check, file URL extraction, process_recording job enqueue).
	// This avoids a race between our status update and the webhook.
	//
	// If StopEgress fails: mark recording as failed immediately.
	// The webhook may never arrive (e.g. self-signed TLS cert rejected by LK,
	// LK crash, network split), so we must not leave the recording stuck
	// in "started" forever. The scheduler's DeleteStaleRecordings also
	// covers this as a safety net (7-day cutoff).
	//
	// Safety: UpdateStatus uses optimistic locking (WHERE status = started),
	// so if the webhook fires between our check and here, the update is
	// safely rejected — no race.
	egressCtx, err := s.egressAuthContext(ctx, room.Name)
	if err != nil {
		return "", fmt.Errorf("egress auth: %w", err)
	}
	if err := s.stopEgress(egressCtx, active.EgressID); err != nil {
		log.Warn().Err(err).Str("egressID", active.EgressID).Msg("RecordingService: stop egress failed — marking recording as failed")
		if updateErr := s.recordingRepo.UpdateError(active.ID, fmt.Sprintf("stop egress failed: %v", err)); updateErr != nil {
			log.Warn().Err(updateErr).Str("recordingID", active.ID).Msg("RecordingService: failed to mark recording as failed after stop egress error")
		}
		// Also attempt status transition started → failed so scheduler can clean up.
		// Best-effort: may fail if webhook already transitioned it.
		_ = s.recordingRepo.UpdateStatus(active.ID, models.RecordingStarted, models.RecordingFailed)
	} else {
		log.Info().Str("egressID", active.EgressID).Str("roomID", roomID).Str("caller", "RecordingHandler.StopRecording (moderator click)").Msg("RecordingService: stop egress via moderator")
	}

	return active.ID, nil
}

func (s *RecordingService) stopEgress(ctx context.Context, egressID string) error {
	_, err := s.egressClient.StopEgress(ctx, &livekit.StopEgressRequest{
		EgressId: egressID,
	})
	return err
}

// ListRecordings returns paginated recordings for a room.
func (s *RecordingService) ListRecordings(roomID string, page, limit int) ([]models.Recording, int64, error) {
	offset := (page - 1) * limit
	return s.recordingRepo.ListByRoomID(roomID, offset, limit)
}

// ListAdminRecordings returns paginated recordings for the admin panel.
// Optional filters: roomID, status, createdAfter, createdBefore (ISO 8601).
func (s *RecordingService) ListAdminRecordings(page, limit int, roomID, status string, createdAfter, createdBefore *time.Time) ([]models.Recording, int64, error) {
	offset := (page - 1) * limit
	return s.recordingRepo.ListAdmin(offset, limit, roomID, status, createdAfter, createdBefore)
}

// GetRecording returns a single recording.
func (s *RecordingService) GetRecording(recordingID string) (*models.Recording, error) {
	return s.recordingRepo.GetByID(recordingID)
}

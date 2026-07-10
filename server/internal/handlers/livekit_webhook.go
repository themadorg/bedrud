// TODO oncoming feature: recording/egress webhook handling
package handlers

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"

	"github.com/gofiber/fiber/v2"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// LiveKitWebhookHandler receives webhooks from LiveKit Server/Cloud.
// Authentication is via LiveKit's JWT signature validation (same API key/secret
// used for the control plane), not the application's auth middleware.
// Events handled:
//   - participant_left:         marks participant inactive in DB
//   - room_finished:            marks all room participants + room inactive
//   - egress_started:           updates recording status to started
//   - egress_ended:             enqueues process_recording job (or marks failed if egress error)
type LiveKitWebhookHandler struct {
	lkCfg         *config.LiveKitConfig
	roomRepo      *repository.RoomRepository
	recordingRepo *repository.RecordingRepository
	webhookRepo   *repository.WebhookRepository
	db            *gorm.DB
}

func NewLiveKitWebhookHandler(
	lkCfg *config.LiveKitConfig,
	roomRepo *repository.RoomRepository,
	recordingRepo *repository.RecordingRepository,
	webhookRepo *repository.WebhookRepository,
	db *gorm.DB,
) *LiveKitWebhookHandler {
	return &LiveKitWebhookHandler{
		lkCfg:         lkCfg,
		roomRepo:      roomRepo,
		recordingRepo: recordingRepo,
		webhookRepo:   webhookRepo,
		db:            db,
	}
}

// Handle processes an incoming LiveKit webhook.
//
// POST /api/livekit/webhook
// Handle receives LiveKit webhook events (room start/end, participant join/leave, egress events).
// POST /api/livekit/webhook
//
// @Summary LiveKit webhook receiver
// @Description Receive webhook events from LiveKit server (room start/end, participant join/leave, egress). Uses LiveKit JWT signature for auth, not app middleware.
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string "{status: ok}"
// @Failure 400 {object} map[string]string "Invalid signature"
// @Failure 500 {object} map[string]string "Internal error"
// @Router /livekit/webhook [post]
func (h *LiveKitWebhookHandler) Handle(c *fiber.Ctx) error {
	if h.lkCfg == nil || h.lkCfg.APIKey == "" || h.lkCfg.APISecret == "" {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{
			"error": "LiveKit not configured",
		})
	}

	event, err := h.receiveWebhook(c)
	if err != nil {
		log.Warn().Err(err).Msg("LiveKit webhook: invalid signature")
		return c.Status(http.StatusUnauthorized).JSON(fiber.Map{
			"error": "invalid webhook signature",
		})
	}

	ctx := context.Background()

	switch event.Event {
	case "participant_left":
		h.handleParticipantDisconnected(ctx, event)
	case "room_finished":
		h.handleRoomFinished(ctx, event)
	case "egress_started":
		h.handleEgressStarted(ctx, event)
	case "egress_ended":
		h.handleEgressEnded(ctx, event)
	default:
		log.Debug().Str("event", event.Event).Msg("LiveKit webhook: unhandled event")
	}

	return c.Status(http.StatusOK).JSON(fiber.Map{"ok": true})
}

// receiveWebhook reads body + auth header from fasthttp request,
// validates the LiveKit webhook JWT signature, and returns the parsed event.
func (h *LiveKitWebhookHandler) receiveWebhook(c *fiber.Ctx) (*livekit.WebhookEvent, error) {
	body := c.Request().Body()
	if len(body) == 0 {
		return nil, fiber.NewError(http.StatusBadRequest, "empty body")
	}

	authToken := string(c.Request().Header.Peek("Authorization"))
	if authToken == "" {
		return nil, fiber.NewError(http.StatusUnauthorized, "missing Authorization header")
	}

	// Parse and verify JWT token
	v, err := auth.ParseAPIToken(authToken)
	if err != nil {
		return nil, fiber.NewError(http.StatusUnauthorized, "invalid token: "+err.Error())
	}

	provider := auth.NewSimpleKeyProvider(h.lkCfg.APIKey, h.lkCfg.APISecret)
	secret := provider.GetSecret(v.APIKey())
	if secret == "" {
		return nil, fiber.NewError(http.StatusUnauthorized, "unknown API key")
	}

	_, claims, err := v.Verify(secret)
	if err != nil {
		return nil, fiber.NewError(http.StatusUnauthorized, "token verification failed: "+err.Error())
	}

	// Verify SHA256 checksum matches body
	sha := sha256.Sum256(body)
	hash := base64.StdEncoding.EncodeToString(sha[:])
	if subtle.ConstantTimeCompare([]byte(claims.Sha256), []byte(hash)) != 1 {
		return nil, fiber.NewError(http.StatusUnauthorized, "checksum mismatch")
	}

	var event livekit.WebhookEvent
	if err := json.Unmarshal(body, &event); err != nil {
		return nil, fiber.NewError(http.StatusBadRequest, "invalid JSON: "+err.Error())
	}
	return &event, nil
}

func (h *LiveKitWebhookHandler) handleParticipantDisconnected(ctx context.Context, event *livekit.WebhookEvent) {
	if h.roomRepo == nil {
		log.Warn().Msg("LiveKit webhook: roomRepo not initialized")
		return
	}
	if event.Room == nil || event.Participant == nil {
		log.Warn().Msg("LiveKit webhook: participant_disconnected missing room or participant")
		return
	}
	roomName := event.Room.Name
	identity := event.Participant.Identity

	room, err := h.roomRepo.GetRoomByName(roomName)
	if err != nil {
		log.Error().Err(err).Str("room", roomName).Msg("LiveKit webhook: failed to look up room by name")
		return
	}
	if room == nil {
		log.Warn().Str("room", roomName).Msg("LiveKit webhook: room not found in DB")
		return
	}

	if err := h.roomRepo.RemoveParticipant(room.ID, identity); err != nil {
		log.Error().Err(err).Str("room", roomName).Str("user", identity).
			Msg("LiveKit webhook: failed to remove participant")
		return
	}

	log.Info().Str("room", roomName).Str("user", identity).
		Msg("LiveKit webhook: participant marked inactive on disconnect")
}

func (h *LiveKitWebhookHandler) handleRoomFinished(ctx context.Context, event *livekit.WebhookEvent) {
	if h.roomRepo == nil {
		log.Warn().Msg("LiveKit webhook: roomRepo not initialized")
		return
	}
	if event.Room == nil {
		log.Warn().Msg("LiveKit webhook: room_finished missing room")
		return
	}
	roomName := event.Room.Name

	room, err := h.roomRepo.GetRoomByName(roomName)
	if err != nil {
		log.Error().Err(err).Str("room", roomName).Msg("LiveKit webhook: failed to look up room by name")
		return
	}
	if room == nil {
		log.Warn().Str("room", roomName).Msg("LiveKit webhook: room not found in DB")
		return
	}

	// Check if there are active recordings before deactivating the room.
	// If recordings are still processing, skip deactivation to prevent
	// disrupting the recording pipeline.
	if h.recordingRepo != nil {
		hasActive, err := h.recordingRepo.HasActiveRecording(room.ID)
		if err != nil {
			log.Error().Err(err).Str("room", roomName).
				Msg("LiveKit webhook: failed to check active recordings")
		}
		if hasActive {
			log.Info().Str("room", roomName).
				Msg("LiveKit webhook: room has active recordings, skipping deactivation")
			return
		}
	}

	if err := h.roomRepo.RemoveAllParticipants(room.ID); err != nil {
		log.Error().Err(err).Str("room", roomName).
			Msg("LiveKit webhook: failed to remove all participants")
		return
	}

	// Mark room inactive via repo instead of raw gorm.DB
	if err := h.roomRepo.SetRoomIdle(room.ID); err != nil {
		log.Error().Err(err).Str("room", roomName).
			Msg("LiveKit webhook: failed to deactivate room")
	}

	log.Info().Str("room", roomName).Msg("LiveKit webhook: room deactivated on finished")
}

// TODO oncoming feature
// handleEgressStarted updates the recording status to "started" when LK egress begins.
func (h *LiveKitWebhookHandler) handleEgressStarted(ctx context.Context, event *livekit.WebhookEvent) {
	if h.recordingRepo == nil {
		return
	}
	if event.EgressInfo == nil {
		return
	}

	rec, err := h.recordingRepo.GetByEgressID(event.EgressInfo.EgressId)
	if err != nil || rec == nil {
		log.Warn().Str("egressID", event.EgressInfo.EgressId).
			Msg("LiveKit webhook: egress_started for unknown egress")
		return
	}

	if err := h.recordingRepo.UpdateStatus(rec.ID, models.RecordingPending, models.RecordingStarted); err != nil {
		log.Warn().Err(err).Str("recordingID", rec.ID).
			Msg("LiveKit webhook: failed to update recording status to started")
		return
	}

	if err := h.recordingRepo.UpdateStartedAt(rec.ID, time.Now()); err != nil {
		log.Warn().Err(err).Str("recordingID", rec.ID).
			Msg("LiveKit webhook: failed to update recording started_at")
	}

	log.Info().Str("recordingID", rec.ID).Str("egressID", event.EgressInfo.EgressId).
		Msg("LiveKit webhook: recording started")
}

// TODO oncoming feature
// handleEgressEnded enqueues the process_recording job when LK egress completes,
// or marks the recording as failed if the egress finished with an error.
func (h *LiveKitWebhookHandler) handleEgressEnded(ctx context.Context, event *livekit.WebhookEvent) {
	if h.recordingRepo == nil {
		return
	}
	if event.EgressInfo == nil {
		return
	}

	rec, err := h.recordingRepo.GetByEgressID(event.EgressInfo.EgressId)
	if err != nil || rec == nil {
		log.Warn().Str("egressID", event.EgressInfo.EgressId).
			Msg("LiveKit webhook: egress_ended for unknown egress")
		return
	}

	// Optimistic lock: only process if currently started
	err = h.recordingRepo.UpdateStatus(rec.ID, models.RecordingStarted, models.RecordingProcessing)
	if err != nil {
		log.Warn().Err(err).Str("egressID", event.EgressInfo.EgressId).
			Msg("LiveKit webhook: recording status already transitioned (duplicate egress_ended?)")
		return
	}

	// Check if egress failed — LK reports failure via egress_ended with error status
	if event.EgressInfo.Status == livekit.EgressStatus_EGRESS_FAILED {
		errMsg := event.EgressInfo.Error
		if errMsg == "" {
			errMsg = "egress failed (no error details)"
		}
		_ = h.recordingRepo.UpdateError(rec.ID, errMsg)
		log.Warn().Str("recordingID", rec.ID).Str("egressID", event.EgressInfo.EgressId).
			Str("error", errMsg).Msg("LiveKit webhook: recording failed")
		return
	}

	// Extract file URL from egress info
	fileURL := extractFileURL(event.EgressInfo)
	if fileURL == "" {
		log.Warn().Str("egressID", event.EgressInfo.EgressId).
			Msg("LiveKit webhook: no file URL in egress_ended event")
		_ = h.recordingRepo.UpdateError(rec.ID, "no file URL in egress_ended event")
		return
	}

	var fileSize int64
	if len(event.EgressInfo.FileResults) > 0 {
		fileSize = event.EgressInfo.FileResults[0].Size
	}
	var durationMs int64
	if len(event.EgressInfo.FileResults) > 0 {
		durationMs = event.EgressInfo.FileResults[0].Duration / 1e6
	}

	var startedAt string
	if rec.StartedAt != nil {
		startedAt = rec.StartedAt.Format(time.RFC3339)
	}
	payload := queue.ProcessRecordingPayload{
		RoomID:        rec.RoomID,
		RoomName:      rec.RoomName,
		EgressID:      rec.EgressID,
		FileURL:       fileURL,
		FileSize:      fileSize,
		RecordingType: rec.RecordingType,
		DurationMs:    durationMs,
		StartedAt:     startedAt,
	}

	if err := queue.Enqueue(ctx, h.db, "process_recording", payload); err != nil {
		log.Error().Err(err).Str("egressID", event.EgressInfo.EgressId).
			Msg("LiveKit webhook: failed to enqueue process_recording")
		_ = h.recordingRepo.UpdateError(rec.ID, fmt.Sprintf("failed to enqueue: %v", err))
		return
	}

	log.Info().Str("recordingID", rec.ID).Str("egressID", event.EgressInfo.EgressId).
		Msg("LiveKit webhook: recording processing enqueued")
}

// TODO oncoming feature
// extractFileURL returns the file URL from an egress info struct.
// The file URL is in FileResults[0].Filename or in the Result oneof File field.
func extractFileURL(info *livekit.EgressInfo) string {
	if info == nil {
		return ""
	}
	if len(info.FileResults) > 0 && info.FileResults[0].Filename != "" {
		return info.FileResults[0].Filename
	}
	return ""
}

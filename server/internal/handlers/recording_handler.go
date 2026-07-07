// TODO oncoming feature
package handlers

import (
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

// RecordingDTO is the API response type for recordings.
// Computed fields like DownloadStatus are added at serialization time.
type RecordingDTO struct {
	ID             string `json:"id"`
	RecordingType  string `json:"recordingType"`
	DurationMs     int64  `json:"durationMs"`
	FileSize       int64  `json:"fileSize"`
	FileURL        string `json:"fileUrl,omitempty"`
	Status         string `json:"status"`
	Error          string `json:"error,omitempty"`
	DownloadStatus string `json:"downloadStatus"`
	RoomID         string `json:"roomId,omitempty"`
	RoomName       string `json:"roomName,omitempty"`
	CreatedBy      string `json:"createdBy,omitempty"`
	CreatedAt      string `json:"createdAt,omitempty"`
}

// computeDownloadStatus determines download availability for a recording.
// Logic:
//   - failed → "failed"
//   - not completed or no file URL → "processing"
//   - completed + fileUrl → "ready"
func computeDownloadStatus(rec *models.Recording) string {
	if rec.Status == models.RecordingFailed {
		return "failed"
	}
	if rec.Status != models.RecordingCompleted || rec.FileURL == "" {
		return "processing"
	}
	return "ready"
}

// recordingToDTO converts a models.Recording to a RecordingDTO.
func recordingToDTO(rec *models.Recording) RecordingDTO {
	dto := RecordingDTO{
		ID:             rec.ID,
		RecordingType:  rec.RecordingType,
		DurationMs:     rec.DurationMs,
		FileSize:       rec.FileSize,
		FileURL:        rec.FileURL,
		Status:         string(rec.Status),
		Error:          rec.Error,
		DownloadStatus: computeDownloadStatus(rec),
		RoomID:         rec.RoomID,
		RoomName:       rec.RoomName,
		CreatedBy:      rec.CreatedBy,
	}
	if !rec.CreatedAt.IsZero() {
		dto.CreatedAt = rec.CreatedAt.Format(time.RFC3339)
	}
	return dto
}

// recordingsToDTOs converts a slice of models.Recording to RecordingDTOs.
func recordingsToDTOs(recs []models.Recording) []RecordingDTO {
	dtos := make([]RecordingDTO, len(recs))
	for i, rec := range recs {
		dtos[i] = recordingToDTO(&rec)
	}
	return dtos
}

// RecordingHandler handles recording API endpoints.
// HTTP-level concerns only: auth extraction, room lookup, moderator check.
// Recording business logic delegated to RecordingService.
type RecordingHandler struct {
	roomRepo         *repository.RoomRepository
	recordingService *services.RecordingService
	recordingRepo    *repository.RecordingRepository
	recordingStore   storage.RecordingStore
}

// NewRecordingHandler creates a new RecordingHandler.
func NewRecordingHandler(roomRepo *repository.RoomRepository, recordingService *services.RecordingService, recordingRepo *repository.RecordingRepository, recStore storage.RecordingStore) *RecordingHandler {
	return &RecordingHandler{
		roomRepo:         roomRepo,
		recordingService: recordingService,
		recordingRepo:    recordingRepo,
		recordingStore:   recStore,
	}
}

// getUserClaims extracts claims from context.
func (h *RecordingHandler) getUserClaims(c *fiber.Ctx) (*auth.Claims, error) {
	raw := c.Locals("user")
	if raw == nil {
		return nil, fmt.Errorf("not authenticated")
	}
	claims, ok := raw.(*auth.Claims)
	if !ok || claims == nil {
		return nil, fmt.Errorf("invalid auth context")
	}
	return claims, nil
}

// canViewRoomRecordings checks whether the authenticated user is allowed to view
// recordings for a given room. Returns consistent 404-spawning results on failure
// to prevent info leakage (room existence vs access rights).
//
// Access granted if:
//   - user is superadmin (global bypass), OR
//   - user is the room admin/owner (works for archived rooms too), OR
//   - user is an active participant in the room (not banned, not historical)
//
// For archived rooms (deleted_at set), only superadmin + room creator can view.
// Participants are deactivated on archive, so the participant check would fail.
//
// Access denied silently — caller must return 404 on false.
func canViewRoomRecordings(claims *auth.Claims, room *models.Room, roomRepo *repository.RoomRepository) bool {
	// Superadmin bypass
	if containsAccess(claims.Accesses, "superadmin") {
		return true
	}

	// Archived room: only creator + superadmin
	if room.DeletedAt != nil {
		return claims.UserID == getRoomAdminID(room)
	}

	// Room admin/owner
	if claims.UserID == getRoomAdminID(room) {
		return true
	}

	// Active participant (not banned, not historical)
	isParticipant, err := roomRepo.IsParticipant(room.ID, claims.UserID)
	if err != nil {
		log.Warn().Err(err).Str("userID", claims.UserID).Str("roomID", room.ID).
			Msg("canViewRoomRecordings: IsParticipant check failed, denying access")
		return false
	}
	return isParticipant
}

// validateUUID returns true if the string is a valid UUID (any version).
func validateUUID(s string) bool {
	_, err := uuid.Parse(s)
	return err == nil
}

// getRoomAdminID resolves the room's admin user ID.
// Room.AdminID takes precedence over Room.CreatedBy as the authoritative admin.
func getRoomAdminID(room *models.Room) string {
	if room.AdminID != "" {
		return room.AdminID
	}
	return room.CreatedBy
}

// StartRecording starts a composite recording for a room.
// POST /api/rooms/:id/recording/start
//
// @Summary Start recording
// @Description Start a composite recording for a room. Moderator access required.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Success 201 {object} map[string]interface{} "Recording started"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Recordings disabled / not allowed / not moderator"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 409 {object} ErrorResponse "Active recording already exists"
// @Failure 500 {object} ErrorResponse "Recording service not available"
// @Router /rooms/{id}/recording/start [post]
func (h *RecordingHandler) StartRecording(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	userID := claims.UserID
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	// Validate room exists
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Moderator check: only creator, admin, or room moderator can start recording
	if !isRoomModerator(claims, getRoomAdminID(room), room.ID, h.roomRepo) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only moderators can start recordings"})
	}

	// Delegate to service
	recordingID, err := h.recordingService.StartRecording(c.Context(), roomID, userID)
	if err != nil {
		switch err {
		case services.ErrRecordingsDisabled:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		case services.ErrRoomNotFound:
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		case services.ErrRecordingsNotAllowed:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		case services.ErrActiveRecordingExists:
			return c.Status(409).JSON(fiber.Map{"error": err.Error()})
		case services.ErrMaxRecordingsPerRoom:
			return c.Status(403).JSON(fiber.Map{"error": err.Error()})
		case services.ErrEgressClientNotReady:
			return c.Status(500).JSON(fiber.Map{"error": "Recording service not available"})
		default:
			log.Error().Err(err).Str("roomID", roomID).Msg("Failed to start recording")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to start recording"})
		}
	}

	return c.Status(201).JSON(fiber.Map{
		"id":     recordingID,
		"status": "started",
		"roomId": roomID,
	})
}

// StopRecording stops the active recording for a room.
// POST /api/rooms/:id/recording/stop
//
// @Summary Stop recording
// @Description Stop the active recording for a room. Moderator access required.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Success 200 {object} map[string]interface{} "Recording stopped, status: processing"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not moderator"
// @Failure 404 {object} ErrorResponse "Room or recording not found"
// @Failure 500 {object} ErrorResponse "Recording service not available"
// @Router /rooms/{id}/recording/stop [post]
func (h *RecordingHandler) StopRecording(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Moderator check
	if !isRoomModerator(claims, getRoomAdminID(room), room.ID, h.roomRepo) {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Only moderators can stop recordings"})
	}

	// Delegate to service
	recordingID, err := h.recordingService.StopRecording(c.Context(), roomID)
	if err != nil {
		switch err {
		case services.ErrRecordingsDisabled:
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
		case services.ErrRoomNotFound:
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		case services.ErrNoActiveRecording:
			return c.Status(404).JSON(fiber.Map{"error": err.Error()})
		case services.ErrEgressClientNotReady:
			return c.Status(500).JSON(fiber.Map{"error": "Recording service not available"})
		default:
			log.Error().Err(err).Str("roomID", roomID).Msg("Failed to stop recording")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to stop recording"})
		}
	}

	return c.JSON(fiber.Map{
		"id":     recordingID,
		"status": "processing",
	})
}

// ListRecordings lists recordings for a room.
// GET /api/rooms/:id/recordings
//
// @Summary List recordings
// @Description List recordings for a room. Participants, room admin, and superadmin can view.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(20) maximum(100)
// @Success 200 {object} map[string]interface{} "{recordings, total, page, limit}"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 500 {object} ErrorResponse "Failed to list recordings"
// @Router /rooms/{id}/recordings [get]
func (h *RecordingHandler) ListRecordings(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 1000 {
		page = 1000
	}
	limit, _ := strconv.Atoi(c.Query("limit", "20"))
	if limit < 1 || limit > 100 {
		limit = 20
	}
	offset := (page - 1) * limit

	// Try room lookup first
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		// Room deleted — check if user created recordings here
		if h.recordingRepo != nil {
			count, countErr := h.recordingRepo.CountByRoomAndCreator(roomID, claims.UserID)
			if countErr == nil && count > 0 {
				recordings, total, listErr := h.recordingRepo.ListByRoomAndCreator(roomID, claims.UserID, offset, limit)
				if listErr != nil {
					log.Error().Err(listErr).Str("roomID", roomID).Msg("Failed to list recordings for deleted room")
					return c.Status(500).JSON(fiber.Map{"error": "Failed to list recordings"})
				}
				return c.JSON(fiber.Map{
					"recordings": recordingsToDTOs(recordings),
					"total":      total,
					"page":       page,
					"limit":      limit,
				})
			}
		}
		log.Warn().Err(err).Str("userID", claims.UserID).Str("roomID", roomID).
			Msg("ListRecordings: room not found and no user recordings")
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	if !canViewRoomRecordings(claims, room, h.roomRepo) {
		// Fallback: check if user created recordings in this room
		if h.recordingRepo != nil {
			count, countErr := h.recordingRepo.CountByRoomAndCreator(roomID, claims.UserID)
			if countErr == nil && count > 0 {
				recordings, total, listErr := h.recordingRepo.ListByRoomAndCreator(roomID, claims.UserID, offset, limit)
				if listErr != nil {
					log.Error().Err(listErr).Str("roomID", roomID).Msg("Failed to list recordings for non-participant")
					return c.Status(500).JSON(fiber.Map{"error": "Failed to list recordings"})
				}
				return c.JSON(fiber.Map{
					"recordings": recordingsToDTOs(recordings),
					"total":      total,
					"page":       page,
					"limit":      limit,
				})
			}
		}
		log.Warn().Str("userID", claims.UserID).Str("roomID", roomID).
			Msg("ListRecordings: access denied — not participant/moderator/admin")
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	recordings, total, err := h.recordingService.ListRecordings(roomID, page, limit)
	if err != nil {
		log.Error().Err(err).Str("roomID", roomID).Msg("Failed to list recordings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list recordings"})
	}

	return c.JSON(fiber.Map{
		"recordings": recordingsToDTOs(recordings),
		"total":      total,
		"page":       page,
		"limit":      limit,
	})
}

// GetRecording returns a single recording's details.
// GET /api/rooms/:id/recordings/:rid
//
// @Summary Get recording
// @Description Get details for a single recording.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Param rid path string true "Recording ID"
// @Success 200 {object} RecordingDTO "Recording details"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Recording not found"
// @Failure 500 {object} ErrorResponse "Failed to get recording"
// @Router /rooms/{id}/recordings/{rid} [get]
func (h *RecordingHandler) GetRecording(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	recordingID := c.Params("rid")
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	// Room lookup + auth gate
	room, roomErr := h.roomRepo.GetRoom(roomID)
	if roomErr != nil || room == nil {
		// Room deleted — check if user is the recording creator
		rec, recErr := h.recordingService.GetRecording(recordingID)
		if recErr != nil {
			if recErr == repository.ErrRecordingNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
			}
			log.Error().Err(recErr).Str("recordingID", recordingID).Msg("GetRecording: failed after room not found")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to get recording"})
		}
		// Recording must belong to this room and be created by this user
		if rec.RoomID != roomID || rec.CreatedBy != claims.UserID {
			log.Warn().Str("userID", claims.UserID).Str("roomID", roomID).Str("recordingID", recordingID).
				Msg("GetRecording: not creator of recording in deleted room")
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}
		return c.JSON(recordingToDTO(rec))
	}

	if !canViewRoomRecordings(claims, room, h.roomRepo) {
		// Fallback: check if user created this recording
		rec, recErr := h.recordingService.GetRecording(recordingID)
		if recErr == nil && rec.CreatedBy == claims.UserID {
			return c.JSON(recordingToDTO(rec))
		}
		log.Warn().Str("userID", claims.UserID).Str("roomID", roomID).Str("recordingID", recordingID).
			Msg("GetRecording: access denied — not participant/moderator/admin")
		return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
	}

	rec, err := h.recordingService.GetRecording(recordingID)
	if err != nil {
		if err == repository.ErrRecordingNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}
		log.Error().Err(err).Str("recordingID", recordingID).Msg("Failed to get recording")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to get recording"})
	}

	return c.JSON(recordingToDTO(rec))
}

// WaitRecordingReady long-polls until the recording egress has started
// or failed. Gives the frontend certainty before showing "Recording" state.
// GET /api/rooms/:id/recordings/:rid/wait
//
// @Summary Wait for recording readiness
// @Description Long-poll endpoint. Blocks up to 15s, returns when egress is confirmed started or failed.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Param rid path string true "Recording ID"
// @Success 200 {object} map[string]interface{} "{status: active}"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Recording not found"
// @Failure 408 {object} ErrorResponse "Timeout waiting for egress"
// @Failure 500 {object} ErrorResponse "Failed to check recording"
// @Router /rooms/{id}/recordings/{rid}/wait [get]
func (h *RecordingHandler) WaitRecordingReady(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	recordingID := c.Params("rid")
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	// Poll up to 15 seconds, checking every 500ms
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		rec, recErr := h.recordingService.GetRecording(recordingID)
		if recErr != nil {
			if recErr == repository.ErrRecordingNotFound {
				return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
			}
			log.Error().Err(recErr).Str("recordingID", recordingID).Msg("WaitRecordingReady: failed")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to check recording"})
		}

		// Check if recording belongs to this room
		if rec.RoomID != roomID {
			return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
		}

		// Only the creator of the recording can wait for it
		if rec.CreatedBy != claims.UserID {
			// Allow room mods/superadmin too
			room, roomErr := h.roomRepo.GetRoom(roomID)
			if roomErr != nil || room == nil || !canViewRoomRecordings(claims, room, h.roomRepo) {
				return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
			}
		}

		switch rec.Status {
		case models.RecordingFailed:
			errMsg := rec.Error
			if errMsg == "" {
				errMsg = "Recording failed to start"
			}
			return c.JSON(fiber.Map{"status": "failed", "error": errMsg})
		case models.RecordingStarted:
			// Confirmed: egress_started webhook has fired (started_at is set)
			if rec.StartedAt != nil {
				return c.JSON(fiber.Map{"status": "active", "id": recordingID})
			}
		case models.RecordingProcessing, models.RecordingCompleted:
			// Already past started — definitely active
			return c.JSON(fiber.Map{"status": "active", "id": recordingID})
		}

		// Not yet confirmed — sleep and retry
		time.Sleep(500 * time.Millisecond)
	}

	return c.Status(408).JSON(fiber.Map{"status": "timeout", "error": "Egress did not start within 15 seconds"})
}

// AdminListRecordings lists all recordings with optional filters.
// GET /api/admin/recordings
//
// @Summary Admin list recordings
// @Description List all recordings with optional filters. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param perPage query int false "Items per page" default(1000) maximum(1000)
// @Param roomId query string false "Filter by room ID" maxlength(36)
// @Param status query string false "Filter by status (completed, processing, failed, deleting)"
// @Param createdAfter query string false "Filter by ISO 8601 timestamp"
// @Param createdBefore query string false "Filter by ISO 8601 timestamp"
// @Success 200 {object} map[string]interface{} "{recordings, total, page, limit}"
// @Failure 400 {object} ErrorResponse "Invalid filter parameters"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Admin access required"
// @Failure 500 {object} ErrorResponse "Failed to list recordings"
// @Router /admin/recordings [get]
func (h *RecordingHandler) AdminListRecordings(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	if !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	if page < 1 {
		page = 1
	}
	if page > 1000 {
		page = 1000
	}
	limit, _ := strconv.Atoi(c.Query("perPage", "1000"))
	if limit < 1 || limit > 1000 {
		limit = 20
	}

	roomID := c.Query("roomId", "")
	if roomID != "" && len(roomID) > 36 {
		roomID = roomID[:36]
	}
	status := c.Query("status", "")

	var createdAfter, createdBefore *time.Time
	if after := c.Query("createdAfter", ""); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid createdAfter: use ISO 8601 (e.g. 2025-01-01T00:00:00Z)"})
		}
		createdAfter = &t
	}
	if before := c.Query("createdBefore", ""); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid createdBefore: use ISO 8601 (e.g. 2025-01-01T00:00:00Z)"})
		}
		createdBefore = &t
	}

	recordings, total, err := h.recordingService.ListAdminRecordings(page, limit, roomID, status, createdAfter, createdBefore)
	if err != nil {
		log.Error().Err(err).Msg("AdminListRecordings: failed to list")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list recordings"})
	}

	return c.JSON(fiber.Map{
		"recordings": recordingsToDTOs(recordings),
		"total":      total,
		"page":       page,
		"limit":      limit,
	})
}

// BulkDeleteRecordings marks recordings as "deleting" and enqueues async deletion jobs.
// POST /api/admin/recordings/bulk/delete
//
// @Summary Bulk delete recordings
// @Description Mark recordings as deleting and enqueue async deletion jobs. Superadmin access required. Max 500 IDs per request.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "Recording IDs to delete"
// @Success 202 {object} BulkResult "Deletion queued"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Admin access required"
// @Failure 500 {object} ErrorResponse "Failed to process"
// @Router /admin/recordings/bulk/delete [post]
func (h *RecordingHandler) BulkDeleteRecordings(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	if !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "Admin access required"})
	}

	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No recording IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 recordings per request"})
	}

	recordings, err := h.recordingRepo.GetRecordingsByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch recordings"})
	}

	recByID := make(map[string]models.Recording, len(recordings))
	for _, r := range recordings {
		recByID[r.ID] = r
	}

	results := make(map[string]BulkItemResult, len(req.IDs))
	processed := 0
	failed := 0

	for _, id := range req.IDs {
		rec, found := recByID[id]
		if !found {
			results[id] = BulkItemResult{Success: false, Error: "recording not found"}
			failed++
			continue
		}

		// Mark as deleting immediately
		if err := h.recordingRepo.MarkDeleting(id); err != nil {
			results[id] = BulkItemResult{Success: false, Name: rec.RoomName, Error: err.Error()}
			failed++
			continue
		}

		payload := queue.RecordingDeletePayload{
			RecordingID: rec.ID,
			RoomID:      rec.RoomID,
			RoomName:    rec.RoomName,
		}
		if err := queue.Enqueue(context.Background(), database.GetDB(), "recording_delete", payload,
			queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
			results[id] = BulkItemResult{Success: false, Name: rec.RoomName, Error: err.Error()}
			failed++
		} else {
			results[id] = BulkItemResult{Success: true, Name: rec.RoomName}
			processed++
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(BulkResult{
		Results:        results,
		TotalProcessed: processed,
		TotalFailed:    failed,
	})
}

// ClearRoomRecordings deletes all recording files + DB records for an archived room.
// Only room creator or superadmin can clear.
// DELETE /api/rooms/:id/recordings
//
// @Summary Clear room recordings
// @Description Delete all recording files and DB records for a room. Room creator or superadmin only.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Success 200 {object} map[string]interface{} "Recordings cleared"
// @Success 207 {object} map[string]interface{} "Cleared with some file deletion failures"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not the room creator"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 500 {object} ErrorResponse "Failed to clear recordings"
// @Router /rooms/{id}/recordings [delete]
func (h *RecordingHandler) ClearRoomRecordings(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Only creator + superadmin can clear recordings
	if claims.UserID != getRoomAdminID(room) && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Only the room creator can clear recordings"})
	}

	// Fetch all recordings for this room
	recordings, _, err := h.recordingRepo.ListByRoomID(roomID, 0, 1000)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch recordings"})
	}

	// Delete files from storage
	ctx := c.Context()
	var deleteErrors []string
	for _, rec := range recordings {
		if rec.FileURL == "" {
			continue
		}
		key := storage.ExtractStorageKey(rec.FileURL)
		if delErr := h.recordingStore.Delete(ctx, key); delErr != nil {
			log.Warn().Err(delErr).Str("recordingID", rec.ID).Msg("ClearRoomRecordings: file delete failed")
			deleteErrors = append(deleteErrors, rec.ID)
		}
	}

	// Delete DB records (all recordings for this room)
	if err := h.recordingRepo.DeleteByRoom(roomID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete recording records"})
	}

	if len(deleteErrors) > 0 {
		return c.Status(207).JSON(fiber.Map{
			"message":      "Recordings cleared with some file deletions failed",
			"failedRecIDs": deleteErrors,
		})
	}
	return c.Status(200).JSON(fiber.Map{"message": "Recordings cleared"})
}

// ClearSingleRecording deletes a single recording file + DB record.
// DELETE /api/rooms/:id/recordings/:recordingId
//
// @Summary Clear single recording
// @Description Delete a single recording file and DB record. Room creator or superadmin only.
// @Tags rooms
// @Accept json
// @Produce json
// @Param id path string true "Room ID"
// @Param recordingId path string true "Recording ID"
// @Success 200 {object} map[string]interface{} "Recording cleared"
// @Failure 400 {object} ErrorResponse "Invalid room ID"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Not the room creator"
// @Failure 404 {object} ErrorResponse "Room or recording not found"
// @Failure 500 {object} ErrorResponse "Failed to clear recording"
// @Router /rooms/{id}/recordings/{recordingId} [delete]
func (h *RecordingHandler) ClearSingleRecording(c *fiber.Ctx) error {
	claims, err := h.getUserClaims(c)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Not authenticated"})
	}
	roomID := c.Params("id")

	if !validateUUID(roomID) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid room ID"})
	}

	recordingID := c.Params("recordingId")

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Only creator + superadmin can clear recordings
	if claims.UserID != getRoomAdminID(room) && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Only the room creator can clear recordings"})
	}

	rec, err := h.recordingRepo.GetByID(recordingID)
	if err != nil || rec == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Recording not found"})
	}

	// Delete file from storage
	if rec.FileURL != "" {
		key := storage.ExtractStorageKey(rec.FileURL)
		if delErr := h.recordingStore.Delete(c.Context(), key); delErr != nil {
			log.Warn().Err(delErr).Str("recordingID", rec.ID).Msg("ClearSingleRecording: file delete failed")
		}
	}

	// Delete DB record
	if err := h.recordingRepo.DeleteRecording(recordingID); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete recording"})
	}

	return c.Status(200).JSON(fiber.Map{"message": "Recording cleared"})
}

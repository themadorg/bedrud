// TODO oncoming feature: recording cleanup in room lifecycle
package services

import (
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"context"
	"fmt"

	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
)

type RoomCleanupService struct {
	roomRepo      *repository.RoomRepository
	recordingRepo *repository.RecordingRepository
	lkClient      livekit.RoomService
	egressClient  livekit.Egress
	apiKey        string
	apiSecret     string
	uploadTracker *storage.ChatUploadTracker
}

func NewRoomCleanupService(
	roomRepo *repository.RoomRepository,
	recordingRepo *repository.RecordingRepository,
	lkClient livekit.RoomService,
	egressClient livekit.Egress,
	apiKey, apiSecret string,
	uploadTracker *storage.ChatUploadTracker,
) *RoomCleanupService {
	return &RoomCleanupService{
		roomRepo:      roomRepo,
		recordingRepo: recordingRepo,
		lkClient:      lkClient,
		egressClient:  egressClient,
		apiKey:        apiKey,
		apiSecret:     apiSecret,
		uploadTracker: uploadTracker,
	}
}

func (s *RoomCleanupService) lkAuthContext(ctx context.Context) context.Context {
	ctx, err := lkutil.AuthContext(ctx, s.apiKey, s.apiSecret, &lkauth.VideoGrant{RoomCreate: true})
	if err != nil {
		log.Error().Err(err).Msg("Failed to create LK auth context (cleanup service)")
	}
	return ctx
}

type CascadeDeleteOptions struct {
	SystemEvent     string
	SystemMessage   string
	DeletedIdentity string
}

func (s *RoomCleanupService) CascadeDeleteRoom(ctx context.Context, room *models.Room, opts CascadeDeleteOptions) error {
	lkCtx := s.lkAuthContext(ctx)

	// Stop any active recordings before deleting the room
	if s.recordingRepo != nil && s.egressClient != nil {
		if err := s.cleanupRecordings(ctx, room.ID, room.Name, "CascadeDeleteRoom"); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("recording cleanup failed during room deletion")
		}
	}

	if opts.DeletedIdentity != "" {
		lkutil.SendSystemMessageWithDeletedIdentity(lkCtx, s.lkClient, room.Name, opts.SystemEvent, opts.SystemMessage, opts.DeletedIdentity)
	} else {
		lkutil.SendSystemMessage(lkCtx, s.lkClient, room.Name, opts.SystemEvent, opts.SystemMessage)
	}

	if _, err := s.lkClient.DeleteRoom(lkCtx, &livekit.DeleteRoomRequest{Room: room.Name}); err != nil {
		log.Warn().Err(err).Str("room", room.Name).Msg("LiveKit DeleteRoom failed during cascade, proceeding with DB cleanup")
	}

	if s.uploadTracker != nil {
		if err := s.uploadTracker.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up chat uploads")
		}
	}

	if s.recordingRepo != nil {
		if err := s.recordingRepo.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up recording DB records")
		}
	}

	if err := s.roomRepo.HardDeleteRoom(room.ID); err != nil {
		return fmt.Errorf("failed to hard-delete room from DB: %w", err)
	}

	return nil
}

// cleanupRecordings stops any active egress for the room and deletes recording files.
// caller identifies the operation that triggered this (e.g. "CascadeDeleteRoom", "SuspendRoom", "ArchiveRoom").
func (s *RoomCleanupService) cleanupRecordings(ctx context.Context, roomID, roomName, caller string) error {
	// Find and stop active egress
	active, err := s.recordingRepo.GetActiveByRoom(roomID)
	if err != nil {
		return fmt.Errorf("get active recording: %w", err)
	}
	if active != nil && active.EgressID != "" && s.egressClient != nil {
		log.Info().Str("egressID", active.EgressID).Str("roomID", roomID).Str("roomName", roomName).Str("caller", caller).Msg("cleanupRecordings: stopping active egress")
		if _, stopErr := s.egressClient.StopEgress(ctx, &livekit.StopEgressRequest{
			EgressId: active.EgressID,
		}); stopErr != nil {
			log.Warn().Err(stopErr).Str("egressID", active.EgressID).Str("roomID", roomID).Str("caller", caller).Msg("cleanupRecordings: stop egress failed")
		}
	}
	return nil
}

func (s *RoomCleanupService) SuspendRoom(ctx context.Context, room *models.Room) error {
	lkCtx := s.lkAuthContext(ctx)

	// Stop any active recordings before suspending
	if s.recordingRepo != nil && s.egressClient != nil {
		if err := s.cleanupRecordings(ctx, room.ID, room.Name, "SuspendRoom"); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("recording cleanup failed during room suspend")
		}
	}

	lkutil.SendSystemMessage(lkCtx, s.lkClient, room.Name, "room_suspended", "This room has been suspended by an administrator")

	if _, err := s.lkClient.DeleteRoom(lkCtx, &livekit.DeleteRoomRequest{Room: room.Name}); err != nil {
		log.Warn().Err(err).Str("room", room.Name).Msg("LiveKit DeleteRoom failed during suspend, proceeding")
	}

	if s.uploadTracker != nil {
		if err := s.uploadTracker.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up chat uploads during suspend")
		}
	}

	if s.recordingRepo != nil {
		if err := s.recordingRepo.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up recording DB records during suspend")
		}
	}

	if err := s.roomRepo.SetRoomIdle(room.ID); err != nil {
		return fmt.Errorf("failed to suspend room: %w", err)
	}

	if err := s.roomRepo.DeactivateRoomParticipants(room.ID); err != nil {
		log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to deactivate participants during suspend")
	}

	return nil
}

// BulkSuspendRooms suspends multiple rooms, collecting per-room errors.
// Non-fatal errors (LK failures) are logged but don't halt the batch.
func (s *RoomCleanupService) BulkSuspendRooms(ctx context.Context, rooms []models.Room) map[string]error {
	errors := make(map[string]error)
	for i := range rooms {
		if err := s.SuspendRoom(ctx, &rooms[i]); err != nil {
			log.Error().Err(err).Str("roomID", rooms[i].ID).Msg("BulkSuspendRooms: room failed")
			errors[rooms[i].ID] = err
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// BulkCloseRooms permanently deletes multiple rooms, collecting per-room errors.
func (s *RoomCleanupService) BulkCloseRooms(ctx context.Context, rooms []models.Room) map[string]error {
	errors := make(map[string]error)
	opts := CascadeDeleteOptions{
		SystemEvent:   "room_closed",
		SystemMessage: "This room has been closed by an administrator",
	}
	for i := range rooms {
		if err := s.CascadeDeleteRoom(ctx, &rooms[i], opts); err != nil {
			log.Error().Err(err).Str("roomID", rooms[i].ID).Msg("BulkCloseRooms: room failed")
			errors[rooms[i].ID] = err
		}
	}
	if len(errors) == 0 {
		return nil
	}
	return errors
}

// ArchiveRoom soft-deletes a room and preserves recordings for retention period.
// Called when user ends meeting. Recordings remain accessible in Archived view.
func (s *RoomCleanupService) ArchiveRoom(ctx context.Context, room *models.Room) error {
	lkCtx := s.lkAuthContext(ctx)

	// 1. Stop active recordings
	if s.recordingRepo != nil && s.egressClient != nil {
		if err := s.cleanupRecordings(ctx, room.ID, room.Name, "ArchiveRoom"); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("recording cleanup failed during room archive")
		}
	}

	// 2. Send system message via LiveKit
	lkutil.SendSystemMessage(lkCtx, s.lkClient, room.Name, "room_archived", "This meeting has ended.")

	// 3. Delete LiveKit room (kill active connections)
	if _, err := s.lkClient.DeleteRoom(lkCtx, &livekit.DeleteRoomRequest{Room: room.Name}); err != nil {
		log.Warn().Err(err).Str("room", room.Name).Msg("LiveKit DeleteRoom failed during archive, proceeding")
	}

	// 4. Delete chat uploads (ephemeral — no reason to keep)
	if s.uploadTracker != nil {
		if err := s.uploadTracker.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up chat uploads during archive")
		}
	}

	// 5. Deactivate all participants
	if err := s.roomRepo.DeactivateRoomParticipants(room.ID); err != nil {
		log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to deactivate participants during archive")
	}

	// 6. Soft-delete the room (sets deleted_at, is_active=false)
	// Recording rows + files are PRESERVED for retention window
	if err := s.roomRepo.SoftDeleteRoom(room.ID); err != nil {
		return fmt.Errorf("failed to archive room: %w", err)
	}

	log.Info().Str("room", room.Name).Str("roomID", room.ID).
		Msg("Room archived — recordings retained")
	return nil
}

func (s *RoomCleanupService) DeleteUserRooms(ctx context.Context, rooms []models.Room, deletedUserID string) error {
	var firstErr error
	for _, r := range rooms {
		opts := CascadeDeleteOptions{
			SystemEvent:     "room_deleted",
			SystemMessage:   "This room has been closed because the owner's account was removed",
			DeletedIdentity: deletedUserID,
		}
		if err := s.CascadeDeleteRoom(ctx, &r, opts); err != nil {
			log.Error().Err(err).Str("roomID", r.ID).Msg("failed to cascade-delete room during user deletion")
			if firstErr == nil {
				firstErr = err
			}
		}
	}
	return firstErr
}

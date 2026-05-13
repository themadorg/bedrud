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
	lkClient      livekit.RoomService
	apiKey        string
	apiSecret     string
	uploadTracker *storage.ChatUploadTracker
}

func NewRoomCleanupService(
	roomRepo *repository.RoomRepository,
	lkClient livekit.RoomService,
	apiKey, apiSecret string,
	uploadTracker *storage.ChatUploadTracker,
) *RoomCleanupService {
	return &RoomCleanupService{
		roomRepo:      roomRepo,
		lkClient:      lkClient,
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

	if err := s.roomRepo.HardDeleteRoom(room.ID); err != nil {
		return fmt.Errorf("failed to hard-delete room from DB: %w", err)
	}

	return nil
}

func (s *RoomCleanupService) SuspendRoom(ctx context.Context, room *models.Room) error {
	lkCtx := s.lkAuthContext(ctx)

	lkutil.SendSystemMessage(lkCtx, s.lkClient, room.Name, "room_suspended", "This room has been suspended by an administrator")

	if _, err := s.lkClient.DeleteRoom(lkCtx, &livekit.DeleteRoomRequest{Room: room.Name}); err != nil {
		log.Warn().Err(err).Str("room", room.Name).Msg("LiveKit DeleteRoom failed during suspend, proceeding")
	}

	if s.uploadTracker != nil {
		if err := s.uploadTracker.DeleteByRoom(room.ID); err != nil {
			log.Warn().Err(err).Str("roomID", room.ID).Msg("failed to clean up chat uploads during suspend")
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

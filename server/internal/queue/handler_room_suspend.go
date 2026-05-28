package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"

	"gorm.io/gorm"
)

// NewRoomSuspendHandler creates a handler that suspends a room.
func NewRoomSuspendHandler(
	cleanupSvc *services.RoomCleanupService,
	roomRepo *repository.RoomRepository,
) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload RoomSuspendPayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal room_suspend payload: %w", err)
		}

		room, err := roomRepo.GetRoom(payload.RoomID)
		if err != nil {
			return fmt.Errorf("fetch room %s: %w", payload.RoomID, err)
		}
		if room == nil {
			return fmt.Errorf("room not found: %s", payload.RoomID)
		}

		return cleanupSvc.SuspendRoom(ctx, room)
	}
}

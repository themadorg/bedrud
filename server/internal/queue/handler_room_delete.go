package queue

import (
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/gorm"
)

// NewRoomDeleteHandler creates a handler that archives or purges a room.
// purge=true: hard-deletes room + recording rows and files.
// purge=false: archives room (soft-delete, recordings preserved).
func NewRoomDeleteHandler(
	cleanupSvc *services.RoomCleanupService,
	roomRepo *repository.RoomRepository,
) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload RoomDeletePayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal room_delete payload: %w", err)
		}

		room, err := roomRepo.GetRoom(payload.RoomID)
		if err != nil {
			return fmt.Errorf("fetch room %s: %w", payload.RoomID, err)
		}
		if room == nil {
			return nil // already gone
		}

		if payload.Purge {
			// Hard-delete: wipes room + recording DB rows + files
			opts := services.CascadeDeleteOptions{
				SystemEvent:     payload.SystemEvent,
				SystemMessage:   payload.SystemMessage,
				DeletedIdentity: payload.DeletedIdentity,
			}
			return cleanupSvc.CascadeDeleteRoom(ctx, room, opts)
		}

		// Archive: soft-delete room, preserve recordings
		return cleanupSvc.ArchiveRoom(ctx, room)
	}
}

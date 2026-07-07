package queue

import (
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"context"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// NewUserDeleteHandler creates a handler that hard-deletes a user with room cleanup.
func NewUserDeleteHandler(
	cleanupSvc *services.RoomCleanupService,
	userRepo *repository.UserRepository,
	passkeyRepo *repository.PasskeyRepository,
	prefsRepo *repository.UserPreferencesRepository,
	roomRepo *repository.RoomRepository,
) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload UserDeletePayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal user_delete payload: %w", err)
		}

		// Fetch rooms for this user (fresh from DB)
		rooms, err := roomRepo.GetRoomsCreatedByUser(payload.UserID)
		if err != nil {
			return fmt.Errorf("fetch rooms for user %s: %w", payload.UserID, err)
		}
		if rooms == nil {
			rooms = []models.Room{}
		}

		// Cascade delete rooms via LiveKit + DB
		if err := cleanupSvc.DeleteUserRooms(ctx, rooms, payload.UserID); err != nil {
			log.Warn().Err(err).Str("userID", payload.UserID).
				Msg("room cleanup had errors, proceeding with user deletion")
		}

		// Clean up auth data
		if err := passkeyRepo.DeleteByUserID(payload.UserID); err != nil {
			return fmt.Errorf("delete passkeys for user %s: %w", payload.UserID, err)
		}
		if err := prefsRepo.DeleteByUserID(payload.UserID); err != nil {
			return fmt.Errorf("delete preferences for user %s: %w", payload.UserID, err)
		}

		// Hard delete the user
		if err := userRepo.DeleteUser(payload.UserID); err != nil {
			return fmt.Errorf("delete user %s: %w", payload.UserID, err)
		}

		log.Info().Str("userID", payload.UserID).Str("email", payload.Email).
			Int("rooms", len(rooms)).Msg("queue: user deleted with room cleanup")
		return nil
	}
}

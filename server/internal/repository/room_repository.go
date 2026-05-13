package repository

import (
	"bedrud/internal/models"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RoomRepository struct {
	db *gorm.DB
}

func NewRoomRepository(db *gorm.DB) *RoomRepository {
	return &RoomRepository{db: db}
}

// CreateRoom creates a new room with default admin permissions for creator.
// If name is empty, a random URL-safe name is generated.
// The name is validated to contain only lowercase letters, numbers, and hyphens.
func (r *RoomRepository) CreateRoom(createdBy, name string, isPublic bool, mode string, maxParticipants int, settings *models.RoomSettings) (*models.Room, error) {
	// Normalize the name: trim whitespace and lowercase
	name = strings.TrimSpace(strings.ToLower(name))

	// Auto-generate name if not provided
	if name == "" {
		generated, err := models.GenerateRandomRoomName()
		if err != nil {
			return nil, errors.New("failed to generate room name")
		}
		name = generated
	}

	// Validate the room name
	if err := models.ValidateRoomName(name); err != nil {
		return nil, err
	}

	var room *models.Room

	err := r.db.Transaction(func(tx *gorm.DB) error {
		// Check for duplicate name inside transaction (TOCTOU-safe)
		var existing models.Room
		if err := tx.Where("name = ?", name).First(&existing).Error; err == nil {
			return models.ErrRoomNameTaken
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now()
		newRoom := &models.Room{
			ID:               uuid.New().String(),
			Name:             name,
			CreatedBy:        createdBy,
			AdminID:          createdBy,
			IsActive:         true,
			IsPublic:         isPublic,
			Settings:         *settings,
			Mode:             mode,
			MaxParticipants:  maxParticipants,
			ExpiresAt:        now.Add(24 * time.Hour),
			CreatedAt:        now,
			UpdatedAt:        now,
		}

		if err := tx.Model(&models.Room{}).Create(map[string]interface{}{
			"ID":                        newRoom.ID,
			"Name":                      newRoom.Name,
			"CreatedBy":                 newRoom.CreatedBy,
			"AdminID":                   newRoom.AdminID,
			"IsActive":                  newRoom.IsActive,
			"IsPublic":                  newRoom.IsPublic,
			"Mode":                      newRoom.Mode,
			"ExpiresAt":                 newRoom.ExpiresAt,
			"MaxParticipants":           newRoom.MaxParticipants,
			"CreatedAt":                 newRoom.CreatedAt,
			"UpdatedAt":                 newRoom.UpdatedAt,
			"settings_allow_chat":       newRoom.Settings.AllowChat,
			"settings_allow_video":      newRoom.Settings.AllowVideo,
			"settings_allow_audio":      newRoom.Settings.AllowAudio,
			"settings_require_approval": newRoom.Settings.RequireApproval,
			"settings_e2_ee":            newRoom.Settings.E2EE,
			"settings_is_persistent":    newRoom.Settings.IsPersistent,
		}).Error; err != nil {
			newRoom = nil
			// Catch unique constraint violations (TOCTOU race safety net)
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "UNIQUE") {
				return models.ErrRoomNameTaken
			}
			return err
		}

		// Create room participant record for the creator
		participant := &models.RoomParticipant{
			ID:         uuid.New().String(),
			RoomID:     newRoom.ID,
			UserID:     createdBy,
			IsActive:   true,
			IsApproved: true, // Creator is automatically approved
			IsOnStage:  true, // Creator is always on stage
		}

		if err := tx.Create(participant).Error; err != nil {
			return err
		}

		// Now create admin permissions
		adminPermissions := &models.RoomPermissions{
			ID:              uuid.New().String(),
			RoomID:          newRoom.ID,
			UserID:          createdBy,
			IsAdmin:         true,
			CanKick:         true,
			CanMuteAudio:    true,
			CanDisableVideo: true,
			CanChat:         true,
		}

		if err := tx.Create(adminPermissions).Error; err != nil {
			return err
		}

		room = newRoom
		return nil
	})
	if err != nil {
		return nil, err
	}

	return room, nil
}

// GetRoom retrieves a room by ID
func (r *RoomRepository) GetRoom(id string) (*models.Room, error) {
	var room models.Room
	result := r.db.First(&room, "id = ?", id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &room, nil
}

// GetRoomByName retrieves a room by name (case-insensitive)
func (r *RoomRepository) GetRoomByName(name string) (*models.Room, error) {
	var room models.Room
	result := r.db.First(&room, "name = ?", strings.ToLower(strings.TrimSpace(name)))
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, result.Error
	}
	return &room, nil
}

// AddParticipant adds a participant to a room or reactivates them if they already exist
func (r *RoomRepository) AddParticipant(roomID, userID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check if participant already exists
		var existing models.RoomParticipant
		err := tx.Where("room_id = ? AND user_id = ?", roomID, userID).First(&existing).Error

		if err == nil {
			// Check if participant is banned
			if existing.IsBanned {
				return errors.New("user is banned from this room")
			}

			// Participant exists, update their status
			return tx.Model(&existing).Updates(map[string]interface{}{
				"is_active": true,
				"left_at":   nil,
				"joined_at": time.Now(),
			}).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			// Unexpected error
			return err
		}

		// Create new participant
		participant := &models.RoomParticipant{
			ID:        uuid.New().String(),
			RoomID:    roomID,
			UserID:    userID,
			IsActive:  true,
			JoinedAt:  time.Now(),
			IsOnStage: false, // Default to audience
		}

		return tx.Create(participant).Error
	})
}

// AddParticipantWithCapacityCheck adds a participant with an atomic capacity check
// inside the transaction. Prevents TOCTOU race between count check and insert.
// Pass maxParticipants=0 to skip capacity limit.
func (r *RoomRepository) AddParticipantWithCapacityCheck(roomID, userID string, maxParticipants int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// Check room capacity inside the transaction
		if maxParticipants > 0 {
			var count int64
			if err := tx.Model(&models.RoomParticipant{}).
				Where("room_id = ? AND is_active = ? AND is_banned = ?", roomID, true, false).
				Count(&count).Error; err != nil {
				return err
			}
			if count >= int64(maxParticipants) {
				return errors.New("room is full")
			}
		}

		// Check if participant already exists
		var existing models.RoomParticipant
		err := tx.Where("room_id = ? AND user_id = ?", roomID, userID).First(&existing).Error

		if err == nil {
			// Check if participant is banned
			if existing.IsBanned {
				return errors.New("user is banned from this room")
			}

			// Participant exists, update their status
			return tx.Model(&existing).Updates(map[string]interface{}{
				"is_active": true,
				"left_at":   nil,
				"joined_at": time.Now(),
			}).Error
		}

		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		// Create new participant
		participant := &models.RoomParticipant{
			ID:        uuid.New().String(),
			RoomID:    roomID,
			UserID:    userID,
			IsActive:  true,
			JoinedAt:  time.Now(),
			IsOnStage: false,
		}
		return tx.Create(participant).Error
	})
}

// RemoveParticipant marks a participant as inactive and sets their leave time
func (r *RoomRepository) RemoveParticipant(roomID, userID string) error {
	now := time.Now()
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ? AND is_active = ?", roomID, userID, true).
		Updates(map[string]interface{}{
			"is_active": false,
			"left_at":   now,
		}).Error
}

// GetActiveParticipants gets all active participants in a room
func (r *RoomRepository) GetActiveParticipants(roomID string) ([]models.RoomParticipant, error) {
	var participants []models.RoomParticipant
	err := r.db.Where("room_id = ? AND is_active = ?", roomID, true).
		Find(&participants).Error
	return participants, err
}

// CleanupExpiredRooms marks rooms as inactive if they've expired.
// Persistent rooms are excluded from this cleanup.
func (r *RoomRepository) CleanupExpiredRooms() error {
	return r.db.Model(&models.Room{}).
		Where("expires_at < ? AND is_active = ? AND settings_is_persistent = ?", time.Now(), true, false).
		Update("is_active", false).Error
}

// UpdateParticipantPermissions updates a participant's permissions
func (r *RoomRepository) UpdateParticipantPermissions(roomID, userID string, updates map[string]interface{}) error {
	return r.db.Model(&models.RoomPermissions{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(updates).Error
}

// BringToStage brings a participant to the stage
func (r *RoomRepository) BringToStage(roomID, userID string) error {
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Update("is_on_stage", true).Error
}

// RemoveFromStage removes a participant from the stage
func (r *RoomRepository) RemoveFromStage(roomID, userID string) error {
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Update("is_on_stage", false).Error
}

// IsParticipantOnStage checks if a participant is on stage
func (r *RoomRepository) IsParticipantOnStage(roomID, userID string) (bool, error) {
	var participant models.RoomParticipant
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&participant).Error
	if err != nil {
		return false, err
	}
	return participant.IsOnStage, nil
}

// GetParticipantPermissions gets a participant's permissions
func (r *RoomRepository) GetParticipantPermissions(roomID, userID string) (*models.RoomPermissions, error) {
	var permissions models.RoomPermissions
	err := r.db.Where("room_id = ? AND user_id = ?", roomID, userID).First(&permissions).Error
	if err != nil {
		return nil, err
	}
	return &permissions, nil
}

// UpdateParticipantStatus updates a participant's status (mute, video, chat)
func (r *RoomRepository) UpdateParticipantStatus(roomID, userID string, updates map[string]interface{}) error {
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(updates).Error
}

// KickParticipant removes a participant from the room
func (r *RoomRepository) KickParticipant(roomID, userID string) error {
	now := time.Now()
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Updates(map[string]interface{}{
			"is_active": false,
			"is_banned": true,
			"left_at":   now,
		}).Error
}

// UpdateRoomSettings updates room global settings
func (r *RoomRepository) UpdateRoomSettings(roomID string, settings *models.RoomSettings) error {
	return r.db.Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"settings_allow_chat":       settings.AllowChat,
			"settings_allow_video":      settings.AllowVideo,
			"settings_allow_audio":      settings.AllowAudio,
			"settings_require_approval": settings.RequireApproval,
			"settings_e2_ee":            settings.E2EE,
			"settings_is_persistent":    settings.IsPersistent,
		}).Error
}

// DeleteRoom deletes a room and its related data. Only the creator can delete.
func (r *RoomRepository) DeleteRoom(roomID, userID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var room models.Room
		if err := tx.Where("id = ? AND created_by = ?", roomID, userID).First(&room).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&models.RoomPermissions{}).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&models.RoomParticipant{}).Error; err != nil {
			return err
		}
		return tx.Delete(&room).Error
	})
}

// HardDeleteRoom deletes a room and its related data without a created_by check.
// Callers must verify authorization before calling this.
func (r *RoomRepository) HardDeleteRoom(roomID string) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("room_id = ?", roomID).Delete(&models.RoomPermissions{}).Error; err != nil {
			return err
		}
		if err := tx.Where("room_id = ?", roomID).Delete(&models.RoomParticipant{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", roomID).Delete(&models.Room{}).Error
	})
}

func (r *RoomRepository) GetAllRooms() ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Find(&rooms).Error
	return rooms, err
}

// GetAllRoomsPaginated returns a paginated list of rooms and the total count.
func (r *RoomRepository) GetAllRoomsPaginated(p PaginationParams) ([]models.Room, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit
	var total int64
	var rooms []models.Room
	if err := r.db.Model(&models.Room{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := r.db.Limit(p.Limit).Offset(offset).Find(&rooms).Error
	return rooms, total, err
}

func (r *RoomRepository) GetAllActiveRooms() ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Where("is_active = ?", true).Limit(1000).Find(&rooms).Error
	return rooms, err
}

func (r *RoomRepository) GetAllActiveRoomsWithLimit(limit int) ([]models.Room, error) {
	var rooms []models.Room
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	err := r.db.Where("is_active = ?", true).Limit(limit).Find(&rooms).Error
	return rooms, err
}

func (r *RoomRepository) SetRoomIdle(roomID string) error {
	return r.db.Model(&models.Room{}).Where("id = ?", roomID).Update("is_active", false).Error
}

func (r *RoomRepository) DeactivateRoomParticipants(roomID string) error {
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND is_active = ?", roomID, true).
		Updates(map[string]interface{}{"is_active": false, "left_at": time.Now()}).Error
}

func (r *RoomRepository) GetRoomParticipantsWithUsers(roomID string) ([]models.RoomParticipant, error) {
	var participants []models.RoomParticipant
	err := r.db.Preload("User").Where("room_id = ? AND is_active = ?", roomID, true).Find(&participants).Error
	return participants, err
}

func (r *RoomRepository) GetUserByID(userID string) (*models.User, error) {
	var user models.User
	err := r.db.Where("id = ?", userID).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetRoomsCreatedByUser retrieves rooms created by a specific user
func (r *RoomRepository) GetRoomsCreatedByUser(userID string) ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Where("created_by = ?", userID).Order("created_at desc").Find(&rooms).Error
	return rooms, err
}

// GetRoomsParticipatedInByUser retrieves rooms a user has participated in (excluding those they created)
func (r *RoomRepository) GetRoomsParticipatedInByUser(userID string) ([]models.Room, error) {
	var rooms []models.Room
	// Find RoomIDs where the user is a participant
	var participantRoomIDs []string
	err := r.db.Model(&models.RoomParticipant{}).
		Where("user_id = ? AND room_id NOT IN (?)", userID, r.db.Model(&models.Room{}).Select("id").Where("created_by = ?", userID)).
		Distinct("room_id").
		Pluck("room_id", &participantRoomIDs).Error
	if err != nil {
		return nil, err
	}

	if len(participantRoomIDs) == 0 {
		return rooms, nil // Return empty slice if no participated rooms
	}

	// Fetch the rooms based on the found IDs
	err = r.db.Where("id IN (?)", participantRoomIDs).Order("created_at desc").Find(&rooms).Error
	return rooms, err
}

func (r *RoomRepository) UpdateRoom(room *models.Room) error {
	return r.db.Save(room).Error
}

func (r *RoomRepository) CountActiveParticipants() (int64, error) {
	var count int64
	err := r.db.Model(&models.RoomParticipant{}).Where("is_active = ?", true).Distinct("user_id").Count(&count).Error
	return count, err
}

// IsParticipantBanned returns true when a participant record exists for the given
// room and user identity with is_banned = true.
func (r *RoomRepository) IsParticipantBanned(roomID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ? AND is_banned = ?", roomID, userID, true).
		Count(&count).Error
	return count > 0, err
}

// IsRoomModerator returns true when the user has is_moderator=true in room_participants
// for this specific room.
func (r *RoomRepository) IsRoomModerator(roomID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ? AND is_moderator = ?", roomID, userID, true).
		Count(&count).Error
	return count > 0, err
}

// SetRoomModerator sets or clears the is_moderator flag for a participant.
func (r *RoomRepository) SetRoomModerator(roomID, userID string, isMod bool) error {
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", roomID, userID).
		Update("is_moderator", isMod).Error
}

// GetParticipantCount returns the number of non-banned participants for a room.
func (r *RoomRepository) GetParticipantCount(roomID string) (int, error) {
	var count int64
	err := r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND is_active = ? AND is_banned = ?", roomID, true, false).
		Count(&count).Error
	return int(count), err
}

// IsParticipant returns true if the user is an active, non-banned participant in the room.
func (r *RoomRepository) IsParticipant(roomID, userID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ? AND is_active = ? AND is_banned = ?", roomID, userID, true, false).
		Count(&count).Error
	return count > 0, err
}

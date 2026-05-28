package repository

import (
	"bedrud/internal/models"
	"errors"
	"fmt"
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
		// Check for duplicate name inside transaction (TOCTOU-safe).
		// Only active rooms block re-creation — idle (inactive but not archived)
		// and archived (soft-deleted) rooms allow name reuse.
		var existing models.Room
		if err := tx.Where("name = ? AND is_active = ?", name, true).First(&existing).Error; err == nil {
			return models.ErrRoomNameTaken
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		now := time.Now()
		newRoom := &models.Room{
			ID:              uuid.New().String(),
			Name:            name,
			CreatedBy:       createdBy,
			AdminID:         createdBy,
			IsActive:        true,
			IsPublic:        isPublic,
			Settings:        *settings,
			Mode:            mode,
			MaxParticipants: maxParticipants,
			ExpiresAt:       now.Add(24 * time.Hour),
			CreatedAt:       now,
			UpdatedAt:       now,
			LastActivityAt:  &now,
		}

		if err := tx.Model(&models.Room{}).Create(map[string]interface{}{
			"ID":                          newRoom.ID,
			"Name":                        newRoom.Name,
			"CreatedBy":                   newRoom.CreatedBy,
			"AdminID":                     newRoom.AdminID,
			"IsActive":                    newRoom.IsActive,
			"IsPublic":                    newRoom.IsPublic,
			"Mode":                        newRoom.Mode,
			"ExpiresAt":                   newRoom.ExpiresAt,
			"MaxParticipants":             newRoom.MaxParticipants,
			"CreatedAt":                   newRoom.CreatedAt,
			"UpdatedAt":                   newRoom.UpdatedAt,
			"LastActivityAt":              now,
			"settings_allow_chat":         newRoom.Settings.AllowChat,
			"settings_allow_video":        newRoom.Settings.AllowVideo,
			"settings_allow_audio":        newRoom.Settings.AllowAudio,
			"settings_require_approval":   newRoom.Settings.RequireApproval,
			"settings_e2_ee":              newRoom.Settings.E2EE,
			"settings_is_persistent":      newRoom.Settings.IsPersistent,
			"settings_recordings_allowed": newRoom.Settings.RecordingsAllowed,
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
		now := time.Now()

		// Check if participant already exists
		var existing models.RoomParticipant
		err := tx.Where("room_id = ? AND user_id = ?", roomID, userID).First(&existing).Error

		if err == nil {
			// Check if participant is banned
			if existing.IsBanned {
				return errors.New("user is banned from this room")
			}

			// Participant exists, update their status
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"is_active": true,
				"left_at":   nil,
				"joined_at": now,
			}).Error; err != nil {
				return err
			}

			// Update room last activity time
			return tx.Model(&models.Room{}).Where("id = ?", roomID).Update("last_activity_at", now).Error
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
			JoinedAt:  now,
			IsOnStage: false, // Default to audience
		}

		if err := tx.Create(participant).Error; err != nil {
			return err
		}

		// Update room last activity time
		return tx.Model(&models.Room{}).Where("id = ?", roomID).Update("last_activity_at", now).Error
	})
}

// AddParticipantWithCapacityCheck adds a participant with an atomic capacity check
// inside the transaction. Prevents TOCTOU race between count check and insert.
// Pass maxParticipants=0 to skip capacity limit.
func (r *RoomRepository) AddParticipantWithCapacityCheck(roomID, userID string, maxParticipants int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		now := time.Now()

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
			if err := tx.Model(&existing).Updates(map[string]interface{}{
				"is_active": true,
				"left_at":   nil,
				"joined_at": now,
			}).Error; err != nil {
				return err
			}

			// Update room last activity time
			return tx.Model(&models.Room{}).Where("id = ?", roomID).Update("last_activity_at", now).Error
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
			JoinedAt:  now,
			IsOnStage: false,
		}

		if err := tx.Create(participant).Error; err != nil {
			return err
		}

		// Update room last activity time
		return tx.Model(&models.Room{}).Where("id = ?", roomID).Update("last_activity_at", now).Error
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

// RemoveAllParticipants marks all active participants in a room as inactive.
// Used when LiveKit sends room_finished webhook.
func (r *RoomRepository) RemoveAllParticipants(roomID string) error {
	now := time.Now()
	return r.db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND is_active = ?", roomID, true).
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
			"settings_allow_chat":         settings.AllowChat,
			"settings_allow_video":        settings.AllowVideo,
			"settings_allow_audio":        settings.AllowAudio,
			"settings_require_approval":   settings.RequireApproval,
			"settings_e2_ee":              settings.E2EE,
			"settings_is_persistent":      settings.IsPersistent,
			"settings_recordings_allowed": settings.RecordingsAllowed,
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
		if err := tx.Where("room_id = ?", roomID).Delete(&models.ChatUpload{}).Error; err != nil {
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
		if err := tx.Where("room_id = ?", roomID).Delete(&models.ChatUpload{}).Error; err != nil {
			return err
		}
		return tx.Where("id = ?", roomID).Delete(&models.Room{}).Error
	})
}

// SoftDeleteRoom marks a room as archived by setting deleted_at and is_active=false.
// Recording rows and files are preserved.
func (r *RoomRepository) SoftDeleteRoom(roomID string) error {
	now := time.Now()
	return r.db.Model(&models.Room{}).
		Where("id = ?", roomID).
		Updates(map[string]interface{}{
			"deleted_at": now,
			"is_active":  false,
			"updated_at": now,
		}).Error
}

// GetArchivedRoomsByUserPaginated returns archived rooms for a user with pagination.
func (r *RoomRepository) GetArchivedRoomsByUserPaginated(userID string, page, limit int) ([]models.Room, int64, error) {
	offset := (page - 1) * limit
	var total int64
	if err := r.db.Model(&models.Room{}).
		Where("created_by = ? AND deleted_at IS NOT NULL", userID).
		Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rooms []models.Room
	if err := r.db.Where("created_by = ? AND deleted_at IS NOT NULL", userID).
		Order("deleted_at DESC").
		Limit(limit).Offset(offset).
		Find(&rooms).Error; err != nil {
		return nil, 0, err
	}
	return rooms, total, nil
}

// FindArchivedRoomsNoRecordings returns archived rooms that have 0 recording rows.
// Used by scheduler to fully purge rooms after all recordings cleared.
func (r *RoomRepository) FindArchivedRoomsNoRecordings() ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Where("deleted_at IS NOT NULL").
		Where("(SELECT COUNT(*) FROM recordings WHERE room_id = rooms.id) = 0").
		Find(&rooms).Error
	return rooms, err
}

func (r *RoomRepository) GetAllRooms() ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Find(&rooms).Error
	return rooms, err
}

// RoomFilterParams holds all filtering, sorting, and pagination params for admin room listing.
type RoomFilterParams struct {
	Page       int
	Limit      int
	Search     string   // q — name LIKE search
	Visibility []string // "public", "private"
	Status     []string // "active", "suspended", "archived"
	Capacity   string   // "empty", "1-5", "6-20", "20+" — DEPRECATED: filters on max_participants, kept for backward compat
	Occupancy  string   // "empty", "1-5", "6-20", "20+" — filters on actual participant count
	Created    string   // "today", "7d", "30d"
	Sort       string   // "name", "createdAt", "maxParticipants", "participantsCount", "lastActivityAt", "createdBy"
	Order      string   // "asc", "desc"
	// NEW filter fields
	Owner            string // owner name or email search
	DateFrom         string // ISO date for custom creation date range
	DateTo           string // ISO date for custom creation date range
	LastActivityFrom string // ISO date for custom last activity range
	LastActivityTo   string // ISO date for custom last activity range
}

// AdminRoomDetail extends Room with computed fields for admin listing.
type AdminRoomDetail struct {
	ID                string              `json:"id"`
	Name              string              `json:"name"`
	CreatedBy         string              `json:"createdBy"`
	IsActive          bool                `json:"isActive"`
	MaxParticipants   int                 `json:"maxParticipants"`
	CreatedAt         time.Time           `json:"createdAt"`
	UpdatedAt         time.Time           `json:"updatedAt"`
	ExpiresAt         time.Time           `json:"expiresAt"`
	AdminID           string              `json:"adminId"`
	IsPublic          bool                `json:"isPublic"`
	Settings          models.RoomSettings `json:"settings" gorm:"embedded;embeddedPrefix:settings_"`
	Mode              string              `json:"mode"`
	ParticipantsCount int                 `json:"participantsCount"`
	LastActivityAt    *time.Time          `json:"lastActivityAt"`
	OwnerName         string              `json:"ownerName"`
	OwnerEmail        string              `json:"ownerEmail"`
	DeletedAt         *time.Time          `json:"deletedAt,omitempty"`
}

func contains(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func startOfDay(t time.Time) time.Time {
	year, month, day := t.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

// GetAllRoomsFiltered returns a filtered, sorted, paginated list of rooms and the total count.
// The returned rooms do not include computed fields; use EnrichAdminRoomDetails() for that.
func (r *RoomRepository) GetAllRoomsFiltered(p RoomFilterParams) ([]models.Room, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit

	query := r.db.Model(&models.Room{})

	// Search
	if p.Search != "" {
		query = query.Where("name LIKE ?", "%"+p.Search+"%")
	}

	// Visibility
	if len(p.Visibility) > 0 {
		bools := make([]bool, len(p.Visibility))
		for i, v := range p.Visibility {
			bools[i] = v == "public"
		}
		query = query.Where("is_public IN ?", bools)
	}

	// Status
	if len(p.Status) > 0 {
		hasActive := contains(p.Status, "active")
		hasSuspended := contains(p.Status, "suspended")
		hasArchived := contains(p.Status, "archived")

		if hasActive && !hasSuspended && !hasArchived {
			query = query.Where("is_active = ? AND deleted_at IS NULL", true)
		} else if hasSuspended && !hasActive && !hasArchived {
			query = query.Where("is_active = ? AND deleted_at IS NULL", false)
		} else if hasArchived && !hasActive && !hasSuspended {
			query = query.Where("deleted_at IS NOT NULL")
		}
	}

	// Occupancy (filters on actual participant count via WHERE EXISTS subquery)
	if p.Occupancy != "" {
		switch p.Occupancy {
		case "empty":
			query = query.Where("(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) = 0", true, false)
		case "1-5":
			query = query.Where("(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) BETWEEN 1 AND 5", true, false)
		case "6-20":
			query = query.Where("(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) BETWEEN 6 AND 20", true, false)
		case "20+":
			query = query.Where("(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) > 20", true, false)
		}
	}

	// Legacy capacity filter (on max_participants) — kept for backward compat
	if p.Occupancy == "" {
		switch p.Capacity {
		case "empty":
			query = query.Where("max_participants = ?", 0)
		case "1-5":
			query = query.Where("max_participants BETWEEN 1 AND 5")
		case "6-20":
			query = query.Where("max_participants BETWEEN 6 AND 20")
		case "20+":
			query = query.Where("max_participants > 20")
		}
	}

	// Owner filter — JOIN with users table for owner lookup
	needOwnerJoin := p.Owner != "" || p.DateFrom != "" || p.DateTo != "" || p.LastActivityFrom != "" || p.LastActivityTo != ""
	if needOwnerJoin || p.Sort == "createdBy" || p.Sort == "lastActivityAt" || p.Sort == "participantsCount" {
		query = query.Joins("LEFT JOIN users ON users.id = rooms.created_by")
	}

	if p.Owner != "" {
		query = query.Where("users.name LIKE ? OR users.email LIKE ?", "%"+p.Owner+"%", "%"+p.Owner+"%")
	}

	// Created date range
	if p.DateFrom != "" {
		t, err := time.Parse(time.RFC3339, p.DateFrom)
		if err == nil {
			query = query.Where("created_at >= ?", t)
		}
	}
	if p.DateTo != "" {
		t, err := time.Parse(time.RFC3339, p.DateTo)
		if err == nil {
			query = query.Where("created_at <= ?", t)
		}
	}

	// Last activity date range
	if p.LastActivityFrom != "" {
		t, err := time.Parse(time.RFC3339, p.LastActivityFrom)
		if err == nil {
			query = query.Where("(SELECT COALESCE(MAX(joined_at), '1970-01-01') FROM room_participants WHERE room_id = rooms.id AND is_active = ?) >= ?", true, t)
		}
	}
	if p.LastActivityTo != "" {
		t, err := time.Parse(time.RFC3339, p.LastActivityTo)
		if err == nil {
			query = query.Where("(SELECT COALESCE(MAX(joined_at), '1970-01-01') FROM room_participants WHERE room_id = rooms.id AND is_active = ?) <= ?", true, t)
		}
	}

	// Legacy created shortcut
	if p.DateFrom == "" && p.DateTo == "" {
		switch p.Created {
		case "today":
			query = query.Where("created_at >= ?", startOfDay(time.Now()))
		case "7d":
			query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -7))
		case "30d":
			query = query.Where("created_at >= ?", time.Now().AddDate(0, 0, -30))
		}
	}

	// Count before sorting/pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// Sort
	orderClause := "created_at DESC"
	switch p.Sort {
	case "name":
		orderClause = "name " + p.Order
	case "maxParticipants":
		orderClause = "max_participants " + p.Order
	case "createdAt":
		orderClause = "created_at " + p.Order
	case "participantsCount":
		orderClause = "(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) " + p.Order
		query = query.Select("rooms.*", true, false)
	case "lastActivityAt":
		orderClause = "COALESCE((SELECT MAX(joined_at) FROM room_participants WHERE room_id = rooms.id AND is_active = ?), created_at) " + p.Order
	case "createdBy":
		orderClause = "users.name " + p.Order
		// Ensure users JOIN is present
		if !needOwnerJoin {
			query = query.Joins("LEFT JOIN users ON users.id = rooms.created_by")
		}
	}
	query = query.Order(orderClause)

	var rooms []models.Room
	if err := query.Limit(p.Limit).Offset(offset).Find(&rooms).Error; err != nil {
		return nil, 0, err
	}
	return rooms, total, nil
}

// EnrichAdminRoomDetails takes a slice of Room and returns AdminRoomDetail with
// participantsCount, lastActivityAt, ownerName, and ownerEmail populated via batch queries.
func (r *RoomRepository) EnrichAdminRoomDetails(rooms []models.Room) ([]AdminRoomDetail, error) {
	if len(rooms) == 0 {
		return []AdminRoomDetail{}, nil
	}

	ids := make([]string, len(rooms))
	for i, room := range rooms {
		ids[i] = room.ID
	}

	// Batch fetch participant counts per room
	type ParticipantCount struct {
		RoomID string
		Count  int
	}
	var counts []ParticipantCount
	r.db.Table("room_participants").
		Select("room_id, COUNT(*) as count").
		Where("room_id IN ? AND is_active = ? AND is_banned = ?", ids, true, false).
		Group("room_id").
		Scan(&counts)

	countMap := make(map[string]int, len(counts))
	for _, c := range counts {
		countMap[c.RoomID] = c.Count
	}

	// Batch fetch last activity per room
	type LastActivity struct {
		RoomID string
		MaxAt  string
	}
	var activities []LastActivity
	r.db.Table("room_participants").
		Select("room_id, MAX(joined_at) as max_at").
		Where("room_id IN ? AND is_active = ?", ids, true).
		Group("room_id").
		Scan(&activities)

	activityMap := make(map[string]*time.Time, len(activities))
	for _, a := range activities {
		if a.MaxAt != "" {
			if t, err := parseSQLiteTime(a.MaxAt); err == nil {
				activityMap[a.RoomID] = &t
			}
		}
	}

	// Batch fetch owner names
	type Owner struct {
		ID    string
		Name  string
		Email string
	}
	var owners []Owner
	// Collect unique createdBy IDs
	userIDs := make([]string, 0, len(rooms))
	seen := make(map[string]bool)
	for _, room := range rooms {
		if !seen[room.CreatedBy] {
			seen[room.CreatedBy] = true
			userIDs = append(userIDs, room.CreatedBy)
		}
	}
	if len(userIDs) > 0 {
		r.db.Table("users").
			Select("id, name, email").
			Where("id IN ?", userIDs).
			Scan(&owners)
	}

	ownerMap := make(map[string]Owner, len(owners))
	for _, o := range owners {
		ownerMap[o.ID] = o
	}

	// Build enriched result
	result := make([]AdminRoomDetail, len(rooms))
	for i, room := range rooms {
		detail := AdminRoomDetail{
			ID:                room.ID,
			Name:              room.Name,
			CreatedBy:         room.CreatedBy,
			IsActive:          room.IsActive,
			MaxParticipants:   room.MaxParticipants,
			CreatedAt:         room.CreatedAt,
			UpdatedAt:         room.UpdatedAt,
			ExpiresAt:         room.ExpiresAt,
			AdminID:           room.AdminID,
			IsPublic:          room.IsPublic,
			Settings:          room.Settings,
			Mode:              room.Mode,
			ParticipantsCount: countMap[room.ID],
			LastActivityAt:    activityMap[room.ID],
			DeletedAt:         room.DeletedAt,
		}
		if o, ok := ownerMap[room.CreatedBy]; ok {
			detail.OwnerName = o.Name
			detail.OwnerEmail = o.Email
		}
		result[i] = detail
	}

	return result, nil
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

// CountActiveRoomsByUser returns the number of active rooms created by a user.
func (r *RoomRepository) CountActiveRoomsByUser(userID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).
		Where("created_by = ? AND is_active = ?", userID, true).
		Count(&count).Error
	return count, err
}

// GetRoomsCreatedByUser retrieves rooms created by a specific user
func (r *RoomRepository) GetRoomsCreatedByUser(userID string) ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Where("created_by = ?", userID).Order("created_at desc").Find(&rooms).Error
	return rooms, err
}

// GetLatestRoomsCreatedByUser returns the latest room per slug created by a user.
// If multiple rooms share the same name, only the most recently created one is kept.
func (r *RoomRepository) GetLatestRoomsCreatedByUser(userID string) ([]models.Room, error) {
	var allRooms []models.Room
	err := r.db.Where("created_by = ?", userID).Order("created_at desc").Find(&allRooms).Error
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	rooms := make([]models.Room, 0, len(allRooms))
	for _, room := range allRooms {
		if !seen[room.Name] {
			seen[room.Name] = true
			rooms = append(rooms, room)
		}
	}
	return rooms, nil
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

type UserParticipationsParams struct {
	Page  int
	Limit int
}

// GetUserParticipationsPaginated returns paginated room participation records for a user.
// Preloads Room data, ordered by joined_at desc. Returns (participations, totalCount, error).
func (r *RoomRepository) GetUserParticipationsPaginated(userID string, p UserParticipationsParams) ([]models.RoomParticipant, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit

	var total int64
	if err := r.db.Model(&models.RoomParticipant{}).Where("user_id = ?", userID).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var participants []models.RoomParticipant
	err := r.db.
		Preload("Room").
		Where("user_id = ?", userID).
		Order("joined_at desc").
		Limit(p.Limit).
		Offset(offset).
		Find(&participants).Error
	return participants, total, err
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

// GetRoomsByIDs fetches multiple rooms by their IDs.
func (r *RoomRepository) GetRoomsByIDs(ids []string) ([]models.Room, error) {
	var rooms []models.Room
	err := r.db.Where("id IN ?", ids).Find(&rooms).Error
	return rooms, err
}

// BatchSuspendRooms marks multiple rooms as inactive in one query.
func (r *RoomRepository) BatchSuspendRooms(ids []string) error {
	for _, chunk := range batchChunk(ids, 100) {
		if err := r.db.Model(&models.Room{}).
			Where("id IN ?", chunk).
			Where("is_active = ?", true).
			Updates(map[string]any{
				"is_active":  false,
				"updated_at": time.Now(),
			}).Error; err != nil {
			return err
		}
	}
	return nil
}

// CountRooms returns total number of rooms.
func (r *RoomRepository) CountRooms() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Count(&count).Error
	return count, err
}

// CountActiveRooms returns rooms with is_active = true.
func (r *RoomRepository) CountActiveRooms() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Where("is_active = ?", true).Count(&count).Error
	return count, err
}

// CountPublicRooms returns public rooms (is_public = true).
func (r *RoomRepository) CountPublicRooms() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Where("is_public = ?", true).Count(&count).Error
	return count, err
}

// CountPrivateRooms returns private rooms (is_public = false).
func (r *RoomRepository) CountPrivateRooms() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Where("is_public = ?", false).Count(&count).Error
	return count, err
}

// CountEmptyRooms returns rooms with 0 active participants.
func (r *RoomRepository) CountEmptyRooms() (int64, error) {
	var count int64
	err := r.db.Table("rooms").
		Where("(SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?) = 0", true, false).
		Count(&count).Error
	return count, err
}

// CountRoomsSince returns rooms created since the given time.
func (r *RoomRepository) CountRoomsSince(t time.Time) (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Where("created_at >= ?", t).Count(&count).Error
	return count, err
}

// AvgParticipantsPerRoom returns average number of active participants per room.
func (r *RoomRepository) AvgParticipantsPerRoom() (float64, error) {
	var avg float64
	row := r.db.Table("rooms").
		Select("COALESCE(AVG(COALESCE((SELECT COUNT(*) FROM room_participants WHERE room_id = rooms.id AND is_active = ? AND is_banned = ?), 0)), 0)", true, false).
		Row()
	if err := row.Scan(&avg); err != nil {
		return 0, err
	}
	return avg, nil
}

// CountStaleRooms returns rooms with no activity in the given number of hours.
// Uses rooms.last_activity_at (updated on participant join) for accuracy.
// Rooms with nil last_activity_at (pre-migration) fall back to created_at.
func (r *RoomRepository) CountStaleRooms(hours int) (int64, error) {
	cutoff := time.Now().Add(-time.Duration(hours) * time.Hour)
	var count int64
	err := r.db.Model(&models.Room{}).
		Where("COALESCE(last_activity_at, created_at) < ?", cutoff).
		Where("is_active = ?", true).
		Count(&count).Error
	return count, err
}

// BatchHardDeleteRooms permanently deletes multiple rooms and their related data.
func (r *RoomRepository) BatchHardDeleteRooms(ids []string) error {
	for _, chunk := range batchChunk(ids, 100) {
		err := r.db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Where("room_id IN ?", chunk).Delete(&models.RoomPermissions{}).Error; err != nil {
				return err
			}
			if err := tx.Where("room_id IN ?", chunk).Delete(&models.RoomParticipant{}).Error; err != nil {
				return err
			}
			if err := tx.Where("room_id IN ?", chunk).Delete(&models.ChatUpload{}).Error; err != nil {
				return err
			}
			return tx.Where("id IN ?", chunk).Delete(&models.Room{}).Error
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// CountPersistentRooms returns rooms with settings_is_persistent = true.
func (r *RoomRepository) CountPersistentRooms() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).Where("settings_is_persistent = ?", true).Count(&count).Error
	return count, err
}

// CountActiveRoomsByDay returns distinct active room counts per day for last N days.
// Counts rooms that had at least one active participant join on that day.
func (r *RoomRepository) CountActiveRoomsByDay(days int) ([]models.DayCount, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	type dateRow struct {
		Date  string
		Count int
	}
	var rows []dateRow
	err := r.db.Model(&models.RoomParticipant{}).
		Select("DATE(joined_at) as date, COUNT(DISTINCT room_id) as count").
		Where("joined_at >= ?", cutoff).
		Where("is_active = ?", true).
		Group("DATE(joined_at)").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]models.DayCount, len(rows))
	for i, r := range rows {
		t, _ := time.Parse("2006-01-02", r.Date)
		results[i] = models.DayCount{Date: t, Count: r.Count}
	}
	return fillMissingDays(results, days, cutoff), nil
}

// CountRoomsByDay returns room creation counts grouped by day for the last N days.
func (r *RoomRepository) CountRoomsByDay(days int) ([]models.DayCount, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	type dateRow struct {
		Date  string
		Count int
	}
	var rows []dateRow
	err := r.db.Model(&models.Room{}).
		Select("DATE(created_at) as date, COUNT(*) as count").
		Where("created_at >= ?", cutoff).
		Group("DATE(created_at)").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]models.DayCount, len(rows))
	for i, r := range rows {
		t, _ := time.Parse("2006-01-02", r.Date)
		results[i] = models.DayCount{Date: t, Count: r.Count}
	}
	return fillMissingDays(results, days, cutoff), nil
}

// CountActiveParticipantsByDay returns distinct active participant counts per day for last N days.
func (r *RoomRepository) CountActiveParticipantsByDay(days int) ([]models.DayCount, error) {
	cutoff := time.Now().Add(-time.Duration(days) * 24 * time.Hour)
	type dateRow struct {
		Date  string
		Count int
	}
	var rows []dateRow
	err := r.db.Model(&models.RoomParticipant{}).
		Select("DATE(joined_at) as date, COUNT(DISTINCT user_id) as count").
		Where("joined_at >= ?", cutoff).
		Where("is_active = ?", true).
		Group("DATE(joined_at)").
		Order("date ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	results := make([]models.DayCount, len(rows))
	for i, r := range rows {
		t, _ := time.Parse("2006-01-02", r.Date)
		results[i] = models.DayCount{Date: t, Count: r.Count}
	}
	return fillMissingDays(results, days, cutoff), nil
}

// fillMissingDays ensures every day in the range has an entry (zero-fill gaps).
func fillMissingDays(results []models.DayCount, days int, cutoff time.Time) []models.DayCount {
	found := make(map[string]int)
	for _, r := range results {
		key := r.Date.Format("2006-01-02")
		found[key] = r.Count
	}
	var filled []models.DayCount
	for i := 0; i < days; i++ {
		day := cutoff.Add(time.Duration(i) * 24 * time.Hour)
		key := day.Format("2006-01-02")
		c := found[key]
		filled = append(filled, models.DayCount{
			Date:  day,
			Count: c,
		})
	}
	return filled
}

// RoomEventsFilterParams holds filtering/pagination params for room events.
type RoomEventsFilterParams struct {
	Page     int
	Limit    int
	Types    []string // "room_created", "room_joined"
	DateFrom string
	DateTo   string
	Search   string // match against room name or user name
	Order    string // "asc", "desc"
}

// GetRoomEventsFiltered returns paginated room events with optional filters.
func (r *RoomRepository) GetRoomEventsFiltered(p RoomEventsFilterParams) ([]models.RoomEvent, int64, error) {
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit

	orderDir := "DESC"
	if p.Order == "asc" {
		orderDir = "ASC"
	}

	// Build WHERE clauses and args (shared between count and data queries)
	var conditions []string
	var args []interface{}

	// Type filter
	if len(p.Types) > 0 {
		placeholders := make([]string, len(p.Types))
		for i, t := range p.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, "type IN ("+strings.Join(placeholders, ",")+")")
	}

	// Search filter
	if p.Search != "" {
		conditions = append(conditions, "(lower(room_name) LIKE ? OR lower(user_name) LIKE ?)")
		searchTerm := "%" + strings.ToLower(p.Search) + "%"
		args = append(args, searchTerm, searchTerm)
	}

	// Date range filters — parameterized
	if p.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", p.DateFrom); err == nil {
			conditions = append(conditions, "timestamp >= ?")
			args = append(args, p.DateFrom)
		}
	}
	if p.DateTo != "" {
		if _, err := time.Parse("2006-01-02", p.DateTo); err == nil {
			conditions = append(conditions, "timestamp < date(?, '+1 day')")
			args = append(args, p.DateTo)
		}
	}

	whereSQL := ""
	if len(conditions) > 0 {
		whereSQL = " WHERE " + strings.Join(conditions, " AND ")
	}

	// Count query — uses same filters
	countSQL := `
		SELECT COUNT(*) FROM (
			SELECT 'room_created' as type, '' as room_name, '' as user_name, created_at as timestamp
			FROM rooms
			UNION ALL
			SELECT 'room_joined' as type, r.name as room_name, COALESCE(u.name, '') as user_name, rp.joined_at as timestamp
			FROM room_participants rp
			JOIN rooms r ON r.id = rp.room_id
			LEFT JOIN users u ON u.id = rp.user_id
		) AS events` + whereSQL

	var total int64
	if err := r.db.Raw(countSQL, args...).Row().Scan(&total); err != nil {
		return nil, 0, err
	}

	// Data query
	dataSQL := `
		SELECT type, room_id, room_name, user_id, user_name, timestamp FROM (
			SELECT 'room_created' as type, id as room_id, name as room_name,
				created_by as user_id, '' as user_name, created_at as timestamp
			FROM rooms
			UNION ALL
			SELECT 'room_joined' as type, r.id as room_id, r.name as room_name,
				rp.user_id as user_id, COALESCE(u.name, '') as user_name, rp.joined_at as timestamp
			FROM room_participants rp
			JOIN rooms r ON r.id = rp.room_id
			LEFT JOIN users u ON u.id = rp.user_id
		) AS events` + whereSQL + `
		ORDER BY timestamp ` + orderDir + `
		LIMIT ? OFFSET ?`

	dataArgs := make([]interface{}, len(args))
	copy(dataArgs, args)
	dataArgs = append(dataArgs, p.Limit, offset)

	rows, err := r.db.Raw(dataSQL, dataArgs...).Rows()
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var events []models.RoomEvent
	for rows.Next() {
		var (
			typ, roomID, roomName, userID, userName, ts string
		)
		if err := rows.Scan(&typ, &roomID, &roomName, &userID, &userName, &ts); err != nil {
			return nil, 0, err
		}
		var timestamp time.Time
		if t, err := parseSQLiteTime(ts); err == nil {
			timestamp = t
		}
		events = append(events, models.RoomEvent{
			Type:      typ,
			RoomID:    roomID,
			RoomName:  roomName,
			UserID:    userID,
			UserName:  userName,
			Timestamp: timestamp,
		})
	}
	if events == nil {
		events = []models.RoomEvent{}
	}
	return events, total, nil
}

// GetRecentRoomEvents returns recent room create/join events.
func (r *RoomRepository) GetRecentRoomEvents(limit int) ([]models.RoomEvent, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	// Union: room creates + participant joins, ordered by timestamp desc
	var events []models.RoomEvent
	rows, err := r.db.Raw(`
		SELECT * FROM (
			SELECT 'room_created' as type, id as room_id, name as room_name,
				created_by as user_id, '' as user_name, created_at as timestamp
			FROM rooms
			UNION ALL
			SELECT 'room_joined' as type, r.id as room_id, r.name as room_name,
				rp.user_id as user_id, COALESCE(u.name, '') as user_name, rp.joined_at as timestamp
			FROM room_participants rp
			JOIN rooms r ON r.id = rp.room_id
			LEFT JOIN users u ON u.id = rp.user_id
		) AS events
		ORDER BY timestamp DESC
		LIMIT ?
	`, limit).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			typ, roomID, roomName, userID, userName, ts string
		)
		if err := rows.Scan(&typ, &roomID, &roomName, &userID, &userName, &ts); err != nil {
			return nil, err
		}
		var timestamp time.Time
		if t, err := parseSQLiteTime(ts); err == nil {
			timestamp = t
		}
		events = append(events, models.RoomEvent{
			Type:      typ,
			RoomID:    roomID,
			RoomName:  roomName,
			UserID:    userID,
			UserName:  userName,
			Timestamp: timestamp,
		})
	}
	if events == nil {
		events = []models.RoomEvent{}
	}
	return events, nil
}

// CountActiveRoomsWithParticipantCount returns active rooms with participant counts.
func (r *RoomRepository) CountActiveRoomsWithParticipantCount() (int64, error) {
	var count int64
	err := r.db.Model(&models.Room{}).
		Where("is_active = ?", true).
		Count(&count).Error
	return count, err
}

// sqliteTimeFormats matches mattn/go-sqlite3's SQLiteTimestampFormats.
var sqliteTimeFormats = []string{
	"2006-01-02 15:04:05.999999999-07:00",
	"2006-01-02T15:04:05.999999999-07:00",
	"2006-01-02 15:04:05.999999999",
	"2006-01-02T15:04:05.999999999",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05",
	"2006-01-02 15:04",
	"2006-01-02T15:04",
	"2006-01-02",
}

// parseSQLiteTime parses a SQLite timestamp string returned by the driver
// into time.Time, trying all common SQLite timestamp formats.
func parseSQLiteTime(s string) (time.Time, error) {
	for _, f := range sqliteTimeFormats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse SQLite time: %s", s)
}

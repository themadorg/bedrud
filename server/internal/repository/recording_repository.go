// TODO oncoming feature
package repository

import (
	"bedrud/internal/models"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// RecordingRepository manages recording sessions.
type RecordingRepository struct {
	db *gorm.DB
}

// NewRecordingRepository creates a new RecordingRepository.
func NewRecordingRepository(db *gorm.DB) *RecordingRepository {
	return &RecordingRepository{db: db}
}

// ErrRecordingNotFound is returned when a recording does not exist.
var ErrRecordingNotFound = errors.New("recording not found")

// ErrOptimisticLock is returned when a status transition fails because the
// current status does not match the expected fromStatus.
var ErrOptimisticLock = errors.New("recording status already changed")

// Create inserts a new recording. Validates required fields before insert.
func (r *RecordingRepository) Create(rec *models.Recording) error {
	if rec.RoomID == "" {
		return ErrRecordingNotFound
	}
	if rec.CreatedBy == "" {
		return fmt.Errorf("created_by is required")
	}
	if !models.IsValidRecordingType(rec.RecordingType) {
		return fmt.Errorf("invalid recording type: %q", rec.RecordingType)
	}
	return r.db.Create(rec).Error
}

// GetByID returns a recording by its ID.
func (r *RecordingRepository) GetByID(id string) (*models.Recording, error) {
	var rec models.Recording
	err := r.db.Where("id = ?", id).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// GetByEgressID returns a recording by its LiveKit Egress ID.
func (r *RecordingRepository) GetByEgressID(egressID string) (*models.Recording, error) {
	var rec models.Recording
	err := r.db.Where("egress_id = ?", egressID).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRecordingNotFound
		}
		return nil, err
	}
	return &rec, nil
}

// GetActiveByRoom returns the active recording for a room (status=pending|started|processing).
func (r *RecordingRepository) GetActiveByRoom(roomID string) (*models.Recording, error) {
	var rec models.Recording
	err := r.db.Where("room_id = ? AND status IN ?", roomID, []string{
		string(models.RecordingPending),
		string(models.RecordingStarted),
		string(models.RecordingProcessing),
	}).First(&rec).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &rec, nil
}

// CountByRoom returns the total number of recordings for a room.
func (r *RecordingRepository) CountByRoom(roomID string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Recording{}).
		Where("room_id = ?", roomID).
		Count(&count).Error
	return count, err
}

// CountByRoomAndCreator returns count of recordings for a room created by a specific user.
func (r *RecordingRepository) CountByRoomAndCreator(roomID, createdBy string) (int64, error) {
	var count int64
	err := r.db.Model(&models.Recording{}).
		Where("room_id = ? AND created_by = ?", roomID, createdBy).
		Count(&count).Error
	return count, err
}

// ListByRoomAndCreator returns recordings for a room filtered by creator.
func (r *RecordingRepository) ListByRoomAndCreator(roomID, createdBy string, offset, limit int) ([]models.Recording, int64, error) {
	var total int64
	q := r.db.Model(&models.Recording{}).Where("room_id = ? AND created_by = ?", roomID, createdBy)
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var recordings []models.Recording
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&recordings).Error
	return recordings, total, err
}

// HasActiveRecording returns true if the room has an active recording.
func (r *RecordingRepository) HasActiveRecording(roomID string) (bool, error) {
	var count int64
	err := r.db.Model(&models.Recording{}).
		Where("room_id = ? AND status IN ?", roomID, []string{
			string(models.RecordingPending),
			string(models.RecordingStarted),
			string(models.RecordingProcessing),
		}).Count(&count).Error
	return count > 0, err
}

// ListByRoomID returns recordings for a room, paginated.
// ListByRoomID returns paginated recordings for a specific room.
func (r *RecordingRepository) ListByRoomID(roomID string, offset, limit int) ([]models.Recording, int64, error) {
	var total int64
	if err := r.db.Model(&models.Recording{}).Where("room_id = ?", roomID).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var recordings []models.Recording
	err := r.db.Where("room_id = ?", roomID).
		Order("created_at desc").
		Offset(offset).
		Limit(limit).
		Find(&recordings).Error
	return recordings, total, err
}

// ListAdmin returns paginated recordings for the admin panel with optional
// filters: roomID, status, createdAfter, createdBefore. All filters are AND-ed.
// Blank/zero filter values are ignored.
func (r *RecordingRepository) ListAdmin(offset, limit int, roomID, status string, createdAfter, createdBefore *time.Time) ([]models.Recording, int64, error) {
	q := r.db.Model(&models.Recording{})
	if roomID != "" {
		q = q.Where("room_id = ?", roomID)
	}
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if createdAfter != nil {
		q = q.Where("created_at > ?", *createdAfter)
	}
	if createdBefore != nil {
		q = q.Where("created_at < ?", *createdBefore)
	}

	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var recordings []models.Recording
	err := q.Order("created_at desc").Offset(offset).Limit(limit).Find(&recordings).Error
	return recordings, total, err
}

// UpdateStatus transitions a recording from fromStatus to toStatus using
// optimistic locking. Returns ErrOptimisticLock if the status has already changed.
func (r *RecordingRepository) UpdateStatus(id string, fromStatus, toStatus models.RecordingStatus) error {
	result := r.db.Model(&models.Recording{}).
		Where("id = ? AND status = ?", id, fromStatus).
		Updates(map[string]interface{}{
			"status":     toStatus,
			"updated_at": time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// UpdateStartedAt sets the started_at timestamp.
func (r *RecordingRepository) UpdateStartedAt(id string, t time.Time) error {
	return r.db.Model(&models.Recording{}).Where("id = ?", id).Update("started_at", t).Error
}

// UpdateEgressID sets the egress ID and transitions status.
// Only transitions from pending (prevents concurrent double-start).
func (r *RecordingRepository) UpdateEgressID(id, egressID string, status models.RecordingStatus) error {
	result := r.db.Model(&models.Recording{}).Where("id = ? AND status = ?", id, models.RecordingPending).Updates(map[string]interface{}{
		"egress_id":  egressID,
		"status":     status,
		"started_at": time.Now(),
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// UpdateError sets the error message and transitions to failed status.
// Uses optimistic lock: only updates if status is pending, started, or processing.
func (r *RecordingRepository) UpdateError(id, errMsg string) error {
	result := r.db.Model(&models.Recording{}).Where("id = ? AND status IN ?", id, []string{
		string(models.RecordingPending),
		string(models.RecordingStarted),
		string(models.RecordingProcessing),
	}).Updates(map[string]interface{}{
		"status":     models.RecordingFailed,
		"error":      errMsg,
		"updated_at": time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// UpdateCompleted sets the recording as completed with the stored file details.
func (r *RecordingRepository) UpdateCompleted(id, fileURL string, fileSize int64, durationMs int64, completedAt time.Time) error {
	result := r.db.Model(&models.Recording{}).Where("id = ? AND status = ?", id, models.RecordingProcessing).Updates(map[string]interface{}{
		"status":       models.RecordingCompleted,
		"file_url":     fileURL,
		"file_size":    fileSize,
		"duration_ms":  durationMs,
		"completed_at": completedAt,
		"updated_at":   time.Now(),
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrOptimisticLock
	}
	return nil
}

// DeleteRecording removes a recording record by ID.
func (r *RecordingRepository) DeleteRecording(id string) error {
	result := r.db.Delete(&models.Recording{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrRecordingNotFound
	}
	return nil
}

// GetRecordingsByIDs returns recordings for the given IDs.
func (r *RecordingRepository) GetRecordingsByIDs(ids []string) ([]models.Recording, error) {
	var recordings []models.Recording
	err := r.db.Where("id IN ?", ids).Find(&recordings).Error
	return recordings, err
}

// MarkDeleting sets a recording's status to "deleting" without optimistic locking.
// Admin-initiated deletes skip the normal status machine.
func (r *RecordingRepository) MarkDeleting(id string) error {
	return r.db.Model(&models.Recording{}).
		Where("id = ?", id).
		Update("status", models.RecordingDeleting).Error
}

// DeleteByRoom removes all recording records for a given room.
func (r *RecordingRepository) DeleteByRoom(roomID string) error {
	return r.db.Where("room_id = ?", roomID).Delete(&models.Recording{}).Error
}

// DeleteStaleRecordings removes recordings that are stuck in failed, pending, or started
// status and older than the given cutoff time. started recordings are included because
// the egress_ended webhook may never arrive (e.g. self-signed TLS, LK crash, network split).
func (r *RecordingRepository) DeleteStaleRecordings(cutoff time.Time) error {
	return r.db.Where("status IN ? AND created_at < ?", []string{
		string(models.RecordingFailed),
		string(models.RecordingPending),
		string(models.RecordingStarted),
	}, cutoff).Delete(&models.Recording{}).Error
}

// FindExpiredOnArchivedRooms returns completed recordings on archived rooms
// where the recording completed_at (or created_at fallback) is older than cutoff.
// These are eligible for retention-based cleanup.
func (r *RecordingRepository) FindExpiredOnArchivedRooms(cutoff time.Time) ([]models.Recording, error) {
	var recordings []models.Recording
	err := r.db.Joins("JOIN rooms ON rooms.id = recordings.room_id").
		Where("rooms.deleted_at IS NOT NULL").
		Where("recordings.status IN ?", []string{
			string(models.RecordingCompleted),
			string(models.RecordingFailed),
		}).
		Where("COALESCE(recordings.completed_at, recordings.created_at) < ?", cutoff).
		Find(&recordings).Error
	return recordings, err
}

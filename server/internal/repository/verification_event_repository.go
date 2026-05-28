package repository

import (
	"bedrud/internal/models"

	"gorm.io/gorm"
)

type VerificationEventRepository struct {
	db *gorm.DB
}

func NewVerificationEventRepository(db *gorm.DB) *VerificationEventRepository {
	return &VerificationEventRepository{db: db}
}

func (r *VerificationEventRepository) RecordEvent(userID, email string, eventType models.VerificationEventType, ip, metadata string) error {
	return r.db.Create(&models.VerificationEvent{
		UserID:    userID,
		Email:     email,
		EventType: eventType,
		IP:        ip,
		Metadata:  metadata,
	}).Error
}

func (r *VerificationEventRepository) GetRecentEvents(limit int) ([]models.VerificationEvent, error) {
	var events []models.VerificationEvent
	err := r.db.Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

func (r *VerificationEventRepository) GetEventsByUser(userID string, limit int) ([]models.VerificationEvent, error) {
	var events []models.VerificationEvent
	err := r.db.Where("user_id = ?", userID).
		Order("created_at DESC").Limit(limit).Find(&events).Error
	return events, err
}

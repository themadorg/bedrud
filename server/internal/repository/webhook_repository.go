package repository

import (
	"bedrud/internal/models"
	"errors"
	"time"

	"gorm.io/gorm"
)

// WebhookRepository manages webhook endpoint configurations.
type WebhookRepository struct {
	db *gorm.DB
}

// NewWebhookRepository creates a new WebhookRepository.
func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// ErrWebhookNotFound is returned when a webhook does not exist.
var ErrWebhookNotFound = errors.New("webhook not found")

// GetByID returns a webhook by its ID.
func (r *WebhookRepository) GetByID(id string) (*models.Webhook, error) {
	var w models.Webhook
	err := r.db.Where("id = ?", id).First(&w).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, err
	}
	return &w, nil
}

// List returns all webhooks (unpaginated — use ListPaginated for pagination).
func (r *WebhookRepository) List() ([]models.Webhook, error) {
	var webhooks []models.Webhook
	err := r.db.Order("created_at desc").Find(&webhooks).Error
	return webhooks, err
}

// ListPaginated returns a paginated list of webhooks with total count.
func (r *WebhookRepository) ListPaginated(p PaginationParams) ([]models.Webhook, int64, error) {
	var total int64
	if err := r.db.Model(&models.Webhook{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}
	if p.Page <= 0 {
		p.Page = 1
	}
	offset := (p.Page - 1) * p.Limit
	if offset > 10000 {
		offset = 10000
	}
	var webhooks []models.Webhook
	err := r.db.Order("created_at desc").Limit(p.Limit).Offset(offset).Find(&webhooks).Error
	return webhooks, total, err
}

// ListActive returns webhooks that are active and subscribed to the given event.
// For SQLite, event filtering is done in Go after fetching all active webhooks.
// For Postgres, JSON_CONTAINS or ? can be used but Go-side filter is simpler and
// scale concerns are minimal for webhook counts (typically < 100).
func (r *WebhookRepository) ListActive(event string) ([]models.Webhook, error) {
	var all []models.Webhook
	err := r.db.Where("is_active = ?", true).Find(&all).Error
	if err != nil {
		return nil, err
	}
	if event == "" {
		return all, nil
	}
	var filtered []models.Webhook
	for _, w := range all {
		for _, e := range w.Events {
			if e == event {
				filtered = append(filtered, w)
				break
			}
		}
	}
	return filtered, nil
}

// Create inserts a new webhook.
func (r *WebhookRepository) Create(w *models.Webhook) error {
	return r.db.Create(w).Error
}

// Update saves changes to an existing webhook.
func (r *WebhookRepository) Update(w *models.Webhook) error {
	return r.db.Save(w).Error
}

// Delete removes a webhook by ID.
func (r *WebhookRepository) Delete(id string) error {
	result := r.db.Delete(&models.Webhook{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// UpdateSecret sets a new secret for the webhook.
func (r *WebhookRepository) UpdateSecret(id, newSecret string) error {
	result := r.db.Model(&models.Webhook{}).Where("id = ?", id).Update("secret", newSecret)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// UpdateLastSeen records the last successful delivery timestamp.
func (r *WebhookRepository) UpdateLastSeen(id string, t time.Time) error {
	result := r.db.Model(&models.Webhook{}).Where("id = ?", id).Update("last_seen", t)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

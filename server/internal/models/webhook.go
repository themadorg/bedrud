package models

import "time"

// Outbound webhook event names (our own event types sent to external URLs).
const (
	EventRoomCreated       = "room.created"
	EventRoomEnded         = "room.ended"
	EventParticipantJoined = "participant.joined"
	// TODO oncoming feature
	EventRecordingCompleted = "recording.completed"
	EventWebhookTest        = "ping"
)

// Webhook represents an outbound webhook endpoint configuration.
type Webhook struct {
	ID        string     `gorm:"primaryKey;type:varchar(36)" json:"id"`
	Name      string     `gorm:"not null;type:varchar(255)" json:"name"`
	URL       string     `gorm:"not null;type:varchar(1024)" json:"url"`
	Secret    string     `gorm:"not null;type:varchar(255)" json:"-"` // masked in API; never exposed after creation
	Events    []string   `gorm:"serializer:json;type:text" json:"events"`
	IsActive  bool       `gorm:"not null;default:true" json:"isActive"`
	LastSeen  *time.Time `gorm:"index" json:"lastSeen"`
	CreatedBy string     `gorm:"type:varchar(36);not null" json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// MaskedSecret returns a masked version of the webhook secret for display.
func (w *Webhook) MaskedSecret() string {
	if len(w.Secret) > 4 {
		return "••••" + w.Secret[len(w.Secret)-4:]
	}
	return "••••"
}

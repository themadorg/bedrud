package models

import "time"

type VerificationEventType string

const (
	VerificationSent        VerificationEventType = "sent"
	VerificationResent      VerificationEventType = "resent"
	VerificationSuccess     VerificationEventType = "success"
	VerificationFailed      VerificationEventType = "failed"
	VerificationAdminForce  VerificationEventType = "admin_force"
	VerificationEmailChange VerificationEventType = "email_change"
)

// VerificationEvent records every send, resend, verify, and failed attempt
// for audit and abuse investigation (SOC 2 / GDPR compliance).
type VerificationEvent struct {
	ID        uint                  `gorm:"primarykey" json:"id"`
	UserID    string                `gorm:"index;not null" json:"userId"`
	Email     string                `gorm:"not null" json:"email"`
	EventType VerificationEventType `gorm:"not null;index" json:"eventType"`
	IP        string                `json:"ip,omitempty"`
	Metadata  string                `json:"metadata,omitempty"` // JSON blob for extra context
	CreatedAt time.Time             `json:"createdAt"`
}

func (VerificationEvent) TableName() string {
	return "verification_events"
}

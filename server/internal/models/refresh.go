package models

import "time"

type BlockedRefreshToken struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Token     string    `json:"token" gorm:"type:text;not null;uniqueIndex"`
	UserID    string    `json:"userId" gorm:"type:varchar(36);not null;index"`
	ExpiresAt time.Time `json:"expiresAt" gorm:"not null;index"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;not null"`
}

// TableName specifies the table name for GORM
func (BlockedRefreshToken) TableName() string {
	return "blocked_refresh_tokens"
}

// BlockedAccessToken stores SHA-256 hashes of revoked access JWTs until natural expiry.
// Survives process restart (unlike the in-memory revoke set alone).
type BlockedAccessToken struct {
	ID        string    `json:"id" gorm:"primaryKey;type:varchar(36)"`
	Token     string    `json:"token" gorm:"type:varchar(64);not null;uniqueIndex"` // sha256 hex
	ExpiresAt time.Time `json:"expiresAt" gorm:"not null;index"`
	CreatedAt time.Time `json:"createdAt" gorm:"autoCreateTime;not null"`
}

func (BlockedAccessToken) TableName() string {
	return "blocked_access_tokens"
}

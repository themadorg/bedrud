// TODO oncoming feature
package models

import "time"

// RecordingStatus tracks the lifecycle of a recording.
type RecordingStatus string

const (
	RecordingPending    RecordingStatus = "pending"
	RecordingStarted    RecordingStatus = "started"
	RecordingProcessing RecordingStatus = "processing"
	RecordingCompleted  RecordingStatus = "completed"
	RecordingFailed     RecordingStatus = "failed"
	RecordingDeleting   RecordingStatus = "deleting"
)

// RecordingType constants.
const (
	RecordingTypeComposite = "composite"
	RecordingTypeAudio     = "audio"
	RecordingTypeVideo     = "video"
	RecordingTypeScreen    = "screen"
)

// ValidRecordingTypes contains all valid recording type values.
var ValidRecordingTypes = map[string]bool{
	RecordingTypeComposite: true,
	RecordingTypeAudio:     true,
	RecordingTypeVideo:     true,
	RecordingTypeScreen:    true,
}

// IsValidRecordingType checks if the given type is a known recording type.
func IsValidRecordingType(t string) bool {
	return ValidRecordingTypes[t]
}

// Recording represents a LiveKit Egress recording session.
type Recording struct {
	ID            string          `gorm:"primaryKey;type:varchar(36)" json:"id"`
	RoomID        string          `gorm:"type:varchar(36);not null;index" json:"roomId"`
	RoomName      string          `gorm:"type:varchar(255);not null" json:"roomName"`
	EgressID      string          `gorm:"uniqueIndex;type:varchar(255)" json:"egressId"`
	FileURL       string          `gorm:"type:text" json:"fileUrl,omitempty"`
	FileSize      int64           `gorm:"default:0" json:"fileSize"`             // int64; recordings can be hours long
	RecordingType string          `gorm:"type:varchar(20)" json:"recordingType"` // audio, video, screen, composite
	DurationMs    int64           `gorm:"default:0" json:"durationMs,omitempty"` // int64; hours of recording in ms; omitempty for 0
	Status        RecordingStatus `gorm:"not null;default:'pending';type:varchar(20)" json:"status"`
	Error         string          `gorm:"type:text" json:"error,omitempty"`
	CreatedBy     string          `gorm:"type:varchar(36);not null" json:"createdBy"`
	StartedAt     *time.Time      `json:"startedAt,omitempty"`
	CompletedAt   *time.Time      `json:"completedAt,omitempty"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
}

package models

import "time"

// JobStatus tracks a job's lifecycle state.
type JobStatus string

const (
	JobPending JobStatus = "pending"
	JobActive  JobStatus = "active"
	JobDone    JobStatus = "done"
	JobFailed  JobStatus = "failed"
)

// Job is the GORM model for the internal job queue.
type Job struct {
	ID          string    `gorm:"type:varchar(36);primaryKey"`
	Type        string    `gorm:"index;not null"`
	Payload     string    `gorm:"type:text"`       // JSON string — works on SQLite + PG
	RunAt       time.Time `gorm:"index;not null"`  // when job becomes eligible
	Priority    int       `gorm:"index;default:0"` // lower = higher priority
	Status      JobStatus `gorm:"index;not null;default:pending"`
	Attempts    int       `gorm:"not null;default:0"`
	MaxAttempts int       `gorm:"not null;default:3"`
	LastError   string    `gorm:"type:text"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// TableName specifies the table name for GORM.
func (Job) TableName() string { return "jobs" }

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
	ID      string `gorm:"type:varchar(36);primaryKey"`
	Type    string `gorm:"index;not null"`
	Payload string `gorm:"type:text"` // JSON string — works on SQLite + PG
	// DedupeKey collapses repeat requests for the same target while a job for
	// it is still unfinished — a second delete of the same room, say. Empty
	// means no deduplication. Enforced by idx_jobs_active_dedupe, a partial
	// unique index over (type, dedupe_key) covering pending and active rows
	// only, so a finished job never blocks a later one.
	DedupeKey   string    `gorm:"type:varchar(255);index"`
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

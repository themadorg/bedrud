package models

import "time"

type QueueStats struct {
	Pending         int64              `json:"pending"`
	Active          int64              `json:"active"`
	Done24h         int64              `json:"done24h"`
	Failed24h       int64              `json:"failed24h"`
	Total           int64              `json:"total"`
	MaxDepth        int64              `json:"maxDepth"`
	OldestPending   *time.Time         `json:"oldestPending,omitempty"`
	RecentFailures  []FailedJobSummary `json:"recentFailures,omitempty"`
	ProcessedPerMin float64            `json:"processedPerMin"`
	FailedPerMin    float64            `json:"failedPerMin"`
	FailRate        float64            `json:"failRate"`

	PendingEmail   int64      `json:"pendingEmail"`
	FailedEmail24h int64      `json:"failedEmail24h"`
	LastSendError  string     `json:"lastSendError,omitempty"`
	LastSendErrorAt *time.Time `json:"lastSendErrorAt,omitempty"`
}

type FailedJobSummary struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Error     string    `json:"error"`
	Attempts  int       `json:"attempts"`
	UpdatedAt time.Time `json:"updatedAt"`
	Age       string    `json:"age"`
}

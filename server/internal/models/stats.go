package models

import "time"

// DayCount represents a single day's aggregate count.
type DayCount struct {
	Date  time.Time `json:"date"`
	Count int       `json:"count"`
}

// RoomEvent represents a recent room activity event.
type RoomEvent struct {
	Type      string    `json:"type"` // room_created, room_joined
	RoomID    string    `json:"roomId,omitempty"`
	RoomName  string    `json:"roomName,omitempty"`
	UserID    string    `json:"userId,omitempty"`
	UserName  string    `json:"userName,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

// OverviewHealth holds system health status.
type OverviewHealth struct {
	Status        string     `json:"status"` // healthy, degraded, down
	TLS           *TLSStatus `json:"tls"`
	Realtime      string     `json:"realtime"` // connected, disconnected
	AlertsCount   int        `json:"alertsCount"`
	UptimeSeconds int64      `json:"uptimeSeconds"`
	DBStatus      string     `json:"dbStatus"` // connected, error
}

// TLSStatus holds TLS certificate info.
type TLSStatus struct {
	Enabled       bool   `json:"enabled"`
	DaysRemaining int    `json:"daysRemaining"`
	ExpiryDate    string `json:"expiryDate,omitempty"`
	Status        string `json:"status"` // valid, expiring, expired, unknown
}

// KpiEntry holds a single KPI value with optional delta.
type KpiEntry struct {
	Value        int    `json:"value"`
	Delta        int    `json:"delta,omitempty"`
	DeltaLabel   string `json:"deltaLabel,omitempty"`
	DeltaPercent int    `json:"deltaPercent,omitempty"`
	PeakToday    int    `json:"peakToday,omitempty"`
	ActiveNow    int    `json:"activeNow,omitempty"`
}

// OverviewKPIs holds all KPI values.
type OverviewKPIs struct {
	TotalUsers     KpiEntry `json:"totalUsers"`
	OnlineNow      KpiEntry `json:"onlineNow"`
	TotalRooms     KpiEntry `json:"totalRooms"`
	ActiveSessions KpiEntry `json:"activeSessions"`
	PendingActions KpiEntry `json:"pendingActions"`
}

// RoomComposition holds room type breakdown.
type RoomComposition struct {
	Live       int `json:"live"`
	Public     int `json:"public"`
	Private    int `json:"private"`
	Persistent int `json:"persistent"`
	Stale      int `json:"stale"`
}

// AttentionItem represents something needing operator review.
type AttentionItem struct {
	Type     string `json:"type"`     // tls_expiry, stale_room, empty_room, auth_spike
	Severity string `json:"severity"` // error, warning, info
	Message  string `json:"message"`
	DaysLeft int    `json:"daysLeft,omitempty"`
	RoomID   string `json:"roomId,omitempty"`
}

// InstanceInfo holds server metadata.
type InstanceInfo struct {
	Name          string `json:"name"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	StartedAt     string `json:"startedAt"`
}

// DayActivity holds one day of activity data.
type DayActivity struct {
	Date         string `json:"date"`
	RoomsCreated int    `json:"roomsCreated"`
	RoomsActive  int    `json:"roomsActive"`
	Participants int    `json:"participants"`
}

// RecentUser holds minimal user info for recent signups.
type RecentUser struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Email     string `json:"email"`
	Provider  string `json:"provider"`
	CreatedAt string `json:"createdAt"`
}

// OverviewResponse is the full response for GET /api/admin/overview.
type OverviewResponse struct {
	Health          OverviewHealth  `json:"health"`
	KPIs            OverviewKPIs    `json:"kpis"`
	ActivityTrend   []DayActivity   `json:"activityTrend"`
	RoomComposition RoomComposition `json:"roomComposition"`
	NeedsAttention  []AttentionItem `json:"needsAttention"`
	RecentSignups   []RecentUser    `json:"recentSignups"`
	RecentEvents    []RoomEvent     `json:"recentRoomEvents"`
	InstanceInfo    InstanceInfo    `json:"instanceInfo"`
}

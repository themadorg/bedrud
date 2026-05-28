package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupQueueTestApp(t *testing.T) (*fiber.App, *AdminQueueHandler) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	h := NewAdminQueueHandler(db)

	app := fiber.New()
	// Simulate Protected + RequireAccess(superadmin) middleware
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user",
			Email:    "admin@ex.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app.Get("/admin/queue", h.GetQueueStats)

	return app, h
}

func seedJob(t *testing.T, h *AdminQueueHandler, id, jobType string, status models.JobStatus, updatedAt time.Time, lastError string, attempts int) {
	t.Helper()
	job := models.Job{
		ID:          id,
		Type:        jobType,
		Payload:     `{}`,
		RunAt:       updatedAt,
		Status:      status,
		Attempts:    attempts,
		MaxAttempts: 3,
		LastError:   lastError,
		CreatedAt:   updatedAt,
		UpdatedAt:   updatedAt,
	}
	if err := h.db.Create(&job).Error; err != nil {
		t.Fatalf("seed job: %v", err)
	}
}

// --- Empty queue ---

func TestAdminQueue_Empty(t *testing.T) {
	app, _ := setupQueueTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("json unmarshal: %v", err)
	}

	if stats.Pending != 0 {
		t.Errorf("expected pending=0, got %d", stats.Pending)
	}
	if stats.Active != 0 {
		t.Errorf("expected active=0, got %d", stats.Active)
	}
	if stats.Done24h != 0 {
		t.Errorf("expected done24h=0, got %d", stats.Done24h)
	}
	if stats.Failed24h != 0 {
		t.Errorf("expected failed24h=0, got %d", stats.Failed24h)
	}
	if stats.Total != 0 {
		t.Errorf("expected total=0, got %d", stats.Total)
	}
	if stats.MaxDepth != 10000 {
		t.Errorf("expected maxDepth=10000, got %d", stats.MaxDepth)
	}
	if stats.OldestPending != nil {
		t.Errorf("expected oldestPending=nil, got %v", stats.OldestPending)
	}
	if len(stats.RecentFailures) != 0 {
		t.Errorf("expected 0 recentFailures, got %d", len(stats.RecentFailures))
	}
	if stats.ProcessedPerMin != 0 {
		t.Errorf("expected processedPerMin=0, got %f", stats.ProcessedPerMin)
	}
	if stats.FailedPerMin != 0 {
		t.Errorf("expected failedPerMin=0, got %f", stats.FailedPerMin)
	}
	if stats.FailRate != 0 {
		t.Errorf("expected failRate=0, got %f", stats.FailRate)
	}
}

// --- Counts correct ---

func TestAdminQueue_Counts(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	seedJob(t, h, uuid.New().String(), "room_delete", models.JobPending, now, "", 0)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobActive, now, "", 1)
	seedJob(t, h, uuid.New().String(), "chat_upload_s3", models.JobDone, now, "", 0)
	seedJob(t, h, uuid.New().String(), "room_delete", models.JobFailed, now, "timeout", 3)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.Pending != 1 {
		t.Errorf("expected pending=1, got %d", stats.Pending)
	}
	if stats.Active != 1 {
		t.Errorf("expected active=1, got %d", stats.Active)
	}
	if stats.Done24h != 1 {
		t.Errorf("expected done24h=1, got %d", stats.Done24h)
	}
	if stats.Failed24h != 1 {
		t.Errorf("expected failed24h=1, got %d", stats.Failed24h)
	}
	if stats.Total != 4 {
		t.Errorf("expected total=4, got %d", stats.Total)
	}
}

// --- Jobs outside 24h window excluded from done/failed counts ---

func TestAdminQueue_Outside24hWindow(t *testing.T) {
	app, h := setupQueueTestApp(t)
	old := time.Now().Add(-48 * time.Hour)

	seedJob(t, h, uuid.New().String(), "room_delete", models.JobDone, old, "", 0)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobFailed, old, "expired", 2)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.Done24h != 0 {
		t.Errorf("expected done24h=0 (48h old), got %d", stats.Done24h)
	}
	if stats.Failed24h != 0 {
		t.Errorf("expected failed24h=0 (48h old), got %d", stats.Failed24h)
	}
	if stats.Total != 2 {
		t.Errorf("expected total=2, got %d", stats.Total)
	}
}

// --- Recent failures returns top 10 ordered by updated_at DESC ---

func TestAdminQueue_RecentFailures(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// Create 12 failed jobs, only 10 should be returned
	for i := 0; i < 12; i++ {
		at := now.Add(-time.Duration(i) * time.Minute)
		seedJob(t, h, uuid.New().String(), "room_delete", models.JobFailed, at, "err", i+1)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if len(stats.RecentFailures) != 10 {
		t.Fatalf("expected 10 recent failures, got %d", len(stats.RecentFailures))
	}

	// Verify most recent first
	if stats.RecentFailures[0].Attempts != 1 {
		t.Errorf("expected first failure attempts=1 (most recent), got %d", stats.RecentFailures[0].Attempts)
	}
	if stats.RecentFailures[9].Attempts != 10 {
		t.Errorf("expected last failure attempts=10 (oldest of 10), got %d", stats.RecentFailures[9].Attempts)
	}

	// Verify fields populated
	f := stats.RecentFailures[0]
	if f.ID == "" {
		t.Error("expected non-empty ID")
	}
	if f.Type != "room_delete" {
		t.Errorf("expected type=room_delete, got %s", f.Type)
	}
	if f.Error != "err" {
		t.Errorf("expected error=err, got %s", f.Error)
	}
	if f.Age == "" {
		t.Error("expected non-empty age")
	}
	if f.UpdatedAt.IsZero() {
		t.Error("expected non-zero updatedAt")
	}
}

// --- Rates: processed and failed per minute ---

func TestAdminQueue_Rates(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// 10 done + 5 failed in last 5 minutes = 2/min done, 1/min failed
	for i := 0; i < 10; i++ {
		seedJob(t, h, uuid.New().String(), "room_delete", models.JobDone, now.Add(-time.Duration(i)*10*time.Second), "", 0)
	}
	for i := 0; i < 5; i++ {
		seedJob(t, h, uuid.New().String(), "user_delete", models.JobFailed, now.Add(-time.Duration(i)*30*time.Second), "err", 1)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.ProcessedPerMin != 2.0 {
		t.Errorf("expected processedPerMin=2.0, got %f", stats.ProcessedPerMin)
	}
	if stats.FailedPerMin != 1.0 {
		t.Errorf("expected failedPerMin=1.0, got %f", stats.FailedPerMin)
	}
}

// --- Fail rate edge case: zero total 24h ---

func TestAdminQueue_FailRateZeroTotal(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// Only pending jobs, no done/failed in 24h
	seedJob(t, h, uuid.New().String(), "room_delete", models.JobPending, now, "", 0)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.FailRate != 0 {
		t.Errorf("expected failRate=0 (no 24h jobs), got %f", stats.FailRate)
	}
	if stats.Done24h != 0 {
		t.Errorf("expected done24h=0, got %d", stats.Done24h)
	}
	if stats.Failed24h != 0 {
		t.Errorf("expected failed24h=0, got %d", stats.Failed24h)
	}
}

// --- Fail rate calculated correctly ---

func TestAdminQueue_FailRateWithData(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// 80 done, 20 failed in 24h → failRate = 0.2
	for i := 0; i < 80; i++ {
		seedJob(t, h, uuid.New().String(), "room_delete", models.JobDone, now.Add(-time.Duration(i)*time.Minute), "", 0)
	}
	for i := 0; i < 20; i++ {
		seedJob(t, h, uuid.New().String(), "user_delete", models.JobFailed, now.Add(-time.Duration(i)*time.Minute), "err", 1)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	expectedFailRate := 20.0 / 100.0 // 0.2
	if stats.FailRate != expectedFailRate {
		t.Errorf("expected failRate=%f, got %f", expectedFailRate, stats.FailRate)
	}
}

// --- OldestPending is MIN(run_at) of pending jobs ---

func TestAdminQueue_OldestPending(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// 3 pending jobs with staggered run_at
	oldest := now.Add(-30 * time.Minute)
	middle := now.Add(-10 * time.Minute)
	latest := now.Add(5 * time.Minute)

	seedJob(t, h, uuid.New().String(), "room_delete", models.JobPending, latest, "", 0)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobPending, oldest, "", 0)
	seedJob(t, h, uuid.New().String(), "chat_upload", models.JobPending, middle, "", 0)

	// Direct DB check — verify run_at values persisted
	var minRunAt *time.Time
	h.db.Model(&models.Job{}).Where("status = ?", models.JobPending).Select("MIN(run_at)").Scan(&minRunAt)
	t.Logf("direct MIN(run_at): %v", minRunAt)
	if minRunAt == nil {
		t.Fatal("direct MIN(run_at) returned nil — seeded jobs not persisted")
	}

	// Count pending jobs
	var cnt int64
	h.db.Model(&models.Job{}).Where("status = ?", models.JobPending).Count(&cnt)
	t.Logf("pending count: %d", cnt)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	if err := json.Unmarshal(body, &stats); err != nil {
		t.Fatalf("json unmarshal: %v, body: %s", err, string(body))
	}
	t.Logf("stats.OldestPending: %v", stats.OldestPending)

	if stats.OldestPending == nil {
		t.Fatal("expected oldestPending to be set")
	}
	// Compare truncated to second since sub-second precision may differ
	if !stats.OldestPending.Truncate(time.Second).Equal(oldest.Truncate(time.Second)) {
		t.Errorf("expected oldestPending=%v, got %v", oldest, *stats.OldestPending)
	}
}

// --- No pending jobs → oldestPending is nil ---

func TestAdminQueue_OldestPendingNone(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	// Only non-pending jobs
	seedJob(t, h, uuid.New().String(), "room_delete", models.JobDone, now, "", 0)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobActive, now, "", 1)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.OldestPending != nil {
		t.Errorf("expected oldestPending=nil, got %v", *stats.OldestPending)
	}
}

// --- Mixed statuses: all in one table ---

func TestAdminQueue_MixedStates(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	seedJob(t, h, uuid.New().String(), "room_delete", models.JobPending, now, "", 0)
	seedJob(t, h, uuid.New().String(), "room_delete", models.JobPending, now.Add(-1*time.Hour), "", 0)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobActive, now, "", 1)
	seedJob(t, h, uuid.New().String(), "chat_upload_s3", models.JobDone, now.Add(-2*time.Hour), "", 0)
	seedJob(t, h, uuid.New().String(), "chat_upload_s3", models.JobDone, now.Add(-30*time.Hour), "", 0) // outside 24h
	seedJob(t, h, uuid.New().String(), "room_delete", models.JobFailed, now.Add(-1*time.Hour), "timeout", 3)
	seedJob(t, h, uuid.New().String(), "user_delete", models.JobFailed, now.Add(-3*time.Hour), "not found", 2)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if stats.Pending != 2 {
		t.Errorf("expected pending=2, got %d", stats.Pending)
	}
	if stats.Active != 1 {
		t.Errorf("expected active=1, got %d", stats.Active)
	}
	// Only 1 done in 24h (the 30h old one excluded)
	if stats.Done24h != 1 {
		t.Errorf("expected done24h=1, got %d", stats.Done24h)
	}
	// Both failed are within 24h
	if stats.Failed24h != 2 {
		t.Errorf("expected failed24h=2, got %d", stats.Failed24h)
	}
	// Total includes ALL jobs
	if stats.Total != 7 {
		t.Errorf("expected total=7, got %d", stats.Total)
	}
	// 2 failures returned
	if len(stats.RecentFailures) != 2 {
		t.Errorf("expected 2 recent failures, got %d", len(stats.RecentFailures))
	}
	// Fail rate: 2 failures / (1 done + 2 failed) = 2/3
	expected := 2.0 / 3.0
	if stats.FailRate != expected {
		t.Errorf("expected failRate=%f, got %f", expected, stats.FailRate)
	}
}

// --- FailedJobSummary fields populated correctly ---

func TestAdminQueue_FailedJobSummaryFields(t *testing.T) {
	app, h := setupQueueTestApp(t)
	now := time.Now()

	seedJob(t, h, "failure-1", "user_delete", models.JobFailed, now, "connection refused", 5)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var stats models.QueueStats
	json.Unmarshal(body, &stats)

	if len(stats.RecentFailures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(stats.RecentFailures))
	}

	f := stats.RecentFailures[0]
	if f.ID != "failure-1" {
		t.Errorf("expected id=failure-1, got %s", f.ID)
	}
	if f.Type != "user_delete" {
		t.Errorf("expected type=user_delete, got %s", f.Type)
	}
	if f.Error != "connection refused" {
		t.Errorf("expected error=connection refused, got %s", f.Error)
	}
	if f.Attempts != 5 {
		t.Errorf("expected attempts=5, got %d", f.Attempts)
	}
	if f.UpdatedAt.IsZero() {
		t.Error("expected non-zero UpdatedAt")
	}
	if f.Age == "" {
		t.Error("expected non-empty Age")
	}
}

// --- formatAge edge cases ---

func TestFormatAge(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		want     string
	}{
		{"just now", 10 * time.Second, "just now"},
		{"30 seconds", 30 * time.Second, "just now"},
		{"1 minute", 1 * time.Minute, "1m ago"},
		{"5 minutes", 5 * time.Minute, "5m ago"},
		{"59 minutes", 59 * time.Minute, "59m ago"},
		{"1 hour", 1 * time.Hour, "1h ago"},
		{"5 hours", 5 * time.Hour, "5h ago"},
		{"23 hours", 23 * time.Hour, "23h ago"},
		{"1 day", 24 * time.Hour, "1d ago"},
		{"3 days", 72 * time.Hour, "3d ago"},
		{"30 days", 720 * time.Hour, "30d ago"},
		{"0 duration", 0, "just now"},
		{"negative", -1 * time.Minute, "just now"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatAge(tt.duration)
			if got != tt.want {
				t.Errorf("formatAge(%v) = %q, want %q", tt.duration, got, tt.want)
			}
		})
	}
}

// TestAdminQueue_NilDB verifies that a nil DB doesn't crash the server.
// All goroutines panic → queueQuery recover catches → errCount >= 5 → 500.
func TestAdminQueue_NilDB(t *testing.T) {
	h := &AdminQueueHandler{db: nil}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user",
			Email:    "admin@ex.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app.Get("/admin/queue", h.GetQueueStats)

	req := httptest.NewRequest(http.MethodGet, "/admin/queue", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 500 with nil DB, got %d: %s", resp.StatusCode, string(body))
	}
}

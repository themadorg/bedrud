package queue

import (
	"bedrud/internal/models"
	"context"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// openTestDB creates an in-memory SQLite DB with Job migrated.
func openTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&models.Job{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)
	return db
}

// countPending returns number of jobs with given status.
func countStatus(t *testing.T, db *gorm.DB, status models.JobStatus) int64 {
	t.Helper()
	var n int64
	db.Model(&models.Job{}).Where("status = ?", status).Count(&n)
	return n
}

// successHandler always returns nil.
func successHandler(context.Context, *gorm.DB, *models.Job) error { return nil }

// failHandler always returns a deadline error.
func failHandler(context.Context, *gorm.DB, *models.Job) error { return context.DeadlineExceeded }

func TestEnqueueAndClaimCycle(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	err := Enqueue(ctx, db, "test_type", map[string]string{"foo": "bar"},
		WithPriority(0), WithMaxAttempts(3))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	if n := countStatus(t, db, models.JobPending); n != 1 {
		t.Fatalf("expected 1 pending job, got %d", n)
	}

	handlers := map[string]Handler{"test_type": successHandler}
	w := NewWorker(db, handlers, WorkerOptions{Interval: time.Hour})
	job := w.claimNextJob(ctx)
	if job == nil {
		t.Fatal("expected to claim a job, got nil")
	}
	if job.Type != "test_type" {
		t.Fatalf("expected type test_type, got %s", job.Type)
	}
	if job.Attempts != 1 {
		t.Fatalf("expected attempts=1 after claim, got %d", job.Attempts)
	}
	if job.Status != models.JobActive {
		t.Fatalf("expected status active, got %s", job.Status)
	}

	w.handleJob(ctx, job)

	if n := countStatus(t, db, models.JobDone); n != 1 {
		t.Fatalf("expected 1 done job, got %d", n)
	}
}

func TestEnqueueAndClaimNoJobs(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()
	w := NewWorker(db, map[string]Handler{}, WorkerOptions{Interval: time.Hour})

	job := w.claimNextJob(ctx)
	if job != nil {
		t.Fatal("expected nil when no pending jobs")
	}
}

func TestRetryBackoff(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	err := Enqueue(ctx, db, "failing_type", "payload", WithMaxAttempts(3))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	handlers := map[string]Handler{"failing_type": failHandler}
	w := NewWorker(db, handlers, WorkerOptions{Interval: time.Hour})

	job1 := w.claimNextJob(ctx)
	if job1 == nil {
		t.Fatal("expected to claim job on attempt 1")
	}
	w.handleJob(ctx, job1)

	// Should still be pending (attempts=1 < maxAttempts=3), run_at bumped
	var j models.Job
	db.First(&j)
	if j.Status != models.JobPending {
		t.Fatalf("expected pending after retry, got %s", j.Status)
	}
	if j.Attempts != 1 {
		t.Fatalf("expected attempts=1, got %d", j.Attempts)
	}
	// run_at should be ~10s in future (2^1 * 5s)
	if !j.RunAt.After(time.Now()) {
		t.Fatal("expected run_at in future after backoff")
	}
}

func TestMaxAttemptsExhaustion(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	err := Enqueue(ctx, db, "fail_fast", "payload", WithMaxAttempts(2))
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	callCount := 0
	handlers := map[string]Handler{
		"fail_fast": func(ctx context.Context, db *gorm.DB, job *models.Job) error {
			callCount++
			return context.DeadlineExceeded
		},
	}
	w := NewWorker(db, handlers, WorkerOptions{Interval: time.Hour})

	// Attempt 1 (of 2)
	job1 := w.claimNextJob(ctx)
	w.handleJob(ctx, job1)

	// Simulate backoff passing by resetting run_at to now
	var firstJob models.Job
	db.First(&firstJob)
	db.Model(&firstJob).Update("run_at", time.Now())

	// Attempt 2 (of 2) — should exhaust → failed
	job2 := w.claimNextJob(ctx)
	w.handleJob(ctx, job2)

	var j models.Job
	db.First(&j)
	if j.Status != models.JobFailed {
		t.Fatalf("expected failed after max attempts, got %s", j.Status)
	}
	if j.Attempts != 2 {
		t.Fatalf("expected attempts=2, got %d", j.Attempts)
	}
	if callCount != 2 {
		t.Fatalf("expected handler called 2 times, got %d", callCount)
	}
}

func TestCleanupJobs(t *testing.T) {
	db := openTestDB(t)

	oldJob := &models.Job{
		ID:        "old-done",
		Type:      "test",
		Status:    models.JobDone,
		UpdatedAt: time.Now().Add(-10 * 24 * time.Hour),
		RunAt:     time.Now().Add(-10 * 24 * time.Hour),
	}
	if err := db.Create(oldJob).Error; err != nil {
		t.Fatalf("insert old job: %v", err)
	}

	recentJob := &models.Job{
		ID:        "recent-done",
		Type:      "test",
		Status:    models.JobDone,
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		RunAt:     time.Now().Add(-1 * time.Hour),
	}
	if err := db.Create(recentJob).Error; err != nil {
		t.Fatalf("insert recent job: %v", err)
	}

	pendingJob := &models.Job{
		ID:     "pending",
		Type:   "test",
		Status: models.JobPending,
		RunAt:  time.Now(),
	}
	if err := db.Create(pendingJob).Error; err != nil {
		t.Fatalf("insert pending job: %v", err)
	}

	CleanupJobs(db, 7*24*time.Hour)

	var remaining int64
	db.Model(&models.Job{}).Count(&remaining)
	if remaining != 2 {
		t.Fatalf("expected 2 remaining jobs, got %d", remaining)
	}

	var found int64
	db.Model(&models.Job{}).Where("id = ?", "old-done").Count(&found)
	if found != 0 {
		t.Fatal("old-done should have been deleted")
	}
}

func TestCleanupFailedJobs(t *testing.T) {
	db := openTestDB(t)

	oldFailed := &models.Job{
		ID:        "old-failed",
		Type:      "test",
		Status:    models.JobFailed,
		UpdatedAt: time.Now().Add(-40 * 24 * time.Hour),
		RunAt:     time.Now().Add(-40 * 24 * time.Hour),
	}
	db.Create(oldFailed)

	recentFailed := &models.Job{
		ID:        "recent-failed",
		Type:      "test",
		Status:    models.JobFailed,
		UpdatedAt: time.Now().Add(-1 * time.Hour),
		RunAt:     time.Now().Add(-1 * time.Hour),
	}
	db.Create(recentFailed)

	CleanupFailedJobs(db, 30*24*time.Hour)

	var remaining int64
	db.Model(&models.Job{}).Count(&remaining)
	if remaining != 1 {
		t.Fatalf("expected 1 remaining failed job, got %d", remaining)
	}

	var found int64
	db.Model(&models.Job{}).Where("id = ?", "old-failed").Count(&found)
	if found != 0 {
		t.Fatal("old-failed should have been deleted")
	}
}

func TestQueueDepthCap(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	origCap := maxQueueDepth
	maxQueueDepth = 2
	defer func() { maxQueueDepth = origCap }()

	// Insert 2 pending jobs directly (bypass Enqueue cap)
	for i := range 2 {
		id := string(rune('a' + i))
		db.Create(&models.Job{
			ID:     "preload-" + id,
			Type:   "test",
			Status: models.JobPending,
			RunAt:  time.Now(),
		})
	}

	err := Enqueue(ctx, db, "overload", "payload")
	if err == nil {
		t.Fatal("expected error when queue depth limit reached")
	}
	t.Logf("got expected error: %v", err)
}

func TestWorkerGracefulShutdown(t *testing.T) {
	db := openTestDB(t)
	ctx, cancel := context.WithCancel(context.Background())

	blockCh := make(chan struct{})
	handlers := map[string]Handler{
		"blocking": func(ctx context.Context, db *gorm.DB, job *models.Job) error {
			<-blockCh
			return nil
		},
	}
	w := NewWorker(db, handlers, WorkerOptions{Interval: 50 * time.Millisecond, Concurrency: 1})

	Enqueue(ctx, db, "blocking", "payload")
	w.Start(ctx)

	time.Sleep(200 * time.Millisecond) // let worker claim and start handling

	cancel()       // trigger shutdown
	close(blockCh) // unblock handler

	time.Sleep(100 * time.Millisecond)

	var j models.Job
	db.First(&j)
	t.Logf("job status after shutdown: %s", j.Status)
}

func TestRecoverStaleJobs(t *testing.T) {
	db := openTestDB(t)

	stale := &models.Job{
		ID:        "stale-active",
		Type:      "test",
		Status:    models.JobActive,
		UpdatedAt: time.Now().Add(-15 * time.Minute),
		RunAt:     time.Now().Add(-15 * time.Minute),
	}
	db.Create(stale)

	w := NewWorker(db, map[string]Handler{}, WorkerOptions{})
	w.recoverStaleJobs()

	var j models.Job
	db.First(&j)
	if j.Status != models.JobPending {
		t.Fatalf("expected stale job recovered to pending, got %s", j.Status)
	}
}

func TestRecoverStaleJobsSkipsRecent(t *testing.T) {
	db := openTestDB(t)

	recent := &models.Job{
		ID:        "recent-active",
		Type:      "test",
		Status:    models.JobActive,
		UpdatedAt: time.Now().Add(-1 * time.Minute),
		RunAt:     time.Now().Add(-1 * time.Minute),
	}
	db.Create(recent)

	w := NewWorker(db, map[string]Handler{}, WorkerOptions{})
	w.recoverStaleJobs()

	var j models.Job
	db.First(&j)
	if j.Status != models.JobActive {
		t.Fatalf("expected recent active job left untouched, got %s", j.Status)
	}
}

func TestContextTimeoutInHandleJob(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	err := Enqueue(ctx, db, "slow_job", "payload")
	if err != nil {
		t.Fatalf("Enqueue: %v", err)
	}

	handlers := map[string]Handler{
		"slow_job": func(ctx context.Context, db *gorm.DB, job *models.Job) error {
			<-ctx.Done()
			return ctx.Err()
		},
	}
	w := NewWorker(db, handlers, WorkerOptions{Interval: time.Hour})

	origTimeout := jobTimeout
	jobTimeout = 50 * time.Millisecond
	defer func() { jobTimeout = origTimeout }()

	job := w.claimNextJob(ctx)
	if job == nil {
		t.Fatal("expected to claim job")
	}

	start := time.Now()
	w.handleJob(ctx, job)
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("handleJob should have timed out quickly, took %v", elapsed)
	}

	var j models.Job
	db.First(&j)
	t.Logf("timed-out job status: %s, attempts: %d", j.Status, j.Attempts)
}

func TestNoHandlerRegistered(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	db.Create(&models.Job{
		ID:     "no-handler",
		Type:   "unknown_type",
		Status: models.JobPending,
		RunAt:  time.Now(),
	})

	w := NewWorker(db, map[string]Handler{}, WorkerOptions{Interval: time.Hour})
	job := w.claimNextJob(ctx)
	if job == nil {
		t.Fatal("expected to claim job")
	}

	w.handleJob(ctx, job)

	var j models.Job
	db.First(&j)
	if j.Status != models.JobFailed {
		t.Fatalf("expected failed for unregistered handler, got %s", j.Status)
	}
	if j.LastError != "no handler registered" {
		t.Fatalf("expected 'no handler registered' error, got %s", j.LastError)
	}
}

func TestPanicRecovery(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	db.Create(&models.Job{
		ID:          "panic-job",
		Type:        "panic_type",
		Status:      models.JobPending,
		RunAt:       time.Now(),
		MaxAttempts: 1,
	})

	handlers := map[string]Handler{
		"panic_type": func(ctx context.Context, db *gorm.DB, job *models.Job) error {
			panic("test panic")
		},
	}
	w := NewWorker(db, handlers, WorkerOptions{Interval: time.Hour})
	job := w.claimNextJob(ctx)
	if job == nil {
		t.Fatal("expected to claim job")
	}

	w.handleJob(ctx, job)

	var j models.Job
	db.First(&j)
	if j.Status != models.JobFailed {
		t.Fatalf("expected failed after panic, got %s", j.Status)
	}
	if j.LastError != "panic: test panic" {
		t.Fatalf("expected 'panic: test panic' error, got %s", j.LastError)
	}
}

package queue

import (
	"bedrud/internal/models"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Handler processes a single claimed job. Return error to trigger retry, nil for success.
type Handler func(ctx context.Context, db *gorm.DB, job *models.Job) error

// EnqueueOptions carries optional parameters for Enqueue.
type EnqueueOptions struct {
	Priority    int
	MaxAttempts int
	RunAt       time.Time // zero = immediate
}

// EnqueueOption is a functional option for Enqueue.
type EnqueueOption func(*EnqueueOptions)

// WithPriority sets job priority (lower = higher priority, default 0).
func WithPriority(p int) EnqueueOption {
	return func(o *EnqueueOptions) { o.Priority = p }
}

// WithMaxAttempts sets max retry attempts (default 3).
func WithMaxAttempts(n int) EnqueueOption {
	return func(o *EnqueueOptions) { o.MaxAttempts = n }
}

// WithRunAt schedules the job for a future time (zero = immediate).
func WithRunAt(t time.Time) EnqueueOption {
	return func(o *EnqueueOptions) { o.RunAt = t }
}

func defaultOptions() *EnqueueOptions {
	return &EnqueueOptions{
		Priority:    0,
		MaxAttempts: 3,
	}
}

// maxQueueDepth caps total enqueued (pending + active) jobs to prevent unbounded growth.
// Bulk endpoints already cap at 500 per request; this is a safety net.
var maxQueueDepth = int64(10000)

func GetMaxDepth() int64 { return maxQueueDepth }

// Enqueue inserts a new job into the queue. payload must be JSON-serializable.
// Returns an error if queue depth exceeds maxQueueDepth.
func Enqueue(ctx context.Context, db *gorm.DB, jobType string, payload interface{}, opts ...EnqueueOption) error {
	// Check queue depth cap before inserting.
	var count int64
	if err := db.WithContext(ctx).Model(&models.Job{}).
		Where("status IN ?", []models.JobStatus{models.JobPending, models.JobActive}).
		Count(&count).Error; err != nil {
		return fmt.Errorf("queue depth check: %w", err)
	}
	if count >= maxQueueDepth {
		return fmt.Errorf("queue depth limit reached (%d/%d), refusing enqueue", count, maxQueueDepth)
	}

	cfg := defaultOptions()
	for _, o := range opts {
		o(cfg)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	runAt := cfg.RunAt
	if runAt.IsZero() {
		runAt = time.Now()
	}

	job := &models.Job{
		ID:          uuid.New().String(),
		Type:        jobType,
		Payload:     string(payloadBytes),
		RunAt:       runAt,
		Priority:    cfg.Priority,
		Status:      models.JobPending,
		Attempts:    0,
		MaxAttempts: cfg.MaxAttempts,
	}

	return db.WithContext(ctx).Create(job).Error
}

// WorkerOptions configures the queue worker.
type WorkerOptions struct {
	Interval    time.Duration // poll interval, default 500ms
	Concurrency int           // worker goroutines, default 1
}

// Worker polls the jobs table and dispatches to registered handlers.
type Worker struct {
	db       *gorm.DB
	handlers map[string]Handler
	opts     WorkerOptions
	stopCh   chan struct{}
}

// NewWorker creates a new queue Worker with the given DB and handler map.
func NewWorker(db *gorm.DB, handlers map[string]Handler, opts WorkerOptions) *Worker {
	if opts.Interval <= 0 {
		opts.Interval = 500 * time.Millisecond
	}
	if opts.Concurrency <= 0 {
		opts.Concurrency = 1
	}
	return &Worker{
		db:       db,
		handlers: handlers,
		opts:     opts,
		stopCh:   make(chan struct{}),
	}
}

// Start launches worker goroutines.
func (w *Worker) Start(ctx context.Context) {
	// Recover stale active jobs from previous crashes.
	// Jobs in 'active' state for >10min (no heartbeat) are reset to 'pending'.
	w.recoverStaleJobs()

	for range w.opts.Concurrency {
		go w.run(ctx)
	}
	log.Info().Int("concurrency", w.opts.Concurrency).Dur("interval", w.opts.Interval).
		Msg("queue worker started")
}

// Stop signals all worker goroutines to exit.
func (w *Worker) Stop() {
	close(w.stopCh)
}

func (w *Worker) run(ctx context.Context) {
	ticker := time.NewTicker(w.opts.Interval)
	defer ticker.Stop()

	for {
		// Drain all available jobs per tick, not just one
		for {
			select {
			case <-ctx.Done():
				return
			case <-w.stopCh:
				return
			default:
			}

			job := w.claimNextJob(ctx)
			if job == nil {
				break // no more jobs this tick
			}
			w.handleJob(ctx, job)
		}

		select {
		case <-ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
		}
	}
}

package queue

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"bedrud/internal/models"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// Handler processes a single claimed job. Return error to trigger retry, nil for success.
type Handler func(ctx context.Context, db *gorm.DB, job *models.Job) error

// ErrDuplicateJob means an unfinished job with the same type and dedupe key is
// already queued, so this one was not inserted. Callers that surface it to a
// user typically map it to 409.
var ErrDuplicateJob = errors.New("a job for this target is already queued")

// EnqueueOptions carries optional parameters for Enqueue.
type EnqueueOptions struct {
	Priority    int
	MaxAttempts int
	RunAt       time.Time // zero = immediate
	DedupeKey   string    // empty = no deduplication
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

// WithDedupeKey refuses the enqueue while an unfinished job of the same type
// carries the same key, returning ErrDuplicateJob.
//
// Key on the identity of the *operation*, not merely of its target: include
// whatever the job will do whenever one job type covers several outcomes.
// Keying on the target alone makes a queued job absorb a later request that
// would have done something different — and the caller is told it succeeded.
// See roomDeleteDedupeKey in internal/handlers for the worked example: an
// archive and a purge share the room_delete type, so the key carries which one.
func WithDedupeKey(key string) EnqueueOption {
	return func(o *EnqueueOptions) { o.DedupeKey = key }
}

// unfinished is the set of statuses that still hold a target: a job in either
// state may yet act, so a second job for the same target would duplicate it.
var unfinished = []models.JobStatus{models.JobPending, models.JobActive}

// hasUnfinishedJob reports whether a job of this type is already queued or
// running for the given dedupe key.
func hasUnfinishedJob(ctx context.Context, db *gorm.DB, jobType, dedupeKey string) (bool, error) {
	var count int64
	err := db.WithContext(ctx).Model(&models.Job{}).
		Where("type = ? AND dedupe_key = ? AND status IN ?", jobType, dedupeKey, unfinished).
		Count(&count).Error
	return count > 0, err
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

	if cfg.DedupeKey != "" {
		dup, err := hasUnfinishedJob(ctx, db, jobType, cfg.DedupeKey)
		if err != nil {
			return fmt.Errorf("dedupe check: %w", err)
		}
		if dup {
			return ErrDuplicateJob
		}
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
		DedupeKey:   cfg.DedupeKey,
	}

	if err := db.WithContext(ctx).Create(job).Error; err != nil {
		// The check above loses to a concurrent enqueue often enough to matter;
		// idx_jobs_active_dedupe is what actually decides. Ask the database
		// which case this was rather than parsing a driver-specific message.
		if cfg.DedupeKey != "" {
			if dup, qErr := hasUnfinishedJob(ctx, db, jobType, cfg.DedupeKey); qErr == nil && dup {
				// Keep the original error visible: an insert that failed for an
				// unrelated reason while a duplicate happened to exist would
				// otherwise be reported as a plain duplicate and lost.
				log.Warn().Err(err).Str("type", jobType).Str("dedupeKey", cfg.DedupeKey).
					Msg("queue: insert failed and a duplicate exists — reporting as duplicate")
				return ErrDuplicateJob
			}
		}
		return err
	}
	return nil
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

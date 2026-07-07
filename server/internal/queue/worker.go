package queue

import (
	"bedrud/internal/models"
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// claimNextJob claims the next pending job using DB-specific locking.
//
// PostgreSQL: single round-trip UPDATE with FOR UPDATE SKIP LOCKED via RETURNING.
// SQLite: two-step (UPDATE LIMIT 1 then SELECT). Safe because database.go
// sets SetMaxOpenConns(1) for SQLite — no concurrent writer.
// If Concurrency > 1 in future on SQLite, wrap SQLite path in a
// transaction with BEGIN IMMEDIATE.
func (w *Worker) claimNextJob(ctx context.Context) *models.Job {
	now := time.Now()
	switch w.db.Dialector.Name() {
	case "postgres":
		return w.claimPostgres(ctx, now)
	default:
		return w.claimSQLite(ctx, now)
	}
}

func (w *Worker) claimPostgres(ctx context.Context, now time.Time) *models.Job {
	var job models.Job
	err := w.db.WithContext(ctx).Raw(`
		UPDATE jobs
		SET status = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = ? AND run_at <= ?
			ORDER BY priority ASC, run_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING *`,
		models.JobActive,
		now,
		models.JobPending,
		now,
	).Scan(&job).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil
		}
		log.Error().Err(err).Msg("queue: claimPostgres failed")
		return nil
	}
	if job.ID == "" {
		return nil
	}
	return &job
}

func (w *Worker) claimSQLite(ctx context.Context, now time.Time) *models.Job {
	result := w.db.WithContext(ctx).Exec(`
		UPDATE jobs
		SET status = ?, attempts = attempts + 1, updated_at = ?
		WHERE id = (
			SELECT id FROM jobs
			WHERE status = ? AND run_at <= ?
			ORDER BY priority ASC, run_at ASC
			LIMIT 1
		)`,
		models.JobActive,
		now,
		models.JobPending,
		now,
	)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("queue: claimSQLite UPDATE failed")
		return nil
	}
	if result.RowsAffected == 0 {
		return nil
	}

	var job models.Job
	if err := w.db.WithContext(ctx).
		Where("status = ?", models.JobActive).
		Order("updated_at DESC").
		First(&job).Error; err != nil {
		return nil
	}
	return &job
}

// jobTimeout limits how long a single job handler may run before cancellation.
var jobTimeout = 5 * time.Minute

// handleJob dispatches to the registered handler with panic recovery and a deadline.
func (w *Worker) handleJob(ctx context.Context, job *models.Job) {
	defer func() {
		if r := recover(); r != nil {
			log.Error().
				Interface("panic", r).
				Str("jobID", job.ID).
				Str("type", job.Type).
				Msg("queue: job panicked")
			w.markError(job, fmt.Sprintf("panic: %v", r))
		}
	}()

	handler, ok := w.handlers[job.Type]
	if !ok {
		// No handler registered — job can never succeed. Fail permanently, skip retries.
		log.Error().Str("type", job.Type).Msg("queue: no handler registered for job type")
		w.markPermanentFail(job, "no handler registered")
		return
	}

	// Apply deadline so hanging jobs don't block worker goroutines indefinitely.
	handlerCtx, cancel := context.WithTimeout(ctx, jobTimeout)
	defer cancel()

	if err := handler(handlerCtx, w.db, job); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			log.Warn().Str("jobID", job.ID).Str("type", job.Type).
				Dur("timeout", jobTimeout).Msg("queue: job timed out")
		}
		log.Warn().Err(err).Str("jobID", job.ID).Str("type", job.Type).
			Int("attempts", job.Attempts).Int("maxAttempts", job.MaxAttempts).
			Msg("queue: job failed")
		w.markError(job, err.Error())
		return
	}

	w.markDone(job)
}

// markDone sets the job status to done.
func (w *Worker) markDone(job *models.Job) {
	if err := w.db.Model(job).Updates(map[string]interface{}{
		"status":     models.JobDone,
		"updated_at": time.Now(),
	}).Error; err != nil {
		log.Error().Err(err).Str("jobID", job.ID).Msg("queue: markDone failed")
	}
}

// markError implements exponential backoff for retries.
//
// Backoff formula: now + (2^attempts * 5s), capped at 1 hour.
// When attempts >= maxAttempts, job moves to failed status.
func (w *Worker) markError(job *models.Job, errMsg string) {
	updates := map[string]interface{}{
		"last_error": errMsg,
		"updated_at": time.Now(),
	}

	if job.Attempts >= job.MaxAttempts {
		updates["status"] = models.JobFailed
	} else {
		// Exponential backoff: 2^attempts * 5s, cap at 1h
		backoff := time.Duration(math.Pow(2, float64(job.Attempts))) * 5 * time.Second
		maxBackoff := 1 * time.Hour
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
		updates["run_at"] = time.Now().Add(backoff)
		// Reset to pending so worker picks it up again after backoff.
		// claimNextJob set status=active; without this it stays active forever.
		updates["status"] = models.JobPending
	}

	if err := w.db.Model(job).Updates(updates).Error; err != nil {
		log.Error().Err(err).Str("jobID", job.ID).Msg("queue: markError failed")
	}
}

// markPermanentFail sets the job to failed status immediately, bypassing retries.
// Used for unrecoverable errors (e.g., no handler registered).
func (w *Worker) markPermanentFail(job *models.Job, errMsg string) {
	if err := w.db.Model(job).Updates(map[string]interface{}{
		"status":     models.JobFailed,
		"last_error": errMsg,
		"updated_at": time.Now(),
	}).Error; err != nil {
		log.Error().Err(err).Str("jobID", job.ID).Msg("queue: markPermanentFail failed")
	}
}

// recoverStaleJobs resets jobs stuck in 'active' for >10 minutes back to 'pending'.
// Handles worker crashes where claimNextJob set status=active but handleJob
// never completed (no heartbeat mechanism yet).
func (w *Worker) recoverStaleJobs() {
	cutoff := time.Now().Add(-10 * time.Minute)
	result := w.db.Model(&models.Job{}).
		Where("status = ? AND updated_at < ?", models.JobActive, cutoff).
		Update("status", models.JobPending)
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("queue: recoverStaleJobs failed")
	} else if result.RowsAffected > 0 {
		log.Warn().Int64("count", result.RowsAffected).Msg("queue: recovered stale active jobs")
	}
}

// CleanupJobs removes completed jobs older than cutoff duration.
func CleanupJobs(db *gorm.DB, cutoff time.Duration) {
	result := db.Where("status = ? AND updated_at < ?",
		models.JobDone, time.Now().Add(-cutoff)).
		Delete(&models.Job{})
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("queue: CleanupJobs failed")
	} else if result.RowsAffected > 0 {
		log.Info().Int64("count", result.RowsAffected).Msg("queue: cleaned up old done jobs")
	}
}

// CleanupFailedJobs removes permanently failed jobs older than cutoff duration.
func CleanupFailedJobs(db *gorm.DB, cutoff time.Duration) {
	result := db.Where("status = ? AND updated_at < ?",
		models.JobFailed, time.Now().Add(-cutoff)).
		Delete(&models.Job{})
	if result.Error != nil {
		log.Error().Err(result.Error).Msg("queue: CleanupFailedJobs failed")
	} else if result.RowsAffected > 0 {
		log.Info().Int64("count", result.RowsAffected).Msg("queue: cleaned up old failed jobs")
	}
}

package handlers

import (
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

type AdminQueueHandler struct {
	db *gorm.DB
}

func NewAdminQueueHandler(db *gorm.DB) *AdminQueueHandler {
	return &AdminQueueHandler{db: db}
}

// GetQueueStats returns real-time queue status, job counts, and failure diagnostics.
// Only accessible by superadmins.
//
// @Summary Queue statistics
// @Description Real-time queue status including pending/active/done/failed counts,
// @Description processed rates, and recent failure diagnostics.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.QueueStats
// @Failure 500 {object} map[string]string
// @Router /admin/queue [get]
func (h *AdminQueueHandler) GetQueueStats(c *fiber.Ctx) error {
	now := time.Now()
	cutoff24h := now.Add(-24 * time.Hour)
	cutoff5min := now.Add(-5 * time.Minute)

	var pending, active, done24h, failed24h, done5min, failed5min, total int64
	var oldestPending *time.Time
	var pendingEmail, failedEmail24h int64
	var lastSendError string
	var lastSendErrorAt *time.Time

	errCh := make(chan error, 12)
	var errCount int32

	queueQuery := func(fn func() error) {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic in queue query: %v", r)
			}
		}()
		errCh <- fn()
	}

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).Where("status = ?", models.JobPending).Count(&pending).Error
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).Where("status = ?", models.JobActive).Count(&active).Error
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("status = ? AND updated_at > ?", models.JobDone, cutoff24h).Count(&done24h).Error
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("status = ? AND updated_at > ?", models.JobFailed, cutoff24h).Count(&failed24h).Error
	})

	go queueQuery(func() error {
		var minStr string
		err := h.db.Raw("SELECT COALESCE(MIN(run_at), '') FROM jobs WHERE status = ?", models.JobPending).Scan(&minStr).Error
		if err == nil && minStr != "" {
			const sqliteTimeLayout = "2006-01-02 15:04:05.999999999-07:00"
			if t, parseErr := time.Parse(sqliteTimeLayout, minStr); parseErr == nil {
				oldestPending = &t
			} else {
				log.Warn().Str("raw", minStr).Err(parseErr).Msg("Queue stats: failed to parse oldest pending time")
			}
		}
		return err
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("status = ? AND updated_at > ?", models.JobDone, cutoff5min).Count(&done5min).Error
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("status = ? AND updated_at > ?", models.JobFailed, cutoff5min).Count(&failed5min).Error
	})

	// Email-specific stats
	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("type = ? AND status = ?", "send_email", models.JobPending).Count(&pendingEmail).Error
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).
			Where("type = ? AND status = ? AND updated_at > ?", "send_email", models.JobFailed, cutoff24h).Count(&failedEmail24h).Error
	})

	// Most recent failed send_email
	go queueQuery(func() error {
		var lastFail struct {
			LastError string
			UpdatedAt time.Time
		}
		if err := h.db.Model(&models.Job{}).
			Where("type = ? AND status = ?", "send_email", models.JobFailed).
			Order("updated_at DESC").Limit(1).Scan(&lastFail).Error; err == nil && lastFail.LastError != "" {
			lastSendError = lastFail.LastError
			lastSendErrorAt = &lastFail.UpdatedAt
		}
		return nil
	})

	go queueQuery(func() error {
		return h.db.Model(&models.Job{}).Count(&total).Error
	})

	// Collect results from all goroutines (must match the count of go queueQuery calls above)
	const numQueries = 11
	for range numQueries {
		if err := <-errCh; err != nil {
			log.Error().Err(err).Msg("Queue stats: count query failed")
			errCount++
		}
	}

	// If more than half of the queries failed, the DB is likely unreachable.
	if errCount >= 5 {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to query queue statistics: database error"})
	}

	// Recent failures
	var recentJobs []struct {
		ID        string
		Type      string
		LastError string
		Attempts  int
		UpdatedAt time.Time
	}
	if err := h.db.Model(&models.Job{}).
		Select("id, type, last_error, attempts, updated_at").
		Where("status = ?", models.JobFailed).
		Order("updated_at DESC").
		Limit(10).
		Scan(&recentJobs).Error; err != nil {
		log.Error().Err(err).Msg("Queue stats: recent failures query failed")
	}

	failures := make([]models.FailedJobSummary, 0, len(recentJobs))
	for _, j := range recentJobs {
		age := formatAge(now.Sub(j.UpdatedAt))
		failures = append(failures, models.FailedJobSummary{
			ID:        j.ID,
			Type:      j.Type,
			Error:     j.LastError,
			Attempts:  j.Attempts,
			UpdatedAt: j.UpdatedAt,
			Age:       age,
		})
	}

	// Rates
	var processedPerMin, failedPerMin float64
	if done5min > 0 {
		processedPerMin = float64(done5min) / 5.0
	}
	if failed5min > 0 {
		failedPerMin = float64(failed5min) / 5.0
	}

	// Fail rate as fraction (0-1)
	total24h := done24h + failed24h
	var failRate float64
	if total24h > 0 {
		failRate = float64(failed24h) / float64(total24h)
	}

	stats := models.QueueStats{
		Pending:         pending,
		Active:          active,
		Done24h:         done24h,
		Failed24h:       failed24h,
		Total:           total,
		MaxDepth:        queue.GetMaxDepth(),
		OldestPending:   oldestPending,
		RecentFailures:  failures,
		ProcessedPerMin: processedPerMin,
		FailedPerMin:    failedPerMin,
		FailRate:        failRate,

		PendingEmail:    pendingEmail,
		FailedEmail24h:  failedEmail24h,
		LastSendError:   lastSendError,
		LastSendErrorAt: lastSendErrorAt,
	}

	return c.JSON(stats)
}

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1m ago"
		}
		return fmt.Sprintf("%dm ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1h ago"
		}
		return fmt.Sprintf("%dh ago", h)
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1d ago"
		}
		return fmt.Sprintf("%dd ago", days)
	}
}

package handlers

import (
	"bedrud/config"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/utils"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/gofiber/fiber/v2"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

const (
	overviewDays   = 7
	versionDefault = "dev"
)

type AdminOverviewHandler struct {
	roomRepo     *repository.RoomRepository
	userRepo     *repository.UserRepository
	settingsRepo *repository.SettingsRepository
	lkCfg        *config.LiveKitConfig
	client       livekit.RoomService
	db           *gorm.DB
	startTime    time.Time
	version      string
}

func NewAdminOverviewHandler(
	roomRepo *repository.RoomRepository,
	userRepo *repository.UserRepository,
	settingsRepo *repository.SettingsRepository,
	lkCfg *config.LiveKitConfig,
	client livekit.RoomService,
	db *gorm.DB,
	startTime time.Time,
	version string,
) *AdminOverviewHandler {
	if version == "" {
		version = versionDefault
	}
	return &AdminOverviewHandler{
		roomRepo:     roomRepo,
		userRepo:     userRepo,
		settingsRepo: settingsRepo,
		lkCfg:        lkCfg,
		client:       client,
		db:           db,
		startTime:    startTime,
		version:      version,
	}
}

// GetOverview returns the admin dashboard overview: system health, KPIs, activity trend,
// room composition, attention items, recent signups/events, and instance info.
//
// @Summary Admin dashboard overview
// @Description Aggregated system stats for the admin dashboard: health status, KPI metrics,
// @Description 7-day activity trend, room composition breakdown, and recent user/room activity.
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Success 200 {object} models.OverviewResponse
// @Failure 500 {object} map[string]string
// @Router /admin/overview [get]
func (h *AdminOverviewHandler) GetOverview(c *fiber.Ctx) error {
	now := time.Now()
	weekAgo := now.Add(-overviewDays * 24 * time.Hour)

	// --- LiveKit real-time participant count ---
	// Prefer LiveKit's real data over DB room_participants (which goes stale on disconnect).
	var liveKitOnline int64 = -1 // -1 = not available
	var liveKitPublishers int64
	var lkErr error
	if h.client != nil && h.lkCfg != nil {
		lkCtx, authErr := lkutil.AuthContext(context.Background(), h.lkCfg.APIKey, h.lkCfg.APISecret, &lkauth.VideoGrant{RoomList: true})
		if authErr == nil {
			if resp, err := h.client.ListRooms(lkCtx, &livekit.ListRoomsRequest{}); err == nil {
				var total uint32
				var publishers uint32
				for _, r := range resp.Rooms {
					total += r.NumParticipants
					publishers += r.NumPublishers
				}
				liveKitOnline = int64(total)
				liveKitPublishers = int64(publishers)
			} else {
				lkErr = err
			}
		} else {
			lkErr = authErr
		}
	}

	// --- Parallel DB queries ---
	type overviewData struct {
		totalRooms      int64
		activeRooms     int64
		publicRooms     int64
		privateRooms    int64
		persistentRooms int64
		staleRooms      int64
		totalUsers      int64
		usersWeek       int64
		onlineNow       int64 // DB fallback (only used if LiveKit unavailable)
		queuePending    int64
		events          []models.RoomEvent
		recentUsers     []models.User
		activityDays    []models.DayCount
		participantDays []models.DayCount
		activeRoomDays  []models.DayCount
	}

	var data overviewData
	var mu sync.Mutex
	var wg sync.WaitGroup
	errCh := make(chan error, 15)

	// fetch runs fn in a goroutine with panic recovery.
	// Panics are converted to errors sent on errCh to prevent goroutine leaks.
	fetch := func(fn func() error) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					if err, ok := r.(error); ok {
						errCh <- err
					} else {
						errCh <- fmt.Errorf("panic: %v", r)
					}
				}
			}()
			if err := fn(); err != nil {
				errCh <- err
			}
		}()
	}

	fetch(func() error {
		v, err := h.roomRepo.CountRooms()
		mu.Lock()
		data.totalRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountActiveRooms()
		mu.Lock()
		data.activeRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountPublicRooms()
		mu.Lock()
		data.publicRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountPrivateRooms()
		mu.Lock()
		data.privateRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountPersistentRooms()
		mu.Lock()
		data.persistentRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountStaleRooms(48)
		mu.Lock()
		data.staleRooms = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.userRepo.CountUsersFiltered([]string{models.ProviderGuest})
		mu.Lock()
		data.totalUsers = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.userRepo.CountUsersSinceFiltered(weekAgo, []string{models.ProviderGuest})
		mu.Lock()
		data.usersWeek = v
		mu.Unlock()
		return err
	})

	// DB fallback for online count (only if LiveKit unavailable)
	if liveKitOnline < 0 {
		fetch(func() error {
			v, err := h.roomRepo.CountActiveParticipants()
			mu.Lock()
			data.onlineNow = v
			mu.Unlock()
			return err
		})
	}

	fetch(func() error {
		v, err := h.getQueuePendingCount()
		mu.Lock()
		data.queuePending = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.GetRecentRoomEvents(5)
		mu.Lock()
		data.events = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.userRepo.GetRecentUsers(5)
		mu.Lock()
		data.recentUsers = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountRoomsByDay(overviewDays)
		mu.Lock()
		data.activityDays = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountActiveParticipantsByDay(overviewDays)
		mu.Lock()
		data.participantDays = v
		mu.Unlock()
		return err
	})

	fetch(func() error {
		v, err := h.roomRepo.CountActiveRoomsByDay(overviewDays)
		mu.Lock()
		data.activeRoomDays = v
		mu.Unlock()
		return err
	})

	wg.Wait()
	close(errCh)

	// Drain ALL errors from channel (not just first one)
	var errs []error
	for e := range errCh {
		errs = append(errs, e)
	}
	if len(errs) > 0 {
		for _, e := range errs {
			log.Error().Err(e).Msg("Overview data fetch failure")
		}
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch overview data"})
	}

	// Use LiveKit count if available, else DB fallback
	onlineNow := liveKitOnline
	if onlineNow < 0 {
		onlineNow = data.onlineNow
	}

	health := h.buildHealth(lkErr)
	kpis := h.buildKPIs(data.totalUsers, data.usersWeek, onlineNow, liveKitPublishers, data.totalRooms, data.activeRooms, data.queuePending)
	trend := h.buildActivityTrend(data.activityDays, data.participantDays, data.activeRoomDays)
	comp := models.RoomComposition{
		Live:       int(data.activeRooms),
		Public:     int(data.publicRooms),
		Private:    int(data.privateRooms),
		Persistent: int(data.persistentRooms),
		Stale:      int(data.staleRooms),
	}

	return c.JSON(models.OverviewResponse{
		Health:          health,
		KPIs:            kpis,
		ActivityTrend:   trend,
		RoomComposition: comp,
		NeedsAttention:  h.buildNeedsAttention(data.staleRooms),
		RecentSignups:   h.buildRecentSignups(data.recentUsers),
		RecentEvents:    data.events,
		InstanceInfo:    h.buildInstanceInfo(),
	})
}

func (h *AdminOverviewHandler) getQueuePendingCount() (int64, error) {
	if h.db == nil {
		return 0, nil
	}
	var count int64
	err := h.db.Model(&models.Job{}).Where("status = ?", models.JobPending).Count(&count).Error
	return count, err
}

func (h *AdminOverviewHandler) buildHealth(lkErr error) models.OverviewHealth {
	health := models.OverviewHealth{
		Status:        "healthy",
		Realtime:      "connected",
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		DBStatus:      "connected",
	}

	// Actual DB ping check
	if h.db != nil {
		if sqlDB, err := h.db.DB(); err != nil {
			health.DBStatus = "error"
			health.Status = "degraded"
			health.AlertsCount++
		} else if err := sqlDB.Ping(); err != nil {
			health.DBStatus = "error"
			health.Status = "degraded"
			health.AlertsCount++
		}
	} else {
		health.DBStatus = "error"
		health.Status = "degraded"
		health.AlertsCount++
	}

	// Actual LiveKit health check
	if lkErr != nil {
		health.Realtime = "disconnected"
		health.Status = "degraded"
		health.AlertsCount++
	}

	settings, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to read settings for TLS health")
		// Don't degrade just for settings read failure — TLS is non-critical
		return health
	}

	if settings.ServerEnableTLS && settings.ServerCertFile != "" && settings.ServerKeyFile != "" {
		info, err := utils.ValidateTLSCertPair(settings.ServerCertFile, settings.ServerKeyFile)
		if err != nil {
			health.TLS = &models.TLSStatus{Enabled: true, Status: "error"}
			health.AlertsCount++
			health.Status = "degraded"
		} else {
			status := "valid"
			if info.DaysRemaining < 30 {
				status = "expiring"
				health.AlertsCount++
			}
			if info.DaysRemaining <= 0 {
				status = "expired"
				health.AlertsCount++
				health.Status = "degraded"
			}
			health.TLS = &models.TLSStatus{
				Enabled:       true,
				DaysRemaining: info.DaysRemaining,
				ExpiryDate:    info.NotAfter.Format(time.RFC3339),
				Status:        status,
			}
		}
	} else {
		health.TLS = &models.TLSStatus{Enabled: false, Status: "unknown"}
	}

	return health
}

func (h *AdminOverviewHandler) buildKPIs(totalUsers, usersWeek, onlineNow, publishersNow, totalRooms, activeRooms, queuePending int64) models.OverviewKPIs {
	prevUsers := totalUsers - usersWeek
	deltaPct := 0
	switch {
	case prevUsers > 0:
		deltaPct = int((usersWeek * 100) / prevUsers)
	case usersWeek > 0:
		// All users joined this week — 100% growth
		deltaPct = 100
	}

	return models.OverviewKPIs{
		TotalUsers: models.KpiEntry{
			Value:        int(totalUsers),
			Delta:        int(usersWeek),
			DeltaLabel:   "this week",
			DeltaPercent: deltaPct,
		},
		OnlineNow: models.KpiEntry{
			Value:     int(onlineNow),
			ActiveNow: int(onlineNow),
		},
		TotalRooms: models.KpiEntry{
			Value:     int(totalRooms),
			ActiveNow: int(activeRooms),
		},
		ActiveSessions: models.KpiEntry{
			Value:     int(publishersNow),
			ActiveNow: int(onlineNow),
		},
		PendingActions: models.KpiEntry{
			Value: int(queuePending),
		},
	}
}

func (h *AdminOverviewHandler) buildActivityTrend(roomDays, participantDays, activeRoomDays []models.DayCount) []models.DayActivity {
	roomsByDay := make(map[string]int)
	for _, d := range roomDays {
		roomsByDay[d.Date.Format("2006-01-02")] = d.Count
	}
	partsByDay := make(map[string]int)
	for _, d := range participantDays {
		partsByDay[d.Date.Format("2006-01-02")] = d.Count
	}
	activeRoomsByDay := make(map[string]int)
	for _, d := range activeRoomDays {
		activeRoomsByDay[d.Date.Format("2006-01-02")] = d.Count
	}

	now := time.Now()
	cutoff := now.Add(-overviewDays * 24 * time.Hour)
	var trend []models.DayActivity
	for i := range overviewDays {
		day := cutoff.Add(time.Duration(i) * 24 * time.Hour)
		key := day.Format("2006-01-02")
		trend = append(trend, models.DayActivity{
			Date:         key,
			RoomsCreated: roomsByDay[key],
			RoomsActive:  activeRoomsByDay[key],
			Participants: partsByDay[key],
		})
	}
	return trend
}

func (h *AdminOverviewHandler) buildNeedsAttention(staleRooms int64) []models.AttentionItem {
	var items []models.AttentionItem

	settings, err := h.settingsRepo.GetEffectiveSettings()
	if err == nil && settings.ServerEnableTLS && settings.ServerCertFile != "" && settings.ServerKeyFile != "" {
		info, err := utils.ValidateTLSCertPair(settings.ServerCertFile, settings.ServerKeyFile)
		if err == nil && info.DaysRemaining < 90 {
			severity := "info"
			if info.DaysRemaining < 30 {
				severity = "warning"
			}
			if info.DaysRemaining < 7 {
				severity = "error"
			}
			items = append(items, models.AttentionItem{
				Type:     "tls_expiry",
				Severity: severity,
				Message:  fmt.Sprintf("TLS cert expires in %d days", info.DaysRemaining),
				DaysLeft: info.DaysRemaining,
			})
		}
	}

	if staleRooms > 0 {
		msg := fmt.Sprintf("%d rooms with no activity for 48h", staleRooms)
		items = append(items, models.AttentionItem{
			Type:     "stale_room",
			Severity: "info",
			Message:  msg,
		})
	}

	if items == nil {
		items = []models.AttentionItem{}
	}
	return items
}

func (h *AdminOverviewHandler) buildRecentSignups(users []models.User) []models.RecentUser {
	signups := make([]models.RecentUser, 0, len(users))
	for _, u := range users {
		// Exclude guest users from recent signups
		if u.Provider == models.ProviderGuest {
			continue
		}
		signups = append(signups, models.RecentUser{
			ID:        u.ID,
			Name:      u.Name,
			Email:     u.Email,
			Provider:  u.Provider,
			CreatedAt: u.CreatedAt.Format(time.RFC3339),
		})
	}
	return signups
}

func (h *AdminOverviewHandler) buildInstanceInfo() models.InstanceInfo {
	return models.InstanceInfo{
		Name:          "bedrud",
		Version:       h.version,
		UptimeSeconds: int64(time.Since(h.startTime).Seconds()),
		StartedAt:     h.startTime.Format(time.RFC3339),
	}
}

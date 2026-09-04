package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

// The overview endpoint does not pass the repository's series through: it
// re-keys the three of them into maps and rebuilds an axis of its own. That
// axis used to come from a local time.Now(), so fixing the repository alone
// moved the key-space mismatch into this function and left the chart reading
// zero in exactly the same window. The days the endpoint emits are the ones the
// dashboard draws, so they are what this test states.
func TestAdminOverviewHandler_ActivityTrendCoversUTCDaysEndingToday(t *testing.T) {
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	handler := NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &config.LiveKitConfig{}, nil, db, time.Now(), "test")

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user-id",
			Email:    "admin@example.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		return c.Next()
	})
	app.Get("/admin/overview", handler.GetOverview)

	if err := db.Create(&models.User{
		ID: "trend-owner", Email: "trend-owner@ex.com", Name: "TrendOwner",
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	// Created now, through the production path, so the row carries the process
	// zone the way a live server's rows do.
	if _, err := roomRepo.CreateRoom("trend-owner", "trend-room", true, "standard", 0, &models.RoomSettings{}); err != nil {
		t.Fatalf("create room: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/admin/overview", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result models.OverviewResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Stated in UTC without consulting the code under test, so this fails in
	// every process zone rather than only west of UTC in the early hours.
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	want := make([]string, 7)
	for i := range want {
		want[i] = today.AddDate(0, 0, -(len(want) - 1 - i)).Format("2006-01-02")
	}

	if len(result.ActivityTrend) != len(want) {
		t.Fatalf("expected %d trend days, got %d", len(want), len(result.ActivityTrend))
	}
	for i, d := range result.ActivityTrend {
		if d.Date != want[i] {
			t.Fatalf("trend day %d is %s, want %s", i, d.Date, want[i])
		}
	}

	// The seeded room and its creator's join are today's, so they belong in the
	// last column. Zeros there with the dates right mean the endpoint's axis and
	// the repository's buckets are still in different key spaces.
	last := result.ActivityTrend[len(result.ActivityTrend)-1]
	if last.RoomsCreated != 1 || last.RoomsActive != 1 || last.Participants != 1 {
		t.Fatalf("expected 1 room created, 1 active, 1 participant on %s, got %d/%d/%d",
			last.Date, last.RoomsCreated, last.RoomsActive, last.Participants)
	}
}

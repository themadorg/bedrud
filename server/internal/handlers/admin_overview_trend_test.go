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
	"gorm.io/gorm"
)

// newOverviewApp wires the overview endpoint over a fresh database and returns
// the handle as well, so a test can seed through the repositories or reach past
// them when it needs a row the repositories would never write.
func newOverviewApp(t *testing.T) (*fiber.App, *gorm.DB, *repository.RoomRepository) {
	t.Helper()

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
	return app, db, roomRepo
}

func getOverview(t *testing.T, app *fiber.App) models.OverviewResponse {
	t.Helper()

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
	return result
}

// seedOverviewRoom creates one room through the production path, at "now", so
// the row carries the process zone the way a live server's rows do.
func seedOverviewRoom(t *testing.T, db *gorm.DB, repo *repository.RoomRepository, userID, name string) *models.Room {
	t.Helper()

	if err := db.Create(&models.User{
		ID: userID, Email: userID + "@ex.com", Name: userID,
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	room, err := repo.CreateRoom(userID, name, true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	return room
}

// The overview endpoint does not pass the repository's series through: it
// re-keys the three of them into maps and rebuilds an axis of its own. That
// axis used to come from a local time.Now(), so fixing the repository alone
// moved the key-space mismatch into this function and left the chart reading
// zero in exactly the same window. The days the endpoint emits are the ones the
// dashboard draws, so they are what this test states.
func TestAdminOverviewHandler_ActivityTrendCoversUTCDaysEndingToday(t *testing.T) {
	app, db, roomRepo := newOverviewApp(t)
	seedOverviewRoom(t, db, roomRepo, "trend-owner", "trend-room")

	result := getOverview(t, app)

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

// TestAdminOverviewHandler_SaysSoWhenTheChartsAreIncomplete covers the half a
// chart cannot show on its own.
//
// A row the database cannot read as a date is dropped from its bucket so that
// one bad timestamp does not take the endpoint down — which leaves a chart that
// looks complete and is not. An incomplete week and a quiet week draw the same
// picture, so the endpoint has to say which one this is.
func TestAdminOverviewHandler_SaysSoWhenTheChartsAreIncomplete(t *testing.T) {
	app, db, roomRepo := newOverviewApp(t)
	seedOverviewRoom(t, db, roomRepo, "attn-owner", "attn-good")
	corrupt := seedOverviewRoom(t, db, roomRepo, "attn-owner-2", "attn-corrupt")

	// Written straight past the driver, the way a hand-edited database or an
	// import from another tool would leave it. SQLite's column is text and
	// takes it; on Postgres the column type makes this unrepresentable.
	if err := db.Exec("UPDATE rooms SET created_at = 'garbage' WHERE id = ?", corrupt.ID).Error; err != nil {
		t.Fatalf("corrupt the timestamp: %v", err)
	}

	result := getOverview(t, app)

	var found bool
	for _, item := range result.NeedsAttention {
		if item.Type == "unreadable_timestamps" {
			found = true
			if item.Severity != "warning" {
				t.Fatalf("expected a warning, got severity %q", item.Severity)
			}
		}
	}
	if !found {
		t.Fatalf("nothing in needsAttention says the charts are incomplete: %+v", result.NeedsAttention)
	}

	// And the readable room is still drawn: the point is a degraded chart, not
	// a missing one.
	last := result.ActivityTrend[len(result.ActivityTrend)-1]
	if last.RoomsCreated != 1 {
		t.Fatalf("expected the readable room on %s, got %d", last.Date, last.RoomsCreated)
	}
}

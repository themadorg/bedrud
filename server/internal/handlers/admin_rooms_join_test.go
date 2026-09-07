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

// The admin rooms list LEFT JOINs users to resolve the owner, and every column
// it filters and sorts by was written unqualified. `rooms` and `users` share
// id, name, is_active, created_at and updated_at, so the moment that join is in
// play SQLite answers "ambiguous column name: created_at" and the endpoint
// returns 500.
//
// The join is added for the owner filter, for all four date filters, and for
// three of the six sort options — so those are 500s in production, not
// mis-filtered results. sort=createdBy has a second defect on top: the join is
// added by the guard above and then added again inside the switch, which makes
// users.id ambiguous against itself.
//
// Nothing reached these paths before: the suite only ever exercised the default
// sort with no filters, which is the one combination that never joins.
func TestAdminListRooms_JoiningFiltersDoNotBreakTheQuery(t *testing.T) {
	app := setupAdminDateFilterApp(t)

	cases := []struct {
		name  string
		query string
		want  int // rooms expected back
	}{
		{"owner filter", "?owner=Admin", 1},
		{"owner filter that matches nothing", "?owner=nobody-by-that-name", 0},
		{"sort by owner", "?sort=createdBy&order=asc", 1},
		{"sort by last activity", "?sort=lastActivityAt&order=desc", 1},
		{"sort by participant count", "?sort=participantsCount&order=desc", 1},
		// Search and status filter on columns that also exist on users, so they
		// only become ambiguous once something else forces the join.
		{"search alongside the owner join", "?owner=Admin&search=date-filter-room", 1},
		{"status alongside the owner join", "?owner=Admin&status=active", 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, count := roomsListStatusAndCount(t, app, tc.query)
			if status != http.StatusOK {
				t.Fatalf("GET /admin/rooms%s = %d, want 200", tc.query, status)
			}
			if count != tc.want {
				t.Errorf("GET /admin/rooms%s returned %d room(s), want %d", tc.query, count, tc.want)
			}
		})
	}
}

// Sorting by last activity carried a `?` in its ORDER BY, and Order() takes no
// bind arguments — so the placeholder reached the driver unfilled and every
// such request failed with "not enough args to execute query: want 1 got 0".
// That is independent of the join: it failed on both dialects, with or without
// one.
//
// Asserting a 200 is not enough on its own. A clause that sorted by nothing at
// all would satisfy it, so this pins the order two rooms come back in.
func TestAdminListRooms_SortByLastActivityOrders(t *testing.T) {
	app, roomRepo, db := setupAdminDateFilterAppWithRepo(t)

	older, err := roomRepo.CreateRoom("admin-user", "older-activity", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create older room: %v", err)
	}
	newer, err := roomRepo.CreateRoom("admin-user", "newer-activity", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create newer room: %v", err)
	}

	// CreateRoom already enrols the owner, so every room in this fixture has a
	// participant row. Pinning all three joined_at values leaves nothing to the
	// clock: the setup room is seeded at "now", which would otherwise sort
	// ahead of anything backdated and make the assertion depend on the date the
	// suite runs. Expectations are computed here, in UTC, without calling the
	// code under test.
	base := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	setupRoom := roomIDByName(t, db, "date-filter-room")
	for _, seed := range []struct {
		roomID string
		at     time.Time
	}{
		{setupRoom, base.Add(-48 * time.Hour)},
		{older.ID, base},
		{newer.ID, base.Add(48 * time.Hour)},
	} {
		res := db.Model(&models.RoomParticipant{}).
			Where("room_id = ? AND user_id = ?", seed.roomID, "admin-user").
			Update("joined_at", seed.at)
		if res.Error != nil {
			t.Fatalf("seed participant for %s: %v", seed.roomID, res.Error)
		}
		if res.RowsAffected != 1 {
			t.Fatalf("seed participant for %s updated %d rows, want 1 — the fixture's shape changed", seed.roomID, res.RowsAffected)
		}
	}

	want := []string{"newer-activity", "older-activity", "date-filter-room"}
	got := roomsListNames(t, app, "?sort=lastActivityAt&order=desc")
	if !sameOrder(got, want) {
		t.Errorf("descending by last activity gave %v, want %v", got, want)
	}

	want = []string{"date-filter-room", "older-activity", "newer-activity"}
	got = roomsListNames(t, app, "?sort=lastActivityAt&order=asc")
	if !sameOrder(got, want) {
		t.Errorf("ascending by last activity gave %v, want %v", got, want)
	}
}

func sameOrder(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func roomIDByName(t *testing.T, db *gorm.DB, name string) string {
	t.Helper()
	var room models.Room
	if err := db.Where("name = ?", name).First(&room).Error; err != nil {
		t.Fatalf("look up room %q: %v", name, err)
	}
	return room.ID
}

// The owner filter has to actually filter. A query that returns everything
// would pass a "did not 500" assertion on its own.
func TestAdminListRooms_OwnerFilterSelects(t *testing.T) {
	app := setupAdminDateFilterApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms?owner=admin@ex.com", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("owner=admin@ex.com = %d, want 200", resp.StatusCode)
	}

	status, count := roomsListStatusAndCount(t, app, "?owner=someone-else@ex.com")
	if status != http.StatusOK {
		t.Fatalf("owner=someone-else@ex.com = %d, want 200", status)
	}
	if count != 0 {
		t.Errorf("owner=someone-else@ex.com returned %d room(s), want 0 — the filter matched a room it does not own", count)
	}
}

// The fixture and readers below are shared with admin_date_filter_test.go. They
// live here because this file is the one that cannot compile without them.

func setupAdminDateFilterApp(t *testing.T) *fiber.App {
	t.Helper()
	app, _, _ := setupAdminDateFilterAppWithRepo(t)
	return app
}

func setupAdminDateFilterAppWithRepo(t *testing.T) (*fiber.App, *repository.RoomRepository, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, recordingRepo, nil, nil, nil, nil)

	claims := &auth.Claims{UserID: "admin-user", Email: "admin@ex.com", Name: "Admin", Accesses: []string{"superadmin"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Get("/admin/rooms", handler.AdminListRooms)

	if err := db.Create(&models.User{
		ID: "admin-user", Email: "admin@ex.com", Name: "Admin",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"superadmin"},
	}).Error; err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := roomRepo.CreateRoom("admin-user", "date-filter-room", true, "standard", 0, &models.RoomSettings{}); err != nil {
		t.Fatalf("create room: %v", err)
	}
	return app, roomRepo, db
}

func roomsListNames(t *testing.T, app *fiber.App, query string) []string {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms"+query, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request %q: %v", query, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET /admin/rooms%s = %d: %s", query, resp.StatusCode, body)
	}
	var payload struct {
		Rooms []struct {
			Name string `json:"name"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode %q: %v — body %s", query, err, body)
	}
	names := make([]string, len(payload.Rooms))
	for i, r := range payload.Rooms {
		names[i] = r.Name
	}
	return names
}

func roomsListStatusAndCount(t *testing.T, app *fiber.App, query string) (int, int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms"+query, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("request %q: %v", query, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return resp.StatusCode, 0
	}
	var payload struct {
		Rooms []struct {
			ID string `json:"id"`
		} `json:"rooms"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("decode %q: %v — body %s", query, err, body)
	}
	return resp.StatusCode, len(payload.Rooms)
}

package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// --- Recent Signups Tests (GET /admin/users/recent) ---

const nameAlice = "Alice"

func setupRecentSignupsTestApp(t *testing.T) (*fiber.App, *repository.UserRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	cleanupSvc := testCleanupSvc(t, roomRepo, storage.NewChatUploadTracker(db, t.TempDir(), nil))
	usersHandler := testUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc)

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
	app.Get("/admin/users/recent", usersHandler.ListRecentSignups)
	return app, userRepo
}

func seedUsers(t *testing.T, userRepo *repository.UserRepository) {
	t.Helper()
	// Use UTC midnight today as reference to avoid timezone/date-boundary flakiness
	today := time.Now().UTC().Truncate(24 * time.Hour)
	users := []*models.User{
		{ID: "u1", Email: "alice@ex.com", Name: nameAlice, Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}, CreatedAt: today.Add(10 * time.Hour)},  // today 10:00 UTC
		{ID: "u2", Email: "bob@ex.com", Name: "Bob", Provider: "google", IsActive: true, Accesses: models.StringArray{"user"}, CreatedAt: today.Add(9 * time.Hour)},        // today 09:00 UTC
		{ID: "u3", Email: "carol@ex.com", Name: "Carol", Provider: "github", IsActive: false, Accesses: models.StringArray{"user"}, CreatedAt: today.Add(-14 * time.Hour)}, // yesterday 10:00 UTC
		{ID: "u4", Email: "dan@ex.com", Name: "Dan", Provider: "guest", IsActive: true, Accesses: models.StringArray{"guest"}, CreatedAt: today.Add(-38 * time.Hour)},      // 2 days ago 10:00 UTC
		{ID: "u5", Email: "eve@ex.com", Name: "Eve", Provider: "local", IsActive: true, Accesses: models.StringArray{"admin"}, CreatedAt: today.Add(-62 * time.Hour)},      // 3 days ago 10:00 UTC
	}
	for _, u := range users {
		if err := userRepo.CreateUser(u); err != nil {
			t.Fatalf("failed to create user %s: %v", u.ID, err)
		}
	}
}

// Helper to decode paginated recent signups response
type recentSignupsResponse struct {
	Users []models.RecentUser `json:"users"`
	Total int                 `json:"total"`
	Page  int                 `json:"page"`
	Limit int                 `json:"limit"`
}

func TestRecentSignups_Empty(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Users == nil {
		t.Fatal("expected non-nil users array")
	}
	if len(result.Users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(result.Users))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
	if result.Limit != 50 {
		t.Fatalf("expected limit 50, got %d", result.Limit)
	}
}

func TestRecentSignups_WithUsers(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	if len(result.Users) != 4 {
		t.Fatalf("expected 4 users (guest excluded by default), got %d", len(result.Users))
	}
	if result.Total != 4 {
		t.Fatalf("expected total 4, got %d", result.Total)
	}
	// Default sort: createdAt desc — most recent first (Alice)
	if result.Users[0].Name != nameAlice {
		t.Fatalf("expected first user Alice, got %s", result.Users[0].Name)
	}
	// Each user should have all fields
	for _, u := range result.Users {
		if u.ID == "" || u.Name == "" || u.Email == "" || u.Provider == "" || u.CreatedAt == "" {
			t.Fatalf("incomplete user data: %+v", u)
		}
	}
}

func TestRecentSignups_Pagination(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// Page 1, limit 2
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?page=1&limit=2", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var p1 recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&p1); err != nil {
		t.Fatal(err)
	}
	if len(p1.Users) != 2 {
		t.Fatalf("expected 2 users on page 1, got %d", len(p1.Users))
	}
	if p1.Total != 4 {
		t.Fatalf("expected total 4 (guest excluded), got %d", p1.Total)
	}
	if p1.Page != 1 || p1.Limit != 2 {
		t.Fatalf("unexpected page/limit: %d/%d", p1.Page, p1.Limit)
	}

	// Page 2, limit 2 — should have 2 users
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?page=2&limit=2", http.NoBody)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()

	var p2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&p2); err != nil {
		t.Fatal(err)
	}
	if len(p2.Users) != 2 {
		t.Fatalf("expected 2 users on page 2, got %d", len(p2.Users))
	}

	// Page 3, limit 2 — should be empty
	req3 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?page=3&limit=2", http.NoBody)
	resp3, _ := app.Test(req3, -1)
	defer resp3.Body.Close()

	var p3 recentSignupsResponse
	if err := json.NewDecoder(resp3.Body).Decode(&p3); err != nil {
		t.Fatal(err)
	}
	if len(p3.Users) != 0 {
		t.Fatalf("expected 0 users on page 3, got %d", len(p3.Users))
	}
}

func TestRecentSignups_SearchFilter(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// Search by name
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?q=ali", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 1 || result.Users[0].Name != nameAlice {
		t.Fatalf("expected 1 user (Alice), got %d: %+v", len(result.Users), result.Users)
	}

	// Search by email
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?q=bob@ex", http.NoBody)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()

	var result2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Users) != 1 || result2.Users[0].Name != "Bob" {
		t.Fatalf("expected 1 user (Bob), got %d", len(result2.Users))
	}

	// Search with no match
	req3 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?q=zzzzz", http.NoBody)
	resp3, _ := app.Test(req3, -1)
	defer resp3.Body.Close()

	var result3 recentSignupsResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatal(err)
	}
	if len(result3.Users) != 0 {
		t.Fatalf("expected 0 users for non-matching search, got %d", len(result3.Users))
	}
}

func TestRecentSignups_ProviderFilter(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// Single provider
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=google", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 1 || result.Users[0].Name != "Bob" {
		t.Fatalf("expected 1 google user (Bob), got %d: %+v", len(result.Users), result.Users)
	}

	// Multiple providers
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=local,google", http.NoBody)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()

	var result2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Users) != 3 { // Alice (local), Bob (google), Eve (local)
		t.Fatalf("expected 3 users for local+google, got %d: %+v", len(result2.Users), result2.Users)
	}
}

func TestRecentSignups_InvalidProvider(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=invalid", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid provider, got %d", resp.StatusCode)
	}
}

func TestRecentSignups_DateFilter(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	today := time.Now().UTC().Format("2006-01-02")

	// dateFrom=today — should get Alice and Bob (created today at 10:00 and 09:00 UTC)
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?dateFrom="+today, http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 2 {
		t.Fatalf("expected 2 users from today, got %d: %+v", len(result.Users), result.Users)
	}

	// dateTo=yesterday — should include Carol, Dan, Eve
	yesterday := time.Now().UTC().Add(-24 * time.Hour).Format("2006-01-02")
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?dateTo="+yesterday, http.NoBody)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()

	var result2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Users) == 0 {
		t.Fatal("expected some older users")
	}
}

func TestRecentSignups_InvalidDate(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?dateFrom=not-a-date", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateFrom, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?dateTo=also-invalid", http.NoBody)
	resp2, _ := app.Test(req2, -1)
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateTo, got %d", resp2.StatusCode)
	}
}

func TestRecentSignups_InvalidSort(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?sort=invalid", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sort, got %d", resp.StatusCode)
	}
}

func TestRecentSignups_InvalidOrder(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?order=invalid", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid order, got %d", resp.StatusCode)
	}
}

func TestRecentSignups_LimitClamping(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// limit=200 should be clamped to 100 (the handler's max)
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?limit=200", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Limit != 100 && result.Limit != 50 {
		t.Fatalf("expected limit clamped to 100 or 50, got %d", result.Limit)
	}
}

func TestRecentSignups_NegativeLimit(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?limit=-5", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Limit != 50 {
		t.Fatalf("expected limit clamped to 50, got %d", result.Limit)
	}
}

func TestRecentSignups_ZeroPage(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// page=0 should default to 1
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?page=0", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
	// Should still return results
	if len(result.Users) == 0 {
		t.Fatal("expected users on page 1")
	}
}

func TestRecentSignups_CombinedFilters(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	today := time.Now().UTC().Format("2006-01-02")

	// Combined: provider=local + dateFrom=today
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=local&dateFrom="+today, http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	// local users created today: Alice, Bob is google, Carol is github, Dan is guest, Eve is local but 3 days ago
	// So only Alice should match (local + today)
	if len(result.Users) != 1 || result.Users[0].Name != nameAlice {
		t.Fatalf("expected 1 local user from today (Alice), got %d: %+v", len(result.Users), result.Users)
	}
}

func TestRecentSignups_SortByName(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// sort=name, order=asc — alphabetical
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?sort=name&order=asc", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 4 {
		t.Fatalf("expected 4 users (guest excluded), got %d", len(result.Users))
	}
	// Alphabetical: Alice, Bob, Carol, Eve (Dan is guest, excluded)
	expected := []string{nameAlice, "Bob", "Carol", "Eve"}
	for i, u := range result.Users {
		if u.Name != expected[i] {
			t.Fatalf("position %d: expected %s, got %s", i, expected[i], u.Name)
		}
	}
}

func TestRecentSignups_EmptyProviderParam(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// provider= with empty value should be treated as no filter
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 4 {
		t.Fatalf("expected 4 users (guest excluded by default), got %d", len(result.Users))
	}
}

// --- Room Events Tests (GET /admin/rooms/events) ---

type roomEventsResponse struct {
	Events []models.RoomEvent `json:"events"`
	Total  int                `json:"total"`
	Page   int                `json:"page"`
	Limit  int                `json:"limit"`
}

func setupRoomEventsTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *repository.UserRepository, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)

	// Use a minimal room handler setup
	roomHandler := &RoomHandler{
		roomRepo: roomRepo,
		userRepo: userRepo,
	}

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
	app.Get("/admin/rooms/events", roomHandler.ListRoomEvents)
	return app, roomRepo, userRepo, db
}

func seedRoomEvents(t *testing.T, roomRepo *repository.RoomRepository, userRepo *repository.UserRepository, db *gorm.DB) {
	t.Helper()
	// Create users
	for _, u := range []*models.User{
		{ID: "owner1", Email: "owner1@ex.com", Name: "RoomOwner", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}},
		{ID: "joiner1", Email: "joiner1@ex.com", Name: "Joiner", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}},
	} {
		if err := userRepo.CreateUser(u); err != nil {
			t.Fatalf("failed to create user: %v", err)
		}
	}

	// Create rooms via repo (returns *models.Room, error)
	r1, err := roomRepo.CreateRoom("owner1", "meeting-room", true, "standard", 10, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	r2, err := roomRepo.CreateRoom("owner1", "call-room", true, "standard", 20, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Add participants (creates room_joined events)
	if err := roomRepo.AddParticipant(r1.ID, "joiner1"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	// Set CreatedAt timestamps via raw DB since CreateRoom auto-sets them
	db.Model(&models.Room{}).Where("id = ?", r1.ID).Update("created_at", time.Now().Add(-1*time.Hour))
	db.Model(&models.Room{}).Where("id = ?", r2.ID).Update("created_at", time.Now().Add(-2*time.Hour))
}

func TestRoomEvents_Empty(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}
	if result.Events == nil {
		t.Fatal("expected non-nil events array")
	}
	if len(result.Events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(result.Events))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0, got %d", result.Total)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
	if result.Limit != 50 {
		t.Fatalf("expected limit 50, got %d", result.Limit)
	}
}

func TestRoomEvents_WithEvents(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(body))
	}

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode: %v", err)
	}

	// 2 room_created + 3 room_joined (2 owner auto-joins + 1 explicit join) = 5 events
	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events, got %d: %+v", len(result.Events), result.Events)
	}
	if result.Total != 5 {
		t.Fatalf("expected total 5, got %d", result.Total)
	}

	// Count types
	created := 0
	joined := 0
	for _, ev := range result.Events {
		if ev.Type == "room_created" {
			created++
			if ev.RoomName == "" {
				t.Fatal("room_created event missing roomName")
			}
		}
		if ev.Type == "room_joined" {
			joined++
		}
	}
	if created != 2 {
		t.Fatalf("expected 2 room_created events, got %d", created)
	}
	if joined != 3 {
		t.Fatalf("expected 3 room_joined events (2 auto-joins + 1 explicit), got %d", joined)
	}
}

// getRoomEvents issues the request and fails unless it comes back 200, then
// checks that every returned event carries a parsed timestamp.
//
// Both guards exist because their absence is invisible: a 500 body
// ({"error":"Failed to fetch room events"}) decodes cleanly into
// roomEventsResponse as {Events: nil, Total: 0}, so every "expect zero events"
// assertion below is also satisfied by a failing request; and a timestamp the
// repository cannot parse is dropped to the zero value rather than reported.
func getRoomEvents(t *testing.T, app *fiber.App, query string) roomEventsResponse {
	t.Helper()

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events"+query, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("GET %q: transport error: %v", query, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("GET %q: expected 200, got %d: %s", query, resp.StatusCode, string(body))
	}

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("GET %q: failed to decode: %v", query, err)
	}
	for i, ev := range result.Events {
		if ev.Timestamp.IsZero() {
			t.Fatalf("GET %q: event %d (%s on %q) came back with a zero timestamp", query, i, ev.Type, ev.RoomName)
		}
	}
	return result
}

func TestRoomEvents_Pagination(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	p1 := getRoomEvents(t, app, "?page=1&limit=2")
	if len(p1.Events) != 2 {
		t.Fatalf("expected 2 events on page 1, got %d", len(p1.Events))
	}
	if p1.Total != 5 {
		t.Fatalf("expected total 5, got %d", p1.Total)
	}

	p2 := getRoomEvents(t, app, "?page=2&limit=2")
	if len(p2.Events) != 2 {
		t.Fatalf("expected 2 events on page 2, got %d", len(p2.Events))
	}

	p3 := getRoomEvents(t, app, "?page=3&limit=2")
	if len(p3.Events) != 1 {
		t.Fatalf("expected 1 event on page 3, got %d", len(p3.Events))
	}

	p4 := getRoomEvents(t, app, "?page=4&limit=2")
	if len(p4.Events) != 0 {
		t.Fatalf("expected 0 events on page 4, got %d", len(p4.Events))
	}

	// Every page reports the same total, and the pages together account for it.
	// Admin pagination derives its page count from total, so a total that
	// disagrees with the rows makes the last page unreachable.
	for i, p := range []roomEventsResponse{p2, p3, p4} {
		if p.Total != p1.Total {
			t.Fatalf("page %d reports total %d, page 1 reports %d", i+2, p.Total, p1.Total)
		}
	}
	if got := len(p1.Events) + len(p2.Events) + len(p3.Events) + len(p4.Events); got != p1.Total {
		t.Fatalf("pages hold %d events but total is %d", got, p1.Total)
	}
}

func TestRoomEvents_TypeFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	created := getRoomEvents(t, app, "?type=room_created")
	if len(created.Events) != 2 {
		t.Fatalf("expected 2 room_created events, got %d", len(created.Events))
	}
	if created.Total != len(created.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", created.Total, len(created.Events))
	}

	joined := getRoomEvents(t, app, "?type=room_joined")
	if len(joined.Events) != 3 {
		t.Fatalf("expected 3 room_joined events (2 auto-joins + 1 explicit), got %d", len(joined.Events))
	}
	if joined.Total != len(joined.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", joined.Total, len(joined.Events))
	}

	both := getRoomEvents(t, app, "?type=room_created,room_joined")
	if len(both.Events) != 5 {
		t.Fatalf("expected 5 events for both types, got %d", len(both.Events))
	}
	if both.Total != len(both.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", both.Total, len(both.Events))
	}
}

func TestRoomEvents_InvalidType(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=invalid_type", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d", resp.StatusCode)
	}
}

func TestRoomEvents_SearchFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// The seed puts three events on meeting-room: the room_created row, the
	// creator's auto-join, and joiner1's join.
	byRoom := getRoomEvents(t, app, "?q=meeting")
	if len(byRoom.Events) != 3 {
		t.Fatalf("expected 3 events matching 'meeting', got %d", len(byRoom.Events))
	}
	// The page holds every match, so total has to agree with it. Count and data
	// are two reads of the same derived table; this is what catches them
	// drifting apart — the count used to project '' as room_name for
	// room_created rows and reported 2 here.
	if byRoom.Total != len(byRoom.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", byRoom.Total, len(byRoom.Events))
	}

	byUser := getRoomEvents(t, app, "?q=joiner")
	if len(byUser.Events) == 0 {
		t.Fatal("expected events matching 'joiner'")
	}
	if byUser.Total != len(byUser.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", byUser.Total, len(byUser.Events))
	}
}

func TestRoomEvents_DateFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	today := time.Now().Format("2006-01-02")

	fromToday := getRoomEvents(t, app, "?dateFrom="+today)
	if len(fromToday.Events) == 0 {
		t.Fatal("expected events from today")
	}

	// dateTo is an exclusive next-day bound computed in Go; it used to be
	// date(?, '+1 day'), a SQLite builtin that 500s on Postgres. The status
	// check in the helper is what stops that from reading as "0 events".
	old := getRoomEvents(t, app, "?dateTo=2010-01-01")
	if len(old.Events) != 0 {
		t.Fatalf("expected 0 events before 2010, got %d", len(old.Events))
	}
	if old.Total != 0 {
		t.Fatalf("expected total 0 before 2010, got %d", old.Total)
	}

	// dateTo is inclusive of its own day, which is what the next-day bound buys.
	throughToday := getRoomEvents(t, app, "?dateTo="+today)
	if len(throughToday.Events) != 5 {
		t.Fatalf("expected all 5 events through today, got %d", len(throughToday.Events))
	}
}

func TestRoomEvents_InvalidDate(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	for _, q := range []string{"?dateFrom=bad", "?dateTo=also-bad"} {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events"+q, http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("GET %q: unexpected error: %v", q, err)
		}
		if resp.StatusCode != http.StatusBadRequest {
			resp.Body.Close()
			t.Fatalf("GET %q: expected 400, got %d", q, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestRoomEvents_OrderAsc(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	result := getRoomEvents(t, app, "?order=asc")
	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(result.Events))
	}
	// asc means asc — checkable only because the timestamps actually parsed.
	for i := 1; i < len(result.Events); i++ {
		if result.Events[i].Timestamp.Before(result.Events[i-1].Timestamp) {
			t.Fatalf("events not in ascending order at %d: %s before %s",
				i, result.Events[i].Timestamp, result.Events[i-1].Timestamp)
		}
	}
}

func TestRoomEvents_InvalidOrder(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?order=invalid", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid order, got %d", resp.StatusCode)
	}
}

func TestRoomEvents_LimitClamping(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	result := getRoomEvents(t, app, "?limit=200")
	if result.Limit > 100 {
		t.Fatalf("expected limit clamped, got %d", result.Limit)
	}
}

func TestRoomEvents_NegativeLimit(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	result := getRoomEvents(t, app, "?limit=-5")
	if result.Limit != 50 {
		t.Fatalf("expected limit clamped to 50, got %d", result.Limit)
	}
}

func TestRoomEvents_ZeroPage(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	result := getRoomEvents(t, app, "?page=0")
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
}

func TestRoomEvents_DateFromAndTo(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	today := time.Now().Format("2006-01-02")

	result := getRoomEvents(t, app, "?dateFrom="+today+"&dateTo="+today)
	if len(result.Events) == 0 {
		t.Fatal("expected some events with dateFrom+dateTo both set to today")
	}
	if result.Total != len(result.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", result.Total, len(result.Events))
	}
}

func TestRoomEvents_DateFromReversed(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// dateFrom after dateTo — an empty range, not an error.
	result := getRoomEvents(t, app, "?dateFrom=2030-01-01&dateTo=2020-01-01")
	if len(result.Events) != 0 {
		t.Fatalf("expected 0 events for reversed date range, got %d", len(result.Events))
	}
	if result.Total != 0 {
		t.Fatalf("expected total 0 for reversed date range, got %d", result.Total)
	}
}

func TestRoomEvents_EmptyTypeParam(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// type= with empty value should be treated as no filter
	result := getRoomEvents(t, app, "?type=")
	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events (no filter), got %d", len(result.Events))
	}
}

func TestRoomEvents_CombinedFilters(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// type=room_joined + q=meeting — should only get join events for "meeting-room"
	result := getRoomEvents(t, app, "?type=room_joined&q=meeting")
	if len(result.Events) == 0 {
		t.Fatal("expected at least 1 join event for meeting-room")
	}
	if result.Total != len(result.Events) {
		t.Fatalf("total %d disagrees with %d events on one page", result.Total, len(result.Events))
	}
	for _, ev := range result.Events {
		if ev.Type != "room_joined" {
			t.Fatalf("expected all events to be room_joined, got %s", ev.Type)
		}
		if !strings.Contains(ev.RoomName, "meeting") {
			t.Fatalf("expected events for meeting-room, got %s", ev.RoomName)
		}
	}
}

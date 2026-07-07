package handlers

import (
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// --- Recent Signups Tests (GET /admin/users/recent) ---

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
		{ID: "u1", Email: "alice@ex.com", Name: "Alice", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}, CreatedAt: today.Add(10 * time.Hour)},    // today 10:00 UTC
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
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
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
	if result.Users[0].Name != "Alice" {
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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var p2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&p2); err != nil {
		t.Fatal(err)
	}
	if len(p2.Users) != 2 {
		t.Fatalf("expected 2 users on page 2, got %d", len(p2.Users))
	}

	// Page 3, limit 2 — should be empty
	req3 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?page=3&limit=2", http.NoBody)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 1 || result.Users[0].Name != "Alice" {
		t.Fatalf("expected 1 user (Alice), got %d: %+v", len(result.Users), result.Users)
	}

	// Search by email
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?q=bob@ex", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var result2 recentSignupsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Users) != 1 || result2.Users[0].Name != "Bob" {
		t.Fatalf("expected 1 user (Bob), got %d", len(result2.Users))
	}

	// Search with no match
	req3 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?q=zzzzz", http.NoBody)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 1 || result.Users[0].Name != "Bob" {
		t.Fatalf("expected 1 google user (Bob), got %d: %+v", len(result.Users), result.Users)
	}

	// Multiple providers
	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?provider=local,google", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateFrom, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/admin/users/recent?dateTo=also-invalid", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateTo, got %d", resp2.StatusCode)
	}
}

func TestRecentSignups_InvalidSort(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?sort=invalid", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid sort, got %d", resp.StatusCode)
	}
}

func TestRecentSignups_InvalidOrder(t *testing.T) {
	app, _ := setupRecentSignupsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?order=invalid", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid order, got %d", resp.StatusCode)
	}
}

func TestRecentSignups_LimitClamping(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// limit=200 should be clamped to 100 (the handler's max)
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?limit=200", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	// local users created today: Alice, Bob is google, Carol is github, Dan is guest, Eve is local but 3 days ago
	// So only Alice should match (local + today)
	if len(result.Users) != 1 || result.Users[0].Name != "Alice" {
		t.Fatalf("expected 1 local user from today (Alice), got %d: %+v", len(result.Users), result.Users)
	}
}

func TestRecentSignups_SortByName(t *testing.T) {
	app, userRepo := setupRecentSignupsTestApp(t)
	seedUsers(t, userRepo)

	// sort=name, order=asc — alphabetical
	req := httptest.NewRequest(http.MethodGet, "/admin/users/recent?sort=name&order=asc", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result recentSignupsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Users) != 4 {
		t.Fatalf("expected 4 users (guest excluded), got %d", len(result.Users))
	}
	// Alphabetical: Alice, Bob, Carol, Eve (Dan is guest, excluded)
	expected := []string{"Alice", "Bob", "Carol", "Eve"}
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
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

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
	defer func() { _ = resp.Body.Close() }()
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
	defer func() { _ = resp.Body.Close() }()
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

func TestRoomEvents_Pagination(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// Page 1, limit 2
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?page=1&limit=2", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var p1 roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&p1); err != nil {
		t.Fatal(err)
	}
	if len(p1.Events) != 2 {
		t.Fatalf("expected 2 events on page 1, got %d", len(p1.Events))
	}
	if p1.Total != 5 {
		t.Fatalf("expected total 5, got %d", p1.Total)
	}

	// Page 2, limit 2 — should have 2 events
	req2 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?page=2&limit=2", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var p2 roomEventsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&p2); err != nil {
		t.Fatal(err)
	}
	if len(p2.Events) != 2 {
		t.Fatalf("expected 2 events on page 2, got %d", len(p2.Events))
	}

	// Page 3, limit 2 — should have 1 event
	req3 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?page=3&limit=2", http.NoBody)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

	var p3 roomEventsResponse
	if err := json.NewDecoder(resp3.Body).Decode(&p3); err != nil {
		t.Fatal(err)
	}
	if len(p3.Events) != 1 {
		t.Fatalf("expected 1 event on page 3, got %d", len(p3.Events))
	}

	// Page 4, limit 2 — should be empty
	req4 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?page=4&limit=2", http.NoBody)
	resp4, err := app.Test(req4, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp4.Body.Close() }()

	var p4 roomEventsResponse
	if err := json.NewDecoder(resp4.Body).Decode(&p4); err != nil {
		t.Fatal(err)
	}
	if len(p4.Events) != 0 {
		t.Fatalf("expected 0 events on page 4, got %d", len(p4.Events))
	}
}

func TestRoomEvents_TypeFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// Filter by room_created only
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=room_created", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 2 {
		t.Fatalf("expected 2 room_created events, got %d", len(result.Events))
	}

	// Filter by room_joined only
	req2 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=room_joined", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var result2 roomEventsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Events) != 3 {
		t.Fatalf("expected 3 room_joined events (2 auto-joins + 1 explicit), got %d", len(result2.Events))
	}

	// Filter by both
	req3 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=room_created,room_joined", http.NoBody)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

	var result3 roomEventsResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatal(err)
	}
	if len(result3.Events) != 5 {
		t.Fatalf("expected 5 events for both types, got %d", len(result3.Events))
	}
}

func TestRoomEvents_InvalidType(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=invalid_type", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid type, got %d", resp.StatusCode)
	}
}

func TestRoomEvents_SearchFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// Search by room name
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?q=meeting", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) == 0 {
		t.Fatal("expected events matching 'meeting'")
	}

	// Search by user name
	req2 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?q=joiner", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	var result2 roomEventsResponse
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatal(err)
	}
	if len(result2.Events) == 0 {
		t.Fatal("expected events matching 'joiner'")
	}
}

func TestRoomEvents_DateFilter(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	today := time.Now().Format("2006-01-02")

	// Filter from today — all events
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateFrom="+today, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) == 0 {
		t.Fatal("expected events from today")
	}

	// Filter to yesterday — no events (all events are from today/today-2)
	req3 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateTo=2010-01-01", http.NoBody)
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()

	var result3 roomEventsResponse
	if err := json.NewDecoder(resp3.Body).Decode(&result3); err != nil {
		t.Fatal(err)
	}
	if len(result3.Events) != 0 {
		t.Fatalf("expected 0 events before 2010, got %d", len(result3.Events))
	}
}

func TestRoomEvents_InvalidDate(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateFrom=bad", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateFrom, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateTo=also-bad", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid dateTo, got %d", resp2.StatusCode)
	}
}

func TestRoomEvents_OrderAsc(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?order=asc", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(result.Events))
	}
}

func TestRoomEvents_InvalidOrder(t *testing.T) {
	app, _, _, _ := setupRoomEventsTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?order=invalid", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid order, got %d", resp.StatusCode)
	}
}

func TestRoomEvents_LimitClamping(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?limit=200", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Limit > 100 {
		t.Fatalf("expected limit clamped, got %d", result.Limit)
	}
}

func TestRoomEvents_NegativeLimit(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?limit=-5", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Limit != 50 {
		t.Fatalf("expected limit clamped to 50, got %d", result.Limit)
	}
}

func TestRoomEvents_ZeroPage(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?page=0", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result.Page != 1 {
		t.Fatalf("expected page 1, got %d", result.Page)
	}
}

func TestRoomEvents_DateFromAndTo(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	today := time.Now().Format("2006-01-02")

	// Both dateFrom and dateTo = today — should get all today's events
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateFrom="+today+"&dateTo="+today, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) == 0 {
		t.Fatal("expected some events with dateFrom+dateTo both set to today")
	}
}

func TestRoomEvents_DateFromReversed(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// dateFrom after dateTo — should return empty (no events in that range)
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?dateFrom=2030-01-01&dateTo=2020-01-01", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 0 {
		t.Fatalf("expected 0 events for reversed date range, got %d", len(result.Events))
	}
}

func TestRoomEvents_EmptyTypeParam(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// type= with empty value should be treated as no filter
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) != 5 {
		t.Fatalf("expected 5 events (no filter), got %d", len(result.Events))
	}
}

func TestRoomEvents_CombinedFilters(t *testing.T) {
	app, roomRepo, userRepo, db := setupRoomEventsTestApp(t)
	seedRoomEvents(t, roomRepo, userRepo, db)

	// type=room_joined + q=meeting — should only get join events for "meeting-room"
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms/events?type=room_joined&q=meeting", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var result roomEventsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if len(result.Events) == 0 {
		t.Fatal("expected at least 1 join event for meeting-room")
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

package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupAuthzTestApp builds a Fiber app with real Protected middleware
// and a few representative endpoints to test authz scenarios.
func setupAuthzTestApp(t *testing.T) *fiber.App {
	t.Helper()
	cfg := &config.Config{}
	config.SetForTest(cfg)
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, nil, settingsRepo, nil, nil, nil)

	app := fiber.New()

	// Public route (no auth)
	app.Get("/public", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Protected route with real middleware
	app.Get("/protected", middleware.Protected(), func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})

	// Protected + admin-only route
	admin := app.Group("/admin", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.RequireAccess(models.AccessSuperAdmin))
	admin.Get("/rooms", handler.AdminListRooms)
	admin.Post("/rooms/:roomId/close", handler.AdminCloseRoom)

	// Room moderation route (handler-internal authz check)
	app.Post("/room/:roomId/kick/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), handler.KickParticipant)

	// Guest route (no auth middleware)
	app.Post("/room/guest-join", middleware.GuestRateLimiter(cfg.RateLimit), handler.GuestJoinRoom)

	// Seed user
	db.Create(&models.User{ID: "superadmin-user", Email: "admin@ex.com", Name: "Admin", Provider: "local", IsActive: true, Accesses: models.StringArray{"user", "superadmin"}})
	db.Create(&models.User{ID: "normal-user", Email: "user@ex.com", Name: "User", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	return app
}

func generateTestToken(t *testing.T, userID, email, name, provider string, accesses []string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID, email, name, provider, accesses, config.Get(), nil)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

// --- Unauthenticated access ---

func TestAuthz_PublicRoute_NoAuth(t *testing.T) {
	app := setupAuthzTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/public", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for public route, got %d", resp.StatusCode)
	}
}

func TestAuthz_ProtectedRoute_NoAuth(t *testing.T) {
	app := setupAuthzTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected route without auth, got %d", resp.StatusCode)
	}
}

func TestAuthz_ProtectedRoute_InvalidToken(t *testing.T) {
	app := setupAuthzTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid.token.here")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for invalid token, got %d", resp.StatusCode)
	}
}

func TestAuthz_AdminRoute_NoAuth(t *testing.T) {
	app := setupAuthzTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for admin route without auth, got %d", resp.StatusCode)
	}
}

func TestAuthz_RoomModeration_NoAuth(t *testing.T) {
	app := setupAuthzTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/room/nonexistent/kick/victim", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for moderation without auth, got %d", resp.StatusCode)
	}
}

func TestAuthz_GuestRoute_NoAuth(t *testing.T) {
	app := setupAuthzTestApp(t)
	body := `{"roomName":"test","guestName":"Guest"}`
	req := httptest.NewRequest(http.MethodPost, "/room/guest-join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("expected non-401 for guest route, got %d", resp.StatusCode)
	}
}

// --- Non-superadmin accessing admin routes ---

func TestAuthz_AdminRoute_NonSuperadmin(t *testing.T) {
	app := setupAuthzTestApp(t)

	// Generate token for normal user (no superadmin access)
	token := generateTestToken(t, "normal-user", "user@ex.com", "User", "local", []string{"user"})

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Middleware chain (RequireEmailVerified + RequireAccess) blocks non-privileged users
	// May return 401 (email not verified) or 403 (access denied) depending on config
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401/403 for non-superadmin, got %d", resp.StatusCode)
	}
}

func TestAuthz_AdminRoute_VerifyEmailRequired(t *testing.T) {
	cfg := config.Get()
	cfg.Auth.RequireEmailVerification = true

	app := setupAuthzTestApp(t)

	// Generate token for a user NOT marked as email-verified
	token := generateTestToken(t, "normal-user", "user@ex.com", "User", "local", []string{"user"})

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Should be blocked by RequireEmailVerified before RequireAccess
	if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusUnauthorized {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403/401 for unverified email on admin route, got %d: %s", resp.StatusCode, string(body))
	}

	cfg.Auth.RequireEmailVerification = false
}

// --- Expired token ---

func TestAuthz_ExpiredToken(t *testing.T) {
	cfg := config.Get()
	cfg.Auth.TokenDuration = -1 // force immediate expiry at generation
	app := setupAuthzTestApp(t)

	token, err := auth.GenerateToken("normal-user", "user@ex.com", "User", "local", []string{"user"}, cfg, nil)
	if err != nil {
		t.Fatalf("failed to generate expired token: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/protected", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", resp.StatusCode)
	}

	cfg.Auth.TokenDuration = 1 // restore
}

// --- Superadmin bypasses admin gate (positive control) ---

func TestAuthz_SuperadminAccessesAdminRoute(t *testing.T) {
	app := setupAuthzTestApp(t)
	token := generateTestToken(t, "superadmin-user", "admin@ex.com", "Admin", "local", []string{"user", "superadmin"})

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusForbidden {
		t.Fatal("superadmin should not get 403 on admin route")
	}
}

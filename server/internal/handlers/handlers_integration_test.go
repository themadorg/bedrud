package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// setupIntegrationApp builds a full Fiber app mimicking the real route registration
// but with in-memory DB + mock LK. Uses real middleware chain (Protected, RequireEmailVerified, RequireAccess).
func setupIntegrationApp(t *testing.T) *fiber.App {
	t.Helper()
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                     "integration-test-secret",
			TokenDuration:                 1,
			RequireEmailVerification:      false,
			FrontendURL:                   "http://localhost:3000",
			PasskeyChallengeTTL:           5,
			VerificationEmailCooldownMins: 1,
		},
		Server: config.ServerConfig{
			Host: "localhost",
			Port: "8090",
		},
		RateLimit: config.RateLimitConfig{}, // disabled by default (nil pointers)
		LiveKit: config.LiveKitConfig{
			Host:      "http://localhost:9999",
			APIKey:    "test-key",
			APISecret: "test-secret-1234567890123456",
		},
		Chat: config.ChatConfig{
			MaxUploadBytesPerUser: 10 * 1024 * 1024,
		},
	}
	config.SetForTest(cfg)
	db := testutil.SetupTestDB(t)

	// Repos
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	webhookRepo := repository.NewWebhookRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	verifEventRepo := repository.NewVerificationEventRepository(db)

	// Services
	lkClient := testutil.NewMockRoomService()
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	challengeStore := auth.NewChallengeStore(5)
	emailCooldown := NewCooldownCache(1)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, nil, "", "")
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := services.NewRoomCleanupService(roomRepo, recordingRepo, lkClient, nil, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, uploadTracker)

	// Handlers
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, challengeStore, emailCooldown, verifEventRepo)
	recordingHandler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	roomHandler := NewRoomHandler(lkClient, &cfg.LiveKit, &cfg.Chat, roomRepo, userRepo, nil, settingsRepo, webhookRepo, uploadTracker, cleanupSvc)
	usersHandler := NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc, verifEventRepo)
	overviewHandler := NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &cfg.LiveKit, lkClient, db, time.Now(), "test")
	lkWebhookHandler := NewLiveKitWebhookHandler(&cfg.LiveKit, roomRepo, recordingRepo, webhookRepo, db)

	// App
	app := fiber.New()
	api := app.Group("/api")

	// Public routes
	api.Get("/health", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "healthy"}) })
	api.Get("/ready", func(c *fiber.Ctx) error { return c.JSON(fiber.Map{"status": "ready"}) })

	// Auth routes (some public, some protected)
	api.Post("/auth/register", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Register)
	api.Post("/auth/login", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Login)
	api.Post("/auth/refresh", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.RefreshToken)
	api.Post("/auth/logout", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.Logout)
	api.Get("/auth/me", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.GetMe)

	// Room routes (protected)
	api.Post("/room/create", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.APIRateLimiter(cfg.RateLimit), roomHandler.CreateRoom)
	api.Post("/room/guest-join", middleware.GuestRateLimiter(cfg.RateLimit), roomHandler.GuestJoinRoom)
	api.Post("/room/:roomId/kick/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.KickParticipant)

	// Recording routes (protected + recordings enabled gate)
	api.Post("/rooms/:id/recording/start", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.RecordingsEnabled(settingsRepo), middleware.APIRateLimiter(cfg.RateLimit), recordingHandler.StartRecording)
	api.Post("/rooms/:id/recording/stop", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.RecordingsEnabled(settingsRepo), recordingHandler.StopRecording)
	api.Get("/rooms/:id/recordings", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.RecordingsEnabled(settingsRepo), recordingHandler.ListRecordings)
	api.Get("/rooms/:id/recordings/:rid", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.RecordingsEnabled(settingsRepo), recordingHandler.GetRecording)

	// Admin routes
	adminGroup := api.Group("/admin",
		middleware.Protected(),
		middleware.RequireEmailVerified(cfg, userRepo),
		middleware.RequireAccess(models.AccessSuperAdmin),
	)
	adminGroup.Get("/users", usersHandler.ListUsers)
	adminGroup.Get("/overview", overviewHandler.GetOverview)
	adminGroup.Get("/rooms", roomHandler.AdminListRooms)

	// Webhook (no JWT auth — uses HMAC)
	api.Post("/livekit/webhook", lkWebhookHandler.Handle)

	// Seed a superadmin user for tests
	db.Create(&models.User{
		ID: "admin-user", Email: "admin@ex.com", Name: "Admin",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user", "superadmin"},
	})
	db.Create(&models.User{
		ID: "normal-user", Email: "user@ex.com", Name: "User",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	})

	return app
}

func integrationToken(t *testing.T, userID, email string, accesses []string) string {
	t.Helper()
	token, err := auth.GenerateToken(userID, email, "Test", "local", accesses, config.Get(), nil)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	return token
}

// --- Public routes ---

func TestIntegration_PublicHealthRoute(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// --- Auth routes (public) ---

func TestIntegration_AuthRegisterPublic(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Should not get 401 — register is public
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("register route should be public, not require auth")
	}
}

func TestIntegration_AuthLoginPublic(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("login route should be public, not require auth")
	}
}

// --- Protected routes require auth ---

func TestIntegration_ProtectedRouteNoAuth(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for protected route without auth, got %d", resp.StatusCode)
	}
}

func TestIntegration_RoomCreateNoAuth(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/room/create", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for room create without auth, got %d", resp.StatusCode)
	}
}

func TestIntegration_AdminRouteNoAuth(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for admin route without auth, got %d", resp.StatusCode)
	}
}

// --- Admin routes require superadmin ---

func TestIntegration_AdminRouteNonSuperadmin(t *testing.T) {
	app := setupIntegrationApp(t)
	token := integrationToken(t, "normal-user", "user@ex.com", []string{"user"})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-superadmin on admin, got %d", resp.StatusCode)
	}
}

func TestIntegration_AdminRouteSuperadminOK(t *testing.T) {
	app := setupIntegrationApp(t)
	token := integrationToken(t, "admin-user", "admin@ex.com", []string{"user", "superadmin"})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/users", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Fatalf("superadmin should access admin routes, got %d", resp.StatusCode)
	}
}

// --- Recording route with recordings disabled ---

func TestIntegration_RecordingsDisabledGate(t *testing.T) {
	app := setupIntegrationApp(t)
	token := integrationToken(t, "admin-user", "admin@ex.com", []string{"user", "superadmin"})

	t.Run("StartRecording disabled", func(t *testing.T) {
		body := `{"roomName":"test-room"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rooms/test-room/recording/start", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			body2, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 403 (recordings disabled), got %d: %s", resp.StatusCode, string(body2))
		}
	})

	t.Run("StopRecording disabled", func(t *testing.T) {
		body := `{"roomName":"test-room"}`
		req := httptest.NewRequest(http.MethodPost, "/api/rooms/test-room/recording/stop", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("ListRecordings disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/rooms/test-room/recordings", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("GetRecording disabled", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/rooms/test-room/recordings/some-id", http.NoBody)
		req.Header.Set("Authorization", "Bearer "+token)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})
}

// --- Guest route bypasses auth ---

func TestIntegration_GuestJoinNoAuth(t *testing.T) {
	app := setupIntegrationApp(t)
	body := `{"roomName":"test-room","guestName":"Guest"}`
	req := httptest.NewRequest(http.MethodPost, "/api/room/guest-join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Should not get 401 — guest route is public
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("guest-join should be public, not require auth")
	}
}

// --- LiveKit webhook route bypasses JWT auth ---

func TestIntegration_LiveKitWebhookNoAuth(t *testing.T) {
	app := setupIntegrationApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/livekit/webhook", http.NoBody)
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Returns 401 from its own HMAC validation (not from Protected middleware).
	// The key test is that it does NOT get 500 from unprotected handler panic.
	if resp.StatusCode == http.StatusInternalServerError {
		t.Fatal("livekit webhook should handle requests without JWT middleware")
	}
}

// --- Valid auth + full chain works ---

func TestIntegration_ValidAuth_ProtectedRouteOK(t *testing.T) {
	app := setupIntegrationApp(t)
	token := integrationToken(t, "normal-user", "user@ex.com", []string{"user"})

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// With valid token but user not found in DB... depends on handler
	// Just check it processes the request (not 401)
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatal("valid token should not get 401")
	}
}

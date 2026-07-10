package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupInviteOnlyRegisterApp(t *testing.T) (*fiber.App, *repository.UserRepository, *repository.InviteTokenRepository, string) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteRepo := repository.NewInviteTokenRepository(db)
	authSvc := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "invite-audit-test-secret-key-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	s, _ := settingsRepo.GetSettings()
	s.RegistrationEnabled = true
	s.TokenRegistrationOnly = true
	if err := settingsRepo.SaveSettings(s); err != nil {
		t.Fatal(err)
	}
	raw := "invite-" + uuid.New().String()[:8]
	tok := &models.InviteToken{
		ID: uuid.New().String(), Token: raw, CreatedBy: "admin",
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}
	if err := inviteRepo.Create(tok); err != nil {
		t.Fatal(err)
	}
	h := NewAuthHandler(authSvc, cfg, settingsRepo, inviteRepo, nil, NewCooldownCache(0), nil)
	app := fiber.New()
	app.Post("/api/auth/register", h.Register)
	return app, userRepo, inviteRepo, raw
}

func TestRegister_InviteOnly_MissingToken(t *testing.T) {
	app, _, _, _ := setupInviteOnlyRegisterApp(t)
	body, _ := json.Marshal(map[string]string{
		"email": "a@ex.com", "password": "securepass123", "name": "Ab",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestRegister_InviteOnly_InvalidToken(t *testing.T) {
	app, _, _, _ := setupInviteOnlyRegisterApp(t)
	body, _ := json.Marshal(map[string]string{
		"email": "b@ex.com", "password": "securepass123", "name": "Bc", "inviteToken": "bad-token",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestRegister_InviteOnly_UsedTokenRejected(t *testing.T) {
	app, _, inviteRepo, raw := setupInviteOnlyRegisterApp(t)
	tok, _ := inviteRepo.GetByToken(raw)
	if tok == nil {
		t.Fatal("token missing")
	}
	if err := inviteRepo.MarkUsed(tok.ID, "u1"); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{
		"email": "used@ex.com", "password": "securepass123", "name": "Us", "inviteToken": raw,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403 for used invite, got %d", resp.StatusCode)
	}
}

func TestRegister_InviteOnly_Success(t *testing.T) {
	app, userRepo, _, raw := setupInviteOnlyRegisterApp(t)
	body, _ := json.Marshal(map[string]string{
		"email": "ok@ex.com", "password": "securepass123", "name": "OK", "inviteToken": raw,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
	u, _ := userRepo.GetUserByEmail("ok@ex.com")
	if u == nil {
		t.Fatal("user not created")
	}
}

func TestRegister_InviteOnly_ConcurrentSingleUse(t *testing.T) {
	app, userRepo, inviteRepo, raw := setupInviteOnlyRegisterApp(t)
	var okCount int32
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			body, _ := json.Marshal(map[string]string{
				"email":       fmt.Sprintf("race%d@ex.com", n),
				"password":    "securepass123",
				"name":        "Race",
				"inviteToken": raw,
			})
			req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				atomic.AddInt32(&okCount, 1)
			}
		}(i)
	}
	wg.Wait()
	if okCount != 1 {
		t.Fatalf("expected exactly one successful registration, got %d", okCount)
	}
	tok, _ := inviteRepo.GetByToken(raw)
	if tok == nil || tok.UsedAt == nil {
		t.Fatal("invite should be marked used")
	}
	_ = userRepo
}

func TestPasskeySignupBegin_InviteRequired_Missing(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteRepo := repository.NewInviteTokenRepository(db)
	authSvc := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth:   config.AuthConfig{JWTSecret: "pk-invite-test-secret-key-32b", TokenDuration: 1, SessionSecret: "s"},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	auth.InitializeSessionStore(cfg.Auth.SessionSecret, false)
	s, _ := settingsRepo.GetSettings()
	s.RegistrationEnabled = true
	s.TokenRegistrationOnly = true
	_ = settingsRepo.SaveSettings(s)
	h := NewAuthHandler(authSvc, cfg, settingsRepo, inviteRepo, auth.NewChallengeStore(5), NewCooldownCache(0), nil)
	app := fiber.New()
	app.Post("/api/auth/passkey/signup/begin", h.PasskeySignupBegin)
	body, _ := json.Marshal(map[string]string{"email": "pk@ex.com", "name": "PK"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("want 403, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_Login_InactiveUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	authSvc := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{JWTSecret: "inactive-login-test-secret-32b", TokenDuration: 1},
	}
	config.SetForTest(cfg)
	user, _ := authSvc.Register("inactive@ex.com", "securepass123", "Inactive")
	user.IsActive = false
	if err := userRepo.UpdateUser(user); err != nil {
		t.Fatal(err)
	}
	_, err := authSvc.Login("inactive@ex.com", "securepass123")
	if err == nil || err.Error() != "account is deactivated" {
		t.Fatalf("want deactivated error, got %v", err)
	}
}

func TestResetPassword_SequentialReuseRejected(t *testing.T) {
	app, authSvc, cfg, _ := setupPasswordResetTestAppWithDB(t)
	_, _ = authSvc.Register("reuse@example.com", "oldpass123", "Reuse")
	user, _ := authSvc.GetUserByEmail("reuse@example.com")
	token, err := auth.GenerateResetToken(user.ID, user.Email, nil, cfg)
	if err != nil {
		t.Fatal(err)
	}
	resetBody := func() *http.Request {
		body, _ := json.Marshal(map[string]string{"token": token, "newPassword": "newSecurePass456!"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		return req
	}
	resp1, _ := app.Test(resetBody(), -1)
	defer resp1.Body.Close()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first reset want 200, got %d", resp1.StatusCode)
	}
	resp2, _ := app.Test(resetBody(), -1)
	defer resp2.Body.Close()
	if resp2.StatusCode == http.StatusOK {
		t.Fatal("second reset with same token should fail")
	}
}

func TestReady_DatabaseUnavailable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	database.SetForTest(db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.Close()
	app := fiber.New()
	app.Get("/api/ready", func(c *fiber.Ctx) error {
		if sqlDB2, err := database.GetDB().DB(); err != nil || sqlDB2.Ping() != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not_ready", "error": "database unavailable"})
		}
		return c.JSON(fiber.Map{"status": "ready"})
	})
	req := httptest.NewRequest(http.MethodGet, "/api/ready", http.NoBody)
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("want 503, got %d", resp.StatusCode)
	}
	var body map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body["status"] != "not_ready" {
		t.Fatalf("want not_ready, got %v", body)
	}
}

func TestAPIRateLimiter_Exhaustion(t *testing.T) {
	max := 2
	cfg := config.RateLimitConfig{APIMaxRequests: &max, APIWindowSecs: auditPtrInt(60)}
	app := fiber.New()
	app.Post("/x", middleware.APIRateLimiter(cfg), func(c *fiber.Ctx) error { return c.SendString("ok") })
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodPost, "/x", http.NoBody)
		resp, _ := app.Test(req, -1)
		if i < 2 && resp.StatusCode != http.StatusOK {
			t.Fatalf("req %d want 200, got %d", i, resp.StatusCode)
		}
		if i == 2 && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("req 2 want 429, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestResendRateLimiter_Exhaustion(t *testing.T) {
	max := 1
	cfg := config.RateLimitConfig{AuthResendMaxRequests: &max, AuthResendWindowSecs: auditPtrInt(60)}
	app := fiber.New()
	app.Post("/resend", middleware.ResendRateLimiter(cfg), func(c *fiber.Ctx) error { return c.SendString("ok") })
	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/resend", http.NoBody)
		resp, _ := app.Test(req, -1)
		if i == 0 && resp.StatusCode != http.StatusOK {
			t.Fatalf("want 200, got %d", resp.StatusCode)
		}
		if i == 1 && resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("want 429, got %d", resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func auditPtrInt(n int) *int { return &n }
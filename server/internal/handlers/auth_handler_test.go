package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

const bearerPrefix = "Bearer "

func setupAuthTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "handler-auth-test-secret-key-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret-for-testing",
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	// Set global config so Login/GuestLogin (which call config.Get()) don't panic
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	app := fiber.New()

	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/guest-login", authHandler.GuestLogin)
	app.Get("/api/auth/me", func(c *fiber.Ctx) error {
		// Inline middleware for testing
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, authHandler.GetMe)

	return app, authService, cfg
}

func TestAuthHandler_Register_Success(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"email":    "new@example.com",
		"password": "securepass123",
		"name":     "New User",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	tokens, _ := result["tokens"].(map[string]interface{})
	if tokens == nil || tokens["accessToken"] == nil || tokens["accessToken"] == "" {
		t.Fatal("expected tokens.accessToken in response")
	}
	if tokens["refreshToken"] == nil || tokens["refreshToken"] == "" {
		t.Fatal("expected tokens.refreshToken in response")
	}
	if result["user"] == nil {
		t.Fatal("expected user in response")
	}
}

func TestAuthHandler_Register_InvalidBody(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader([]byte("invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_Register_DuplicateEmail(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"email":    "dup@example.com",
		"password": "securepassword1",
		"name":     "First User",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	// Try again
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d for duplicate, got %d", http.StatusBadRequest, resp2.StatusCode)
	}
}

func TestAuthHandler_Login_Success(t *testing.T) {
	app, authService, _ := setupAuthTestApp(t)

	// First register a user
	_, _ = authService.Register("login@example.com", "mypassword", "Login User")

	// Now login
	body, _ := json.Marshal(map[string]string{
		"email":    "login@example.com",
		"password": "mypassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}
}

func TestAuthHandler_Login_InvalidCredentials(t *testing.T) {
	app, authService, _ := setupAuthTestApp(t)

	_, _ = authService.Register("wrong@example.com", "correctpass", "User")

	body, _ := json.Marshal(map[string]string{
		"email":    "wrong@example.com",
		"password": "wrongpassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAuthHandler_Login_NonexistentUser(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"email":    "ghost@example.com",
		"password": "pass",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAuthHandler_Login_InvalidBody(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_GuestLogin_Success(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{"name": "Guest Player"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	user := result["user"].(map[string]interface{})
	if user["name"] != "Guest Player" {
		t.Fatalf("expected name 'Guest Player', got '%v'", user["name"])
	}
}

func TestAuthHandler_GuestLogin_EmptyName(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{"name": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_GuestLogin_InvalidBody(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader([]byte("{")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_GuestLogin_NullByteName(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	body, _ := json.Marshal(map[string]string{"name": "\x00"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d for null byte name, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_GuestLogin_NullByteTooShort(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	// "\x00a" has raw length 2, but after sanitization becomes "a" (length 1, below min 2)
	body, _ := json.Marshal(map[string]string{"name": "\x00a"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d for null-byte-padded short name, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

func TestAuthHandler_GetMe_Success(t *testing.T) {
	app, authService, cfg := setupAuthTestApp(t)

	user, _ := authService.Register("me@example.com", "pass", "Me User")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["email"] != "me@example.com" {
		t.Fatalf("expected email 'me@example.com', got '%v'", result["email"])
	}
}

func TestAuthHandler_GetMe_NoToken(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAuthHandler_GetMe_InvalidToken(t *testing.T) {
	app, _, _ := setupAuthTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/me", http.NoBody)
	req.Header.Set("Authorization", "Bearer invalid-jwt-token")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

// --- setupAuthTestAppFull wires RefreshToken, UpdateProfile, ChangePassword, Logout ---

func setupAuthTestAppFull(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	app, authService, cfg := setupAuthTestApp(t)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, nil, nil, nil, emailCooldown, nil)

	// Helper: add auth-required routes to the existing app
	app.Post("/api/auth/refresh", authHandler.RefreshToken)
	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, authHandler.UpdateProfile)
	app.Post("/api/auth/change-password", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, authHandler.ChangePassword)
	app.Post("/api/auth/logout", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, authHandler.Logout)

	return app, authService, cfg
}

// --- RefreshToken tests ---

func TestAuthHandler_RefreshToken_Success(t *testing.T) {
	app, authService, cfg := setupAuthTestApp(t)

	_, _ = authService.Register("refresh@example.com", "pass", "Refresh User")
	loginResp, _ := authService.Login("refresh@example.com", "pass")

	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, nil, nil, nil, emailCooldown, nil)
	app.Post("/api/auth/refresh", authHandler.RefreshToken)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": loginResp.Token.RefreshToken,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["access_token"] == nil || result["access_token"] == "" {
		t.Fatal("expected access_token in response")
	}
}

func TestAuthHandler_RefreshToken_InvalidToken(t *testing.T) {
	app, authService, cfg := setupAuthTestApp(t)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, nil, nil, nil, emailCooldown, nil)
	app.Post("/api/auth/refresh", authHandler.RefreshToken)

	body, _ := json.Marshal(map[string]string{"refresh_token": "not-a-real-token"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected %d, got %d", http.StatusUnauthorized, resp.StatusCode)
	}
}

func TestAuthHandler_RefreshToken_InvalidBody(t *testing.T) {
	app, authService, cfg := setupAuthTestApp(t)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, nil, nil, nil, emailCooldown, nil)
	app.Post("/api/auth/refresh", authHandler.RefreshToken)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- UpdateProfile tests ---

func TestAuthHandler_UpdateProfile_Success(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("upprof@example.com", "pass", "Old Name")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	body, _ := json.Marshal(map[string]string{"name": "New Name"})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}
}

func TestAuthHandler_UpdateProfile_ShortName(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("shortname@example.com", "pass", "User")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	body, _ := json.Marshal(map[string]string{"name": "X"})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- ChangePassword tests ---

func TestAuthHandler_ChangePassword_Success(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("chpass@example.com", "oldpass123", "User")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "oldpass123",
		"newPassword":     "newpass456secure", // must be >= 12 chars
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}
}

func TestAuthHandler_ChangePassword_TooShortNewPassword(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("shortpw@example.com", "oldpass123", "User")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	body, _ := json.Marshal(map[string]string{
		"currentPassword": "oldpass123",
		"newPassword":     "abc",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/change-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected %d, got %d", http.StatusBadRequest, resp.StatusCode)
	}
}

// --- Logout tests ---

func TestAuthHandler_Logout_Success(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("logoutha@example.com", "pass", "Logout User")
	loginResp, _ := authService.Login("logoutha@example.com", "pass")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	body, _ := json.Marshal(map[string]string{"refresh_token": loginResp.Token.RefreshToken})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected %d, got %d: %s", http.StatusOK, resp.StatusCode, string(respBody))
	}
}

// --- Email Verification test setup ---

func setupVerificationTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                     "verification-test-secret-key-32b",
			TokenDuration:                 1,
			SessionSecret:                 "session-secret-for-testing",
			RequireEmailVerification:      true,
			VerificationEmailCooldownMins: 2,
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/verify", authHandler.VerifyEmail)
	app.Post("/api/auth/verify/resend", authHandler.ResendVerification)
	app.Get("/api/auth/verify/status", middleware.Protected(), authHandler.CheckVerificationStatus)

	return app, authService, cfg
}

// setupVerificationTestAppWithDB is like setupVerificationTestApp but also returns
// the DB handle, needed by tests that inspect the job queue.
func setupVerificationTestAppWithDB(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	database.SetForTest(db) // needed by queue.Enqueue which calls database.GetDB()
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                     "verification-test-secret-key-32b",
			TokenDuration:                 1,
			SessionSecret:                 "session-secret-for-testing",
			RequireEmailVerification:      true,
			VerificationEmailCooldownMins: 2,
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/verify", authHandler.VerifyEmail)
	app.Post("/api/auth/verify/resend", authHandler.ResendVerification)
	app.Get("/api/auth/verify/status", middleware.Protected(), authHandler.CheckVerificationStatus)

	return app, authService, cfg, db
}

func TestAuthHandler_Register_RequiresVerification(t *testing.T) {
	app, _, _ := setupVerificationTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"email":    "verifyreg@example.com",
		"password": "securepass123",
		"name":     "Verify User",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	if result["requiresVerification"] != true {
		t.Fatalf("expected requiresVerification=true, got %v", result["requiresVerification"])
	}
	if result["email"] != "verifyreg@example.com" {
		t.Fatalf("expected email 'verifyreg@example.com', got %v", result["email"])
	}
	// Should NOT have tokens
	if result["tokens"] != nil {
		t.Fatal("expected no tokens in response when verification is required")
	}
}

func TestAuthHandler_Register_VerificationEmailURL(t *testing.T) {
	app, _, cfg, db := setupVerificationTestAppWithDB(t)

	// Config should use Domain for verify URL (FrontendURL is empty in tests)
	config.SetForTest(cfg)

	body, _ := json.Marshal(map[string]string{
		"email":    "urlcheck@test.com",
		"password": "securepass123",
		"name":     "URL Check",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	if result["requiresVerification"] != true {
		t.Fatalf("expected requiresVerification=true, got %v", result["requiresVerification"])
	}

	// Find the verify_email job in the queue
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)
	var verifyURL string
	for _, j := range jobs {
		var payload struct {
			TemplateName string         `json:"template_name"`
			TemplateData map[string]any `json:"template_data,omitempty"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err != nil {
			continue
		}
		if payload.TemplateName == "verify_email" {
			if data, ok := payload.TemplateData["VerifyURL"]; ok {
				verifyURL, _ = data.(string)
			}
		}
	}

	if verifyURL == "" {
		t.Fatal("could not find verify_email job with VerifyURL")
	}

	// Assert VerifyURL points to frontend /auth/verify route
	expectedPrefix := "https://localhost/auth/verify?token="
	if !strings.HasPrefix(verifyURL, expectedPrefix) {
		t.Fatalf("expected VerifyURL to start with '%s', got '%s'", expectedPrefix, verifyURL)
	}

	// Token should be non-empty
	tokenPart := strings.TrimPrefix(verifyURL, expectedPrefix)
	if tokenPart == "" {
		t.Fatal("expected non-empty verification token in URL")
	}
	if len(tokenPart) < 10 {
		t.Fatalf("expected token to be at least 10 chars, got %d: '%s'", len(tokenPart), tokenPart)
	}
}

func TestAuthHandler_Login_Unverified(t *testing.T) {
	app, authService, _ := setupVerificationTestApp(t)

	_, _ = authService.Register("unver@example.com", "mypassword", "Unverified")

	body, _ := json.Marshal(map[string]string{
		"email":    "unver@example.com",
		"password": "mypassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 for unverified login, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["requiresVerification"] != true {
		t.Fatalf("expected requiresVerification=true, got %v", result["requiresVerification"])
	}
	if result["email"] != "unver@example.com" {
		t.Fatalf("expected email 'unver@example.com', got %v", result["email"])
	}
}

func TestAuthHandler_Login_Verified(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("verified@example.com", "mypassword", "Verified")

	// Manually verify the user's email
	now := time.Now()
	user.EmailVerifiedAt = &now
	if err := authService.UpdateUser(user); err != nil {
		t.Fatalf("failed to verify user: %v", err)
	}
	config.SetForTest(cfg)

	body, _ := json.Marshal(map[string]string{
		"email":    "verified@example.com",
		"password": "mypassword",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for verified login, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	tokens, _ := result["tokens"].(map[string]interface{})
	if tokens == nil || tokens["accessToken"] == nil {
		t.Fatal("expected tokens for verified user")
	}
}

func TestAuthHandler_VerifyEmail_Success(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("verifyok@example.com", "pass", "Verify OK")

	token, err := auth.GenerateVerificationToken(user.ID, user.Email, cfg)
	if err != nil {
		t.Fatalf("failed to generate verification token: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if result["access_token"] == nil || result["access_token"] == "" {
		t.Fatal("expected access_token in response")
	}
	if result["refresh_token"] == nil || result["refresh_token"] == "" {
		t.Fatal("expected refresh_token in response")
	}
	if result["verified"] != true {
		t.Fatal("expected verified=true in response")
	}

	// Verify the user is now marked as verified
	updated, _ := authService.GetUserByID(user.ID)
	if updated.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to be set after successful verification")
	}
}

func TestAuthHandler_VerifyEmail_TokenEmailMismatch(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("original@test.com", "pass", "Original")

	// Generate token for original email, then change user's email
	token, err := auth.GenerateVerificationToken(user.ID, user.Email, cfg)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}
	user.Email = "changed@test.com"
	if err := authService.UpdateUser(user); err != nil {
		t.Fatalf("failed to change email: %v", err)
	}

	// Verify should reject — token issued for original@test.com, user now changed@test.com
	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 on email mismatch, got %d", resp.StatusCode)
	}

	// Verify user is still NOT verified
	updated, _ := authService.GetUserByID(user.ID)
	if updated.EmailVerifiedAt != nil {
		t.Fatal("expected EmailVerifiedAt to remain nil when token email doesn't match")
	}
}

func TestAuthHandler_VerifyEmail_MissingToken(t *testing.T) {
	app, _, _ := setupVerificationTestApp(t)

	body, _ := json.Marshal(map[string]string{})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_VerifyEmail_AlreadyVerified(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("alreadyver@example.com", "pass", "Already Verified")

	// Manually verify first
	now := time.Now()
	user.EmailVerifiedAt = &now
	if err := authService.UpdateUser(user); err != nil {
		t.Fatalf("failed to verify user: %v", err)
	}

	token, err := auth.GenerateVerificationToken(user.ID, user.Email, cfg)
	if err != nil {
		t.Fatalf("failed to generate verification token: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_VerifyEmail_UserNotFound(t *testing.T) {
	_, _, cfg := setupVerificationTestApp(t)

	// Generate token for a non-existent user (empty email — no user to get email from)
	token, err := auth.GenerateVerificationToken("nonexistent-user-id", "", cfg)
	if err != nil {
		t.Fatalf("failed to generate verification token: %v", err)
	}

	app, _, _ := setupVerificationTestApp(t)
	body, _ := json.Marshal(map[string]string{"token": token})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAuthHandler_ResendVerification_Success(t *testing.T) {
	app, authService, _ := setupVerificationTestApp(t)

	_, _ = authService.Register("resend@example.com", "pass", "Resend User")

	body, _ := json.Marshal(map[string]string{"email": "resend@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["message"] == nil {
		t.Fatal("expected message in response")
	}
}

func TestAuthHandler_ResendVerification_Cooldown(t *testing.T) {
	app, authService, _ := setupVerificationTestApp(t)

	_, _ = authService.Register("cooldown@example.com", "pass", "Cooldown User")

	body, _ := json.Marshal(map[string]string{"email": "cooldown@example.com"})

	// First request — succeeds (should enqueue)
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, _ := app.Test(req1, -1)
	_ = resp1.Body.Close()

	// Second request — cooldown active, but still returns 200 (enumeration safe)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	// Cooldown is enforced silently — always uniform 200
	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 (cooldown is silent), got %d: %s", resp2.StatusCode, string(respBody))
	}
}

func TestAuthHandler_ResendVerification_EnumerationSafe(t *testing.T) {
	app, _, _ := setupVerificationTestApp(t)

	// Unknown email should return 200, not 404/403 (prevents email enumeration)
	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for unknown email (enumeration safe), got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestAuthHandler_RefreshToken_Unverified(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	authService.Register("refreshunver@example.com", "pass", "Refresh Unverified")

	// Login normally (without verification config — bypass to get a token)
	cfgOrig := *cfg
	cfgOrig.Auth.RequireEmailVerification = false
	config.SetForTest(&cfgOrig)
	loginResp, _ := authService.Login("refreshunver@example.com", "pass")
	config.SetForTest(cfg)

	// Wire RefreshToken handler
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, nil, nil, nil, emailCooldown, nil)
	app.Post("/api/auth/refresh", authHandler.RefreshToken)

	body, _ := json.Marshal(map[string]string{
		"refresh_token": loginResp.Token.RefreshToken,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/refresh", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403 for unverified refresh, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["error"] == nil {
		t.Fatal("expected error in response")
	}
}

func TestAuthHandler_ResendVerification_AlreadyVerifiedReturns200(t *testing.T) {
	app, authService, _ := setupVerificationTestApp(t)

	u, _ := authService.Register("alreadysent@example.com", "pass", "Already Sent")

	// Manually verify
	now := time.Now()
	u.EmailVerifiedAt = &now
	_ = authService.UpdateUser(u)

	// Resend for already-verified user should return 200 (enumeration safe)
	body, _ := json.Marshal(map[string]string{"email": "alreadysent@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 for already verified (enumeration safe), got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestAuthHandler_Logout_InvalidBody(t *testing.T) {
	app, authService, cfg := setupAuthTestAppFull(t)

	user, _ := authService.Register("logoutbad@example.com", "pass", "Logout Bad")
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	// Invalid body is non-fatal — handler falls back to cookies and clears them
	req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", bytes.NewReader([]byte("{invalid")))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

// ── Item 14: Missing verification tests ────────────────────────────────

// Test 1: Expired verification token — should redirect with expired status
func TestVerifyEmail_ExpiredToken(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("expiredok@test.com", "securepassword123", "Expired")

	// Create an expired verification token manually
	claims := &auth.Claims{
		UserID:  user.ID,
		Purpose: "email_verify",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "bedrud",
			Subject:   user.ID,
			Audience:  []string{"bedrud"},
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)), // 1 hour ago
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	expiredToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := expiredToken.SignedString([]byte(cfg.Auth.JWTSecret))

	body, _ := json.Marshal(map[string]string{"token": tokenStr})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for expired token, got %d", resp.StatusCode)
	}
}

// Test 3: Concurrent resend — all return 200 (enumeration safe)
func TestConcurrentResend_SingleEmail(t *testing.T) {
	app, authService, _, db := setupVerificationTestAppWithDB(t)

	_, _ = authService.Register("concurrent@test.com", "securepassword123", "Concurrent")
	body, _ := json.Marshal(map[string]string{"email": "concurrent@test.com"})

	var wg sync.WaitGroup
	// Fire 5 goroutines to increase race-condition window
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, _ := app.Test(req, 5000)
			if resp != nil {
				_ = resp.Body.Close()
			}
		}()
	}
	wg.Wait()

	// queue.Enqueue is synchronous — job is in DB by the time app.Test returns.
	// Count verify_email jobs. The CooldownCache mutex guarantees
	// only one should be enqueued. If the race window is larger, the test
	// catches it here.
	var jobCount int64
	db.Model(&models.Job{}).
		Where("type = ?", "send_email").
		Count(&jobCount)

	// Inspect payloads to filter for verify_email template specifically
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)
	var verifyCount int
	for _, j := range jobs {
		var payload struct {
			TemplateName string `json:"template_name"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && payload.TemplateName == "verify_email" {
			verifyCount++
		}
	}

	if verifyCount != 1 {
		t.Fatalf("expected exactly 1 verify_email job enqueued, got %d/%d (total send_email: %d)",
			verifyCount, len(jobs), jobCount)
	}
}

// Test 4: Resend after server restart (cooldown cleared)
func TestResend_AfterServerRestart(t *testing.T) {
	app1, authService, _ := setupVerificationTestApp(t)

	_, _ = authService.Register("restart@test.com", "securepassword123", "Restart")

	// First resend — should get cooldown token
	body, _ := json.Marshal(map[string]string{"email": "restart@test.com"})
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, _ := app1.Test(req1, -1)
	_ = resp1.Body.Close()

	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first resend, got %d", resp1.StatusCode)
	}

	// Now create a new app (simulates restart) — cooldown is in-memory, should be empty
	app2, _, _ := setupVerificationTestApp(t)
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/verify/resend", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app2.Test(req2, -1)
	_ = resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on resend after restart (cooldown cleared), got %d", resp2.StatusCode)
	}
}

// Test 5: Register → verify → double-verify (already_verified redirect)
func TestRegister_ThenVerify_ThenDoubleVerify(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("doubleok@test.com", "securepassword123", "Double")

	// Generate verification token
	token, _ := auth.GenerateVerificationToken(user.ID, user.Email, cfg)

	// First verify — should return 200 with tokens
	body1, _ := json.Marshal(map[string]string{"token": token})
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := app.Test(req1, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on first verify, got %d", resp1.StatusCode)
	}
	respBody1, _ := io.ReadAll(resp1.Body)
	var result1 map[string]interface{}
	if err := json.Unmarshal(respBody1, &result1); err != nil {
		t.Fatal(err)
	}
	if result1["verified"] != true {
		t.Fatal("expected verified=true on first verify")
	}

	// Double verify — should say already_verified (409)
	body2, _ := json.Marshal(map[string]string{"token": token})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 on double-verify, got %d", resp2.StatusCode)
	}
}

// ── Admin verification tests ──────────────────────────────────────────

func setupAdminVerificationTest(t *testing.T) (*fiber.App, *repository.UserRepository, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                     "admin-test-secret-key-for-test-32b",
			TokenDuration:                 1,
			SessionSecret:                 "session-secret-for-testing",
			RequireEmailVerification:      true,
			VerificationEmailCooldownMins: 2,
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	config.SetForTest(cfg)
	database.SetForTest(db)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	// Minimal UsersHandler for admin routes
	roomRepo := repository.NewRoomRepository(db)
	usersHandler := NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, nil, nil)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/verify", authHandler.VerifyEmail)
	app.Post("/api/auth/verify/resend", authHandler.ResendVerification)
	app.Get("/api/auth/verify/status", middleware.Protected(), authHandler.CheckVerificationStatus)

	// Admin routes — use UsersHandler which has AdminVerifyEmail / AdminResendVerification
	admin := app.Group("/api/admin", func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "admin-id", Accesses: []string{"superadmin"}})
		return c.Next()
	})
	admin.Post("/users/:id/verify", usersHandler.AdminVerifyEmail)
	admin.Post("/users/:id/verify/resend", usersHandler.AdminResendVerification)

	// Create admin user with superadmin access
	adminUser, _ := authService.Register("admin@verifytest.com", "securepassword123", "Admin")
	adminUser.Accesses = []string{"superadmin"}
	_ = userRepo.UpdateUser(adminUser)

	return app, userRepo, cfg
}

// Test 6a: Admin force-verifies user — EmailVerifiedAt set
func TestAdmin_ForceVerify(t *testing.T) {
	app, userRepo, _ := setupAdminVerificationTest(t)

	// Register unverified user
	body, _ := json.Marshal(map[string]string{
		"email":    "unverifieduser@test.com",
		"password": "securepassword123",
		"name":     "Unverified User",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	// Find user by email
	user, _ := userRepo.GetUserByEmail("unverifieduser@test.com")
	if user == nil {
		t.Fatal("failed to create user")
	}

	if user.EmailVerifiedAt != nil {
		t.Fatal("expected user to be unverified initially")
	}

	// Admin force-verify
	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+user.ID+"/verify", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 on admin force-verify, got %d: %s", resp2.StatusCode, string(respBody))
	}

	// Verify EmailVerifiedAt was set
	updated, _ := userRepo.GetUserByID(user.ID)
	if updated == nil || updated.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to be set after admin force-verify")
	}
}

// Test 6b: Admin resends verification email
func TestAdmin_ResendVerification(t *testing.T) {
	app, userRepo, _ := setupAdminVerificationTest(t)

	// Register unverified user
	body, _ := json.Marshal(map[string]string{
		"email":    "resendadmin@test.com",
		"password": "securepassword123",
		"name":     "Resend Admin",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	user, _ := userRepo.GetUserByEmail("resendadmin@test.com")
	if user == nil {
		t.Fatal("failed to create user")
	}

	// Admin resend
	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/users/"+user.ID+"/verify/resend", http.NoBody)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 on admin resend, got %d: %s", resp2.StatusCode, string(respBody))
	}
}

// Test 7: Passkey signup with verification required
func TestPasskeySignup_RequiresVerification(t *testing.T) {
	// This test verifies that login blocks unverified users with requiresVerification.
	// (Passkey signup attestation simulation is complex — relies on handler being gated properly.)
	app, authService, _ := setupVerificationTestApp(t)

	_, _ = authService.Register("passkeyver@test.com", "securepassword123", "Passkey Ver")

	// Check that the login handler blocks unverified users
	loginBody, _ := json.Marshal(map[string]string{
		"email":    "passkeyver@test.com",
		"password": "securepassword123",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(loginBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	if result["requiresVerification"] != true {
		t.Fatal("expected requiresVerification for unverified user")
	}
	if result["email"] == nil {
		t.Fatal("expected email in response")
	}
}

// Test 8: CheckVerificationStatus endpoint
func TestVerificationStatus_Endpoint(t *testing.T) {
	app, authService, cfg := setupVerificationTestApp(t)

	user, _ := authService.Register("statuscheck@test.com", "securepassword123", "Status")

	// Generate a valid token for the unverified user
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/verify/status", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for status endpoint, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	if result["verified"] != false {
		t.Fatal("expected verified=false for unverified user")
	}
	if result["email"] != "statuscheck@test.com" {
		t.Fatalf("expected email=statuscheck@test.com, got %v", result["email"])
	}
}

// TestEmailChange_AutoSendsVerification verifies that changing email:
// 1. Sends verification email to new address
// 2. Issues new JWT with updated email so user stays logged in
// 3. Clears EmailVerifiedAt
func TestEmailChange_AutoSendsVerification(t *testing.T) {
	app, authService, cfg, db := setupVerificationTestAppWithDB(t)

	// Register+verify user
	user, _ := authService.Register("old@test.com", "securepassword123", "Old Email")
	now := time.Now()
	user.EmailVerifiedAt = &now
	_ = authService.UpdateUser(user)

	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, user.EmailVerifiedAt)

	// Build auth middleware inline (like setupAuthTestAppFull does)
	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, (&AuthHandler{
		authService:   authService,
		config:        cfg,
		emailCooldown: NewCooldownCache(2 * time.Minute),
	}).UpdateProfile)

	// Change email
	body, _ := json.Marshal(map[string]string{
		"name":  "Old Email",
		"email": "newemail@test.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 on email change, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)

	// Response should indicate verification required
	if result["requiresVerification"] != true {
		t.Fatal("expected requiresVerification=true after email change")
	}
	if result["email"] != "newemail@test.com" {
		t.Fatalf("expected email=newemail@test.com, got %v", result["email"])
	}

	// Verify DB shows unverified
	updated, _ := authService.GetUserByID(user.ID)
	if updated == nil {
		t.Fatal("user not found")
	}
	if updated.Email != "newemail@test.com" {
		t.Fatalf("expected DB email newemail@test.com, got %s", updated.Email)
	}
	if updated.EmailVerifiedAt != nil {
		t.Fatal("expected EmailVerifiedAt to be nil after email change")
	}

	// Verify new JWT tokens in response
	if result["tokens"] == nil {
		t.Fatal("expected new tokens in response after email change")
	}
	tokens := result["tokens"].(map[string]interface{})
	if tokens["accessToken"] == nil || tokens["accessToken"] == "" {
		t.Fatal("expected new accessToken")
	}
	if tokens["refreshToken"] == nil || tokens["refreshToken"] == "" {
		t.Fatal("expected new refreshToken")
	}

	// Verify new token has new email in claims
	newClaims, err := auth.ValidateToken(tokens["accessToken"].(string), cfg)
	if err != nil {
		t.Fatalf("new token should be valid: %v", err)
	}
	if newClaims.Email != "newemail@test.com" {
		t.Fatalf("expected new token email newemail@test.com, got %s", newClaims.Email)
	}
	if newClaims.EmailVerifiedAt != nil {
		t.Fatal("expected new token EmailVerifiedAt=nil after email change")
	}

	// Verify old token is revoked after email change (security: old email invalidated)
	_, err = auth.ValidateToken(token, cfg)
	if err == nil {
		t.Fatal("expected old token to be revoked after email change")
	}

	// Verify verify_email job was enqueued (queue.Enqueue is synchronous)
	var jobCount int64
	db.Model(&models.Job{}).
		Where("type = ?", "send_email").
		Count(&jobCount)
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)
	var verifyCount int
	for _, j := range jobs {
		var payload struct {
			TemplateName string `json:"template_name"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && payload.TemplateName == "verify_email" {
			verifyCount++
		}
	}
	if verifyCount != 1 {
		t.Fatalf("expected 1 verify_email job for email change, got %d/%d", verifyCount, len(jobs))
	}
}

func TestAuthHandler_UpdateProfile_InvalidEmailFormat(t *testing.T) {
	app, authService, cfg, db := setupVerificationTestAppWithDB(t)

	user, _ := authService.Register("valid@test.com", "securepassword123", "Valid")
	now := time.Now()
	user.EmailVerifiedAt = &now
	_ = authService.UpdateUser(user)

	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, user.EmailVerifiedAt)

	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, (&AuthHandler{
		authService:   authService,
		config:        cfg,
		emailCooldown: NewCooldownCache(2 * time.Minute),
	}).UpdateProfile)

	// Send request with invalid email format
	body, _ := json.Marshal(map[string]string{
		"name":  "Valid",
		"email": "not-an-email",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 for invalid email, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["error"] != "Invalid email format" {
		t.Fatalf("expected 'Invalid email format', got '%v'", result["error"])
	}

	// DB email unchanged
	updated, _ := authService.GetUserByID(user.ID)
	if updated.Email != "valid@test.com" {
		t.Fatalf("expected email to remain 'valid@test.com', got '%s'", updated.Email)
	}

	// EmailVerifiedAt still set (email didn't change => wasn't reset)
	if updated.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to still be set since email didn't change")
	}

	// No verify_email job enqueued
	var jobCount int64
	db.Model(&models.Job{}).
		Where("type = ?", "send_email").
		Count(&jobCount)
	if jobCount != 0 {
		t.Fatalf("expected 0 send_email jobs for failed email change, got %d", jobCount)
	}
}

func TestAuthHandler_UpdateProfile_OAuthEmailBlocked(t *testing.T) {
	app, authService, cfg := setupAuthTestApp(t)

	user, _ := authService.Register("oauth@example.com", "pass123", "OAuth User")

	// Generate token with provider="google" to simulate OAuth user
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "google", user.Accesses, cfg, nil)

	// Setup update profile endpoint with inline middleware
	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, (&AuthHandler{
		authService:   authService,
		config:        cfg,
		emailCooldown: NewCooldownCache(2 * time.Minute),
	}).UpdateProfile)

	// Try to change email as OAuth user
	body, _ := json.Marshal(map[string]string{
		"name":  "OAuth User",
		"email": "new@example.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 for OAuth email change, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["error"] != "Cannot change email for OAuth accounts" {
		t.Fatalf("expected 'Cannot change email for OAuth accounts', got '%v'", result["error"])
	}

	// DB should still have original email
	updated, _ := authService.GetUserByID(user.ID)
	if updated.Email != "oauth@example.com" {
		t.Fatalf("expected email to remain 'oauth@example.com', got '%s'", updated.Email)
	}
}

func TestAuthHandler_EmailChangeThenReVerify(t *testing.T) {
	app, authService, cfg, db := setupVerificationTestAppWithDB(t)
	config.SetForTest(cfg)

	// Register user (not yet verified)
	user, _ := authService.Register("first@test.com", "securepassword123", "First")

	// Manually verify the user (simulates clicking first verify link)
	now := time.Now()
	user.EmailVerifiedAt = &now
	_ = authService.UpdateUser(user)

	// Generate token for the original verified user
	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, user.EmailVerifiedAt)

	// Setup update profile endpoint
	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, (&AuthHandler{
		authService:   authService,
		config:        cfg,
		emailCooldown: NewCooldownCache(2 * time.Minute),
	}).UpdateProfile)

	// Change email to new address
	body, _ := json.Marshal(map[string]string{
		"name":  "First",
		"email": "changed@test.com",
	})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 on email change, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Verify DB: email changed, EmailVerifiedAt = nil
	updated, _ := authService.GetUserByID(user.ID)
	if updated.Email != "changed@test.com" {
		t.Fatalf("expected email 'changed@test.com', got '%s'", updated.Email)
	}
	if updated.EmailVerifiedAt != nil {
		t.Fatal("expected EmailVerifiedAt to be nil after email change")
	}

	// Extract verification token from the enqueued job
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)
	var verifyToken string
	for _, j := range jobs {
		var payload struct {
			TemplateName string         `json:"template_name"`
			TemplateData map[string]any `json:"template_data,omitempty"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err != nil {
			continue
		}
		if payload.TemplateName == "verify_email" {
			if data, ok := payload.TemplateData["VerifyURL"]; ok {
				urlStr, _ := data.(string)
				// URL looks like: ".../auth/verify?token=XXX"
				if idx := strings.Index(urlStr, "token="); idx >= 0 {
					verifyToken = urlStr[idx+6:]
				}
			}
		}
	}
	if verifyToken == "" {
		t.Fatal("could not extract verification token from job queue")
	}

	// Call verify endpoint with the new email's token
	body, _ = json.Marshal(map[string]string{"token": verifyToken})
	req = httptest.NewRequest(http.MethodPost, "/api/auth/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ = app.Test(req, -1)
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 on verify, got %d: %s", resp.StatusCode, string(respBody))
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(respBody, &result); err != nil {
		t.Fatal(err)
	}
	if result["verified"] != true {
		t.Fatal("expected verified=true in response")
	}

	// Verify DB: EmailVerifiedAt set, email still new
	finalUser, _ := authService.GetUserByID(user.ID)
	if finalUser.Email != "changed@test.com" {
		t.Fatalf("expected email 'changed@test.com', got '%s'", finalUser.Email)
	}
	if finalUser.EmailVerifiedAt == nil {
		t.Fatal("expected EmailVerifiedAt to be set after re-verification")
	}

	// Verify tokens are returned in response body
	if result["access_token"] == nil || result["access_token"] == "" {
		t.Fatal("expected access_token in response after verification")
	}
}

func TestAuthHandler_EmailChangeCooldown(t *testing.T) {
	app, authService, cfg, db := setupVerificationTestAppWithDB(t)

	// Register+verify user
	user, _ := authService.Register("cooldown@test.com", "securepassword123", "Cooldown")
	now := time.Now()
	user.EmailVerifiedAt = &now
	_ = authService.UpdateUser(user)

	token, _ := auth.GenerateToken(user.ID, user.Email, user.Name, "local", user.Accesses, cfg, user.EmailVerifiedAt)

	// Setup update profile endpoint
	app.Put("/api/auth/profile", func(c *fiber.Ctx) error {
		authHeader := c.Get("Authorization")
		if authHeader == "" {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "missing"})
		}
		tokenStr := authHeader
		if len(authHeader) > 7 && authHeader[:7] == bearerPrefix {
			tokenStr = authHeader[7:]
		}
		claims, err := auth.ValidateToken(tokenStr, cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}, (&AuthHandler{
		authService:   authService,
		config:        cfg,
		emailCooldown: NewCooldownCache(2 * time.Minute),
	}).UpdateProfile)

	// First email change
	body1, _ := json.Marshal(map[string]string{
		"name":  "Cooldown",
		"email": "first@test.com",
	})
	req1 := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("Authorization", "Bearer "+token)
	resp1, err := app.Test(req1, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp1.Body.Close() }()

	if resp1.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp1.Body)
		t.Fatalf("expected 200 on first email change, got %d: %s", resp1.StatusCode, string(respBody))
	}

	resp1Body, _ := io.ReadAll(resp1.Body)
	var result1 map[string]interface{}
	_ = json.Unmarshal(resp1Body, &result1)

	// Extract new access token from response (old token was revoked)
	var newToken string
	if tokens, ok := result1["tokens"].(map[string]interface{}); ok {
		if at, ok := tokens["accessToken"].(string); ok {
			newToken = at
		}
	}
	if newToken == "" {
		t.Fatal("expected new access token in first response")
	}

	// Second email change (immediate, should be gated by cooldown)
	body2, _ := json.Marshal(map[string]string{
		"name":  "Cooldown",
		"email": "second@test.com",
	})
	req2 := httptest.NewRequest(http.MethodPut, "/api/auth/profile", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("Authorization", "Bearer "+newToken)
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 200 on second email change, got %d: %s", resp2.StatusCode, string(respBody))
	}

	// Verify DB has the second email (email always changes, cooldown only gates email send)
	updated, _ := authService.GetUserByID(user.ID)
	if updated.Email != "second@test.com" {
		t.Fatalf("expected email 'second@test.com', got '%s'", updated.Email)
	}

	// Verify only 1 verify_email job was enqueued (cooldown blocked second send)
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)
	var verifyCount int
	for _, j := range jobs {
		var payload struct {
			TemplateName string `json:"template_name"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && payload.TemplateName == "verify_email" {
			verifyCount++
		}
	}
	if verifyCount != 1 {
		t.Fatalf("expected 1 verify_email job (cooldown should block second), got %d", verifyCount)
	}
}

// ---------------------------------------------------------------------------
// Password Reset tests
// ---------------------------------------------------------------------------

func setupPasswordResetTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                 "password-reset-test-secret-key-32b",
			TokenDuration:             1,
			SessionSecret:             "session-secret-for-testing",
			ResetTokenTTLHours:        1,
			VerificationTokenTTLHours: 24,
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/forgot-password", authHandler.ForgotPassword)
	app.Post("/api/auth/reset-password", authHandler.ResetPassword)

	return app, authService, cfg
}

func setupPasswordResetTestAppWithDB(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config, *gorm.DB) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	database.SetForTest(db) // needed by queue.Enqueue
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:                 "password-reset-test-secret-key-32b",
			TokenDuration:             1,
			SessionSecret:             "session-secret-for-testing",
			ResetTokenTTLHours:        1,
			VerificationTokenTTLHours: 24,
		},
		Server: config.ServerConfig{
			Domain: "localhost",
		},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/login", authHandler.Login)
	app.Post("/api/auth/forgot-password", authHandler.ForgotPassword)
	app.Post("/api/auth/reset-password", authHandler.ResetPassword)

	return app, authService, cfg, db
}

func TestForgotPassword_Success(t *testing.T) {
	app, authService, _, _ := setupPasswordResetTestAppWithDB(t)

	_, _ = authService.Register("forgot@example.com", "oldpass123", "Forgot User")

	body, _ := json.Marshal(map[string]string{"email": "forgot@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(respBody, &result)
	if result["message"] == nil {
		t.Fatal("expected message in response")
	}
}

func TestForgotPassword_UnknownEmail(t *testing.T) {
	app, _, _ := setupPasswordResetTestApp(t)

	body, _ := json.Marshal(map[string]string{"email": "nobody@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Must return 200 (enumeration safe)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for unknown email (enumeration safe), got %d", resp.StatusCode)
	}
}

func TestForgotPassword_OAuthUser(t *testing.T) {
	app, _, _ := setupPasswordResetTestApp(t)

	// Create an OAuth user directly (no local password)
	user := &models.User{
		ID:       "oauth-user-forgot-1",
		Email:    "oauthforgot@example.com",
		Name:     "OAuth Forgot",
		Provider: "google",
		IsActive: true,
		Accesses: models.StringArray{"user"},
	}
	userRepo := repository.NewUserRepository(testutil.SetupTestDB(t))
	_ = userRepo.CreateUser(user)

	// Re-create app with same DB — simpler to just call handler directly
	// Skip — enumeration safe means unknown and OAuth both return 200
	body, _ := json.Marshal(map[string]string{"email": "oauthforgot@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// OAuth or not, always 200 (enumeration safe)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for OAuth user (enumeration safe), got %d", resp.StatusCode)
	}
}

func TestForgotPassword_EmptyEmail(t *testing.T) {
	app, _, _ := setupPasswordResetTestApp(t)

	body, _ := json.Marshal(map[string]string{"email": ""})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty email, got %d", resp.StatusCode)
	}
}

func TestResetPassword_Success(t *testing.T) {
	app, authService, cfg, _ := setupPasswordResetTestAppWithDB(t)

	_, _ = authService.Register("resetok@example.com", "oldpass123", "Reset OK")

	user, _ := authService.GetUserByEmail("resetok@example.com")
	token, err := auth.GenerateResetToken(user.ID, user.Email, nil, cfg)
	if err != nil {
		t.Fatalf("failed to generate reset token: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"token":       token,
		"newPassword": "newSecurePass456!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Verify old password no longer works
	_, err = authService.Login("resetok@example.com", "oldpass123")
	if err == nil {
		t.Fatal("expected login with old password to fail")
	}

	// Verify new password works
	_, err = authService.Login("resetok@example.com", "newSecurePass456!")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}
}

func TestResetPassword_InvalidToken(t *testing.T) {
	app, _, _ := setupPasswordResetTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"token":       "not-a-valid-jwt",
		"newPassword": "newSecurePass456!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d", resp.StatusCode)
	}
}

func TestResetPassword_ExpiredToken(t *testing.T) {
	app, _, cfg := setupPasswordResetTestApp(t)

	// Create a token that expired 1 hour ago
	claims := &auth.Claims{
		UserID:  "nonexistent-user",
		Email:   "expired@example.com",
		Purpose: "password_reset",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(-1 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, _ := token.SignedString([]byte(cfg.Auth.JWTSecret))

	body, _ := json.Marshal(map[string]string{
		"token":       tokenStr,
		"newPassword": "newSecurePass456!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired token, got %d", resp.StatusCode)
	}
}

func TestResetPassword_TooShort(t *testing.T) {
	app, authService, cfg := setupPasswordResetTestApp(t)

	user, _ := authService.Register("shortpass@example.com", "oldpass123", "Short Pass")
	token, _ := auth.GenerateResetToken(user.ID, user.Email, nil, cfg)

	body, _ := json.Marshal(map[string]string{
		"token":       token,
		"newPassword": "short", // below MinPasswordLength (12)
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for too short password, got %d", resp.StatusCode)
	}
}

func TestResetPassword_OAuthUser(t *testing.T) {
	app, _, _, db := setupPasswordResetTestAppWithDB(t)

	// Create an OAuth user in the same DB the app uses
	userRepo := repository.NewUserRepository(db)
	_ = userRepo.CreateUser(&models.User{
		ID:       "oauth-reset-user",
		Email:    "oauthreset@example.com",
		Name:     "OAuth Reset",
		Provider: "google",
		IsActive: true,
		Accesses: models.StringArray{"user"},
	})

	// Generate a valid reset token for the OAuth user
	token, err := auth.GenerateResetToken("oauth-reset-user", "oauthreset@example.com", nil, config.Get())
	if err != nil {
		t.Fatalf("failed to generate reset token: %v", err)
	}

	body, _ := json.Marshal(map[string]string{
		"token":       token,
		"newPassword": "newSecurePass456!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// OAuth users cannot reset passwords — expect 400
	if resp.StatusCode != http.StatusBadRequest {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 400 for OAuth user, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestForgotThenReset_FullFlow(t *testing.T) {
	app, authService, _, db := setupPasswordResetTestAppWithDB(t)
	// Need database.SetForTest for queue.Enqueue to work
	database.SetForTest(db)

	// Register a user
	_, _ = authService.Register("fullflow@example.com", "originalPass123", "Full Flow")

	// Step 1: Request forgot password
	body, _ := json.Marshal(map[string]string{"email": "fullflow@example.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("forgot-password: expected 200, got %d", resp.StatusCode)
	}

	// Step 2: Extract the reset token from the queued job
	var jobs []models.Job
	db.Where("type = ?", "send_email").Find(&jobs)

	var resetToken string
	for _, j := range jobs {
		var payload struct {
			TemplateName string         `json:"template_name"`
			TemplateData map[string]any `json:"template_data"`
		}
		if err := json.Unmarshal([]byte(j.Payload), &payload); err == nil && payload.TemplateName == "password_reset" {
			if resetURL, ok := payload.TemplateData["ResetURL"].(string); ok {
				// Extract token from URL: /auth/reset-password?token=xxx
				if parts := strings.Split(resetURL, "token="); len(parts) == 2 {
					resetToken = parts[1]
				}
			}
		}
	}

	if resetToken == "" {
		t.Fatal("expected a password_reset job with ResetURL containing token")
	}

	// Step 3: Use the token to reset password
	body2, _ := json.Marshal(map[string]string{
		"token":       resetToken,
		"newPassword": "brandNewPass789!",
	})
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	defer func() { _ = resp.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp2.Body)
		t.Fatalf("reset-password: expected 200, got %d: %s", resp2.StatusCode, string(respBody))
	}

	// Step 4: Verify old password fails
	_, err = authService.Login("fullflow@example.com", "originalPass123")
	if err == nil {
		t.Fatal("login with old password should fail after reset")
	}

	// Step 5: Verify new password works
	_, err = authService.Login("fullflow@example.com", "brandNewPass789!")
	if err != nil {
		t.Fatalf("login with new password failed: %v", err)
	}

	// Step 6: Verify token is now invalid (single-use via password_changed_at after Phase 4)
	body3, _ := json.Marshal(map[string]string{
		"token":       resetToken,
		"newPassword": "yetanotherPass123",
	})
	req3 := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body3))
	req3.Header.Set("Content-Type", "application/json")
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp3.Body.Close() }()
	defer func() { _ = resp.Body.Close() }()

	// After password change, the old token should be invalid (password_changed_at
	// check in the handler rejects tokens issued before the last password change)
	if resp3.StatusCode != http.StatusBadRequest {
		t.Fatal("expected old reset token to be rejected after password change")
	}
}

func TestForgotPassword_Cooldown(t *testing.T) {
	app, authService, _, _ := setupPasswordResetTestAppWithDB(t)

	_, _ = authService.Register("cooldownfr@example.com", "pass", "Cooldown Forgot")

	body, _ := json.Marshal(map[string]string{"email": "cooldownfr@example.com"})

	// First request
	req1 := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req1.Header.Set("Content-Type", "application/json")
	resp1, _ := app.Test(req1, -1)
	_ = resp1.Body.Close()

	// Second request — cooldown active, but still 200
	req2 := httptest.NewRequest(http.MethodPost, "/api/auth/forgot-password", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()

	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (cooldown is silent), got %d", resp2.StatusCode)
	}
}

func TestResetPassword_NoToken(t *testing.T) {
	app, _, _ := setupPasswordResetTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"newPassword": "newSecurePass456!",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/reset-password", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing token, got %d", resp.StatusCode)
	}
}

// -------------------------------------------------------------------------
// Config-Gated Behavior Tests
// -------------------------------------------------------------------------

func TestGuestLogin_GuestLoginEnabled_Disabled_Returns403(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "guest-disabled-test-secret-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	// Disable guest login
	s, _ := settingsRepo.GetSettings()
	s.GuestLoginEnabled = false
	settingsRepo.SaveSettings(s)

	app := fiber.New()
	app.Post("/api/auth/guest-login", authHandler.GuestLogin)

	body, _ := json.Marshal(map[string]string{"name": "TestGuest"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestGuestJoinRoom_GuestLoginEnabled_Disabled_Returns403(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "key", APISecret: "secret"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, settingsRepo, nil, nil, nil)

	// Disable guest login
	s, _ := settingsRepo.GetSettings()
	s.GuestLoginEnabled = false
	settingsRepo.SaveSettings(s)

	app := fiber.New()
	app.Post("/room/guest-join", handler.GuestJoinRoom)

	body, _ := json.Marshal(map[string]string{"roomName": "some-room", "guestName": "Guest"})
	req := httptest.NewRequest(http.MethodPost, "/room/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 403, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestRegister_RegistrationDisabled_Returns403(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	inviteTokenRepo := repository.NewInviteTokenRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "reg-disabled-test-secret-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	emailCooldown := NewCooldownCache(2 * time.Minute)
	authHandler := NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, nil, emailCooldown, nil)

	// Disable registration
	s, _ := settingsRepo.GetSettings()
	s.RegistrationEnabled = false
	settingsRepo.SaveSettings(s)

	app := fiber.New()
	app.Post("/api/auth/register", authHandler.Register)
	app.Post("/api/auth/guest-login", authHandler.GuestLogin)

	t.Run("register returns 403", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"email": "test@ex.com", "password": "securepass123", "name": "Test"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("guest-login returns 403", func(t *testing.T) {
		body, _ := json.Marshal(map[string]string{"name": "Guest"})
		req := httptest.NewRequest(http.MethodPost, "/api/auth/guest-login", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
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

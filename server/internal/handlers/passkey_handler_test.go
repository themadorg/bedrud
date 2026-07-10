package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func setupPasskeyTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "handler-auth-test-secret-key-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret-for-testing",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	auth.InitializeSessionStore(cfg.Auth.SessionSecret, false)
	cs := auth.NewChallengeStore(5)
	h := NewAuthHandler(authService, cfg, settingsRepo, nil, cs, NewCooldownCache(0), nil)

	authMW := func(c *fiber.Ctx) error {
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
	}

	app := fiber.New()
	app.Post("/api/auth/passkey/register/begin", authMW, h.PasskeyRegisterBegin)
	app.Post("/api/auth/passkey/register/finish", authMW, h.PasskeyRegisterFinish)
	app.Post("/api/auth/passkey/login/begin", h.PasskeyLoginBegin)
	app.Post("/api/auth/passkey/login/finish", h.PasskeyLoginFinish)
	app.Post("/api/auth/passkey/signup/begin", h.PasskeySignupBegin)
	app.Post("/api/auth/passkey/signup/finish", h.PasskeySignupFinish)

	if _, err := authService.Register("pk@ex.com", "securepass123", "Passkey User"); err != nil {
		t.Fatal(err)
	}

	return app, authService, cfg
}

func TestPasskeyRegisterBegin_Success(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/begin", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil || result["challenge"] == "" {
		t.Fatalf("expected challenge, got %#v", result)
	}
	if result["user"] == nil || result["rp"] == nil {
		t.Fatalf("expected user+rp shape, got %#v", result)
	}
}

func TestPasskeyRegisterBegin_Unauthenticated(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/begin", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestPasskeyRegisterFinish_NoChallenge(t *testing.T) {
	app, authService, cfg := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"clientDataJSON":    "e30",
		"attestationObject": "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/register/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "pk@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeyLoginBegin_Success(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/begin", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil || result["challenge"] == "" {
		t.Fatalf("expected challenge, got %#v", result)
	}
}

func TestPasskeyLoginFinish_NoChallenge(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"credentialId":      "e30",
		"clientDataJSON":    "e30",
		"authenticatorData": "e30",
		"signature":         "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/login/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeySignupBegin_Success(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"email": "newpk@ex.com",
		"name":  "New PK",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["challenge"] == nil {
		t.Fatalf("expected challenge, got %#v", result)
	}
}

func TestPasskeySignupBegin_MissingFields(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{"email": "x@ex.com"})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/begin", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestPasskeySignupFinish_NoChallenge(t *testing.T) {
	app, _, _ := setupPasskeyTestApp(t)
	body, _ := json.Marshal(map[string]string{
		"clientDataJSON":    "e30",
		"attestationObject": "e30",
	})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/passkey/signup/finish", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func TestBeginAuthHandler_InvalidProvider(t *testing.T) {
	app := fiber.New()
	app.Get("/api/auth/:provider/login", BeginAuthHandler)

	req := httptest.NewRequest(http.MethodGet, "/api/auth/not-a-provider/login", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestOAuthCallback_MissingSession(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
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
	h := NewAuthHandler(authService, cfg, nil, nil, nil, NewCooldownCache(0), nil)

	app := fiber.New()
	app.Get("/api/auth/:provider/callback", h.CallbackHandler)

	// No gothic session / code — fails offline without IdP network
	req := httptest.NewRequest(http.MethodGet, "/api/auth/google/callback", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 400 {
		t.Fatalf("expected error status, got %d", resp.StatusCode)
	}
}

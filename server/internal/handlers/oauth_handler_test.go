package handlers

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/markbates/goth"
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

func setupOAuthFinishApp(t *testing.T) (*fiber.App, *AuthHandler, *repository.UserRepository, *repository.SettingsRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	database.SetForTest(db)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "oauth-finish-test-secret-key-32b",
			TokenDuration: 1,
			SessionSecret: "session-secret-for-testing",
		},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	h := NewAuthHandler(authService, cfg, settingsRepo, nil, nil, NewCooldownCache(0), nil)
	app := fiber.New()
	// Drive finishOAuthLogin without gothic
	app.Post("/oauth/finish", func(c *fiber.Ctx) error {
		var gu goth.User
		if err := c.BodyParser(&gu); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "bad"})
		}
		return h.finishOAuthLogin(c, gu)
	})
	return app, h, userRepo, settingsRepo
}

func TestOAuthFinish_DisabledRegistration_NewUser(t *testing.T) {
	app, _, _, settingsRepo := setupOAuthFinishApp(t)
	s, _ := settingsRepo.GetSettings()
	s.RegistrationEnabled = false
	_ = settingsRepo.SaveSettings(s)
	body := `{"Email":"new@ex.com","Name":"New","Provider":"google"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/finish", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 403, got %d: %s", resp.StatusCode, b)
	}
}

func TestOAuthFinish_LocalEmailCollision(t *testing.T) {
	app, _, userRepo, _ := setupOAuthFinishApp(t)
	_ = userRepo.CreateUser(&models.User{
		ID: "local-1", Email: "same@ex.com", Name: "Local", Provider: "local",
		IsActive: true, Accesses: models.StringArray{"user"},
	})
	body := `{"Email":"same@ex.com","Name":"G","Provider":"google"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/finish", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 409 collision, got %d: %s", resp.StatusCode, b)
	}
	u, _ := userRepo.GetUserByEmailAndProvider("same@ex.com", "local")
	if u == nil || u.Provider != "local" {
		t.Fatal("local account must remain")
	}
	g, _ := userRepo.GetUserByEmailAndProvider("same@ex.com", "google")
	if g != nil {
		t.Fatal("must not create google account on collision")
	}
}

func TestOAuthFinish_ExistingOAuthUser_OK(t *testing.T) {
	app, _, userRepo, _ := setupOAuthFinishApp(t)
	_ = userRepo.CreateUser(&models.User{
		ID: "g-1", Email: "g@ex.com", Name: "G", Provider: "google",
		IsActive: true, Accesses: models.StringArray{"user"},
	})
	body := `{"Email":"g@ex.com","Name":"G2","Provider":"google"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/finish", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
}

func TestOAuthFinish_UnbansAfterForceLogout(t *testing.T) {
	app, _, userRepo, _ := setupOAuthFinishApp(t)
	_ = userRepo.CreateUser(&models.User{
		ID: "g-ban", Email: "gb@ex.com", Name: "G", Provider: "google",
		IsActive: true, Accesses: models.StringArray{"user"},
	})
	auth.BanUser("g-ban")
	t.Cleanup(func() { auth.UnbanUser("g-ban") })
	if !auth.IsUserBanned("g-ban") {
		t.Fatal("precondition: banned")
	}
	body := `{"Email":"gb@ex.com","Name":"G","Provider":"google"}`
	req := httptest.NewRequest(http.MethodPost, "/oauth/finish", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("want 200, got %d: %s", resp.StatusCode, b)
	}
	if auth.IsUserBanned("g-ban") {
		t.Fatal("OAuth login must clear force-logout ban")
	}
}

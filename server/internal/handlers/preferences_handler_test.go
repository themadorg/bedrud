package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func setupPrefsTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *config.Config) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	passkeyRepo := repository.NewPasskeyRepository(db)
	prefsRepo := repository.NewUserPreferencesRepository(db)
	authService := auth.NewAuthService(userRepo, passkeyRepo)
	cfg := &config.Config{
		Auth: config.AuthConfig{
			JWTSecret:     "handler-auth-test-secret-key-32b",
			TokenDuration: 1,
		},
	}
	config.SetForTest(cfg)
	h := NewPreferencesHandler(prefsRepo)

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
	app.Get("/api/auth/preferences", authMW, h.GetPreferences)
	app.Put("/api/auth/preferences", authMW, h.UpdatePreferences)

	if _, err := authService.Register("prefs@ex.com", "securepass123", "Prefs User"); err != nil {
		t.Fatal(err)
	}

	return app, authService, cfg
}

func TestGetPreferences_DefaultEmpty(t *testing.T) {
	app, authService, cfg := setupPrefsTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/preferences", http.NoBody)
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "prefs@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var result map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["preferencesJson"] != "{}" {
		t.Fatalf("got %q", result["preferencesJson"])
	}
}

func TestPreferences_PutGetRoundTrip(t *testing.T) {
	app, authService, cfg := setupPrefsTestApp(t)
	hdr := authHeaderFor(t, authService, cfg, "prefs@ex.com")
	body, _ := json.Marshal(map[string]string{"preferencesJson": `{"theme":"dark"}`})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", hdr)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("put status %d: %s", resp.StatusCode, b)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/auth/preferences", http.NoBody)
	req.Header.Set("Authorization", hdr)
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var result map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["preferencesJson"] != `{"theme":"dark"}` {
		t.Fatalf("got %q", result["preferencesJson"])
	}
}

func TestUpdatePreferences_Oversize(t *testing.T) {
	app, authService, cfg := setupPrefsTestApp(t)
	big := `{"x":"` + strings.Repeat("a", 5*1024) + `"}`
	body, _ := json.Marshal(map[string]string{"preferencesJson": big})
	req := httptest.NewRequest(http.MethodPut, "/api/auth/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, "prefs@ex.com"))
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("expected 413, got %d", resp.StatusCode)
	}
}

func TestPreferences_Unauthenticated(t *testing.T) {
	app, _, _ := setupPrefsTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/auth/preferences", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}

	body, _ := json.Marshal(map[string]string{"preferencesJson": `{}`})
	req = httptest.NewRequest(http.MethodPut, "/api/auth/preferences", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err = app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 put, got %d", resp.StatusCode)
	}
}

package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func TestHealthAndReady(t *testing.T) {
	app := fiber.New()
	app.Get("/api/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy"})
	})
	app.Get("/api/ready", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "ready"})
	})

	for _, path := range []string{"/api/health", "/api/ready"} {
		req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", path, resp.StatusCode)
		}
		var body map[string]string
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if body["status"] == "" {
			t.Fatalf("%s: empty status", path)
		}
	}
}

func getHealth(t *testing.T, settingsRepo *repository.SettingsRepository) map[string]any {
	t.Helper()
	app := fiber.New()
	app.Get("/api/health", Health(settingsRepo, "1.2.3"))

	resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/api/health", http.NoBody), -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	}
	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestHealth_ReturnsDefaultNameAndVersion(t *testing.T) {
	settingsRepo := repository.NewSettingsRepository(testutil.SetupTestDB(t))
	cfg := &config.Config{}
	cfg.Server.Domain = "meet.example.com"
	settingsRepo.SetConfig(cfg)

	body := getHealth(t, settingsRepo)

	if body["status"] != "healthy" {
		t.Errorf("expected status 'healthy', got %v", body["status"])
	}
	if body["serverName"] != "meet.example.com" {
		t.Errorf("expected serverName 'meet.example.com', got %v", body["serverName"])
	}
	if body["version"] != "1.2.3" {
		t.Errorf("expected version '1.2.3', got %v", body["version"])
	}
}

func TestHealth_ReturnsConfiguredName(t *testing.T) {
	settingsRepo := repository.NewSettingsRepository(testutil.SetupTestDB(t))
	settingsRepo.SetConfig(&config.Config{})
	if err := settingsRepo.SaveSettings(&models.SystemSettings{ServerName: "Team Calls"}); err != nil {
		t.Fatal(err)
	}

	body := getHealth(t, settingsRepo)

	if body["serverName"] != "Team Calls" {
		t.Errorf("expected serverName 'Team Calls', got %v", body["serverName"])
	}
}

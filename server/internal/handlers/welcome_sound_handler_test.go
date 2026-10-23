package handlers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

const welcomeSoundTestEmail = "sound@ex.com"

func readWelcomeSoundFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "storage", "testdata", "welcome-sound", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func enableWelcomeSounds(t *testing.T, settingsRepo *repository.SettingsRepository) {
	t.Helper()
	settings, err := settingsRepo.GetSettings()
	if err != nil {
		t.Fatal(err)
	}
	settings.WelcomeSoundEnabled = true
	if err := settingsRepo.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
}

func decodeJSONBody(t *testing.T, resp *http.Response) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result
}

func setupWelcomeSoundTestApp(t *testing.T) (*fiber.App, *auth.AuthService, *repository.SettingsRepository, *config.Config) {
	t.Helper()
	t.Cleanup(storage.SetWelcomeSoundDirForTest(t.TempDir()))

	db := testutil.SetupTestDB(t)
	authService := auth.NewAuthService(repository.NewUserRepository(db), repository.NewPasskeyRepository(db))
	settingsRepo := repository.NewSettingsRepository(db)
	cfg := &config.Config{
		Auth:   config.AuthConfig{JWTSecret: "handler-auth-test-secret-key-32b", TokenDuration: 1, SessionSecret: "session-secret-for-testing"},
		Server: config.ServerConfig{Domain: "localhost"},
	}
	config.SetForTest(cfg)
	handler := NewAuthHandler(authService, cfg, settingsRepo, nil, nil, NewCooldownCache(0), nil)

	authMiddleware := func(c *fiber.Ctx) error {
		claims, err := auth.ValidateToken(strings.TrimPrefix(c.Get("Authorization"), bearerPrefix), cfg)
		if err != nil {
			return c.Status(http.StatusUnauthorized).JSON(fiber.Map{"error": "invalid"})
		}
		c.Locals("user", claims)
		return c.Next()
	}
	app := fiber.New()
	app.Post("/api/auth/me/welcome-sound", authMiddleware, handler.UploadWelcomeSound)
	app.Delete("/api/auth/me/welcome-sound", authMiddleware, handler.DeleteWelcomeSound)

	if _, err := authService.Register(welcomeSoundTestEmail, "securepass123", "Sound User"); err != nil {
		t.Fatal(err)
	}
	return app, authService, settingsRepo, cfg
}

func uploadWelcomeSound(t *testing.T, app *fiber.App, authorization string, data []byte) *http.Response {
	t.Helper()
	body, contentType := multipartAvatar(t, "welcomeSound", "sound.mp3", data)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/me/welcome-sound", body)
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("Authorization", authorization)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func TestAuthHandler_UploadWelcomeSound(t *testing.T) {
	t.Run("should refuse uploads while the feature is off", func(t *testing.T) {
		app, authService, _, cfg := setupWelcomeSoundTestApp(t)

		resp := uploadWelcomeSound(t, app, authHeaderFor(t, authService, cfg, welcomeSoundTestEmail), readWelcomeSoundFixture(t, "valid.mp3"))
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("should store a valid clip and return its URL", func(t *testing.T) {
		app, authService, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)

		resp := uploadWelcomeSound(t, app, authHeaderFor(t, authService, cfg, welcomeSoundTestEmail), readWelcomeSoundFixture(t, "valid.mp3"))
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		url, _ := decodeJSONBody(t, resp)["welcomeSoundUrl"].(string)
		if !strings.HasPrefix(url, "/uploads/welcome-sounds/") {
			t.Fatalf("expected a welcome sound URL, got %q", url)
		}
	})

	t.Run("should return the reason a clip was rejected", func(t *testing.T) {
		app, authService, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)

		resp := uploadWelcomeSound(t, app, authHeaderFor(t, authService, cfg, welcomeSoundTestEmail), readWelcomeSoundFixture(t, "too-loud.mp3"))
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
		if message, _ := decodeJSONBody(t, resp)["error"].(string); message != storage.ErrWelcomeSoundTooLoud.Error() {
			t.Fatalf("expected the loudness message, got %q", message)
		}
	})

	t.Run("should refuse a third change in one day", func(t *testing.T) {
		app, authService, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)
		authorization := authHeaderFor(t, authService, cfg, welcomeSoundTestEmail)
		clip := readWelcomeSoundFixture(t, "valid.mp3")
		for change := 1; change <= 2; change++ {
			resp := uploadWelcomeSound(t, app, authorization, clip)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("change %d: expected 200, got %d", change, resp.StatusCode)
			}
		}

		resp := uploadWelcomeSound(t, app, authorization, clip)
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusTooManyRequests {
			t.Fatalf("expected 429, got %d", resp.StatusCode)
		}
	})

	t.Run("should refuse guests", func(t *testing.T) {
		app, _, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)
		token, err := auth.GenerateToken("guest-1", "guest-1@guest.local", "Guest", "guest", []string{"guest"}, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}

		resp := uploadWelcomeSound(t, app, bearerPrefix+token, readWelcomeSoundFixture(t, "valid.mp3"))
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("expected 403, got %d", resp.StatusCode)
		}
	})

	t.Run("should require a file", func(t *testing.T) {
		app, authService, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)
		body, contentType := multipartAvatar(t, "", "", nil)
		req := httptest.NewRequest(http.MethodPost, "/api/auth/me/welcome-sound", body)
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("Authorization", authHeaderFor(t, authService, cfg, welcomeSoundTestEmail))

		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", resp.StatusCode)
		}
	})
}

func TestAuthHandler_DeleteWelcomeSound(t *testing.T) {
	t.Run("should clear the clip even while the feature is off", func(t *testing.T) {
		app, authService, settingsRepo, cfg := setupWelcomeSoundTestApp(t)
		enableWelcomeSounds(t, settingsRepo)
		authorization := authHeaderFor(t, authService, cfg, welcomeSoundTestEmail)
		upload := uploadWelcomeSound(t, app, authorization, readWelcomeSoundFixture(t, "valid.mp3"))
		upload.Body.Close()
		settings, _ := settingsRepo.GetSettings()
		settings.WelcomeSoundEnabled = false
		if err := settingsRepo.SaveSettings(settings); err != nil {
			t.Fatal(err)
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/auth/me/welcome-sound", http.NoBody)
		req.Header.Set("Authorization", authorization)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		if url := decodeJSONBody(t, resp)["welcomeSoundUrl"]; url != "" {
			t.Fatalf("expected an empty welcomeSoundUrl, got %v", url)
		}
	})
}

func TestRoomHandler_GetParticipantProfile_WelcomeSound(t *testing.T) {
	const soundURL = "/uploads/welcome-sounds/creator-user.mp3?v=1"

	setup := func(t *testing.T) (*fiber.App, *repository.SettingsRepository, string) {
		t.Helper()
		db := testutil.SetupTestDB(t)
		roomRepo := repository.NewRoomRepository(db)
		settingsRepo := repository.NewSettingsRepository(db)
		lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
		handler := NewRoomHandler(testutil.NewMockRoomService(), &lkCfg, &config.ChatConfig{}, roomRepo, repository.NewUserRepository(db), nil, settingsRepo, nil, nil, nil)

		users := []models.User{
			{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, WelcomeSoundURL: soundURL},
			{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true},
		}
		for index := range users {
			db.Create(&users[index])
		}
		room, err := roomRepo.CreateRoom("creator-user", "welcome-room", true, "standard", 0, &models.RoomSettings{})
		if err != nil {
			t.Fatal(err)
		}
		_ = roomRepo.AddParticipant(room.ID, "other-user")

		app := fiber.New()
		app.Use(func(c *fiber.Ctx) error {
			c.Locals("user", &auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}})
			return c.Next()
		})
		app.Get("/room/:roomId/participant/:identity/profile", handler.GetParticipantProfile)
		return app, settingsRepo, room.ID
	}
	fetchSoundURL := func(t *testing.T, app *fiber.App, roomID string) any {
		t.Helper()
		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/room/"+roomID+"/participant/creator-user/profile", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		return decodeJSONBody(t, resp)["welcomeSoundUrl"]
	}

	t.Run("should hide the welcome sound while the feature is off", func(t *testing.T) {
		app, _, roomID := setup(t)

		if url := fetchSoundURL(t, app, roomID); url != "" {
			t.Fatalf("expected an empty welcomeSoundUrl, got %v", url)
		}
	})

	t.Run("should share the welcome sound while the feature is on", func(t *testing.T) {
		app, settingsRepo, roomID := setup(t)
		enableWelcomeSounds(t, settingsRepo)

		if url := fetchSoundURL(t, app, roomID); url != soundURL {
			t.Fatalf("expected %q, got %v", soundURL, url)
		}
	})
}

func TestUsersHandler_ClearUserWelcomeSound(t *testing.T) {
	t.Run("should clear another user's welcome sound", func(t *testing.T) {
		t.Cleanup(storage.SetWelcomeSoundDirForTest(t.TempDir()))
		app, userRepo := setupUsersTestApp(t)
		target := &models.User{ID: "target-user", Email: "target@ex.com", Name: "Target", Provider: "local", IsActive: true, WelcomeSoundURL: "/uploads/welcome-sounds/target-user.mp3?v=1"}
		if err := userRepo.CreateUser(target); err != nil {
			t.Fatal(err)
		}

		resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/admin/users/target-user/welcome-sound", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		updated, err := userRepo.GetUserByID("target-user")
		if err != nil || updated == nil {
			t.Fatalf("reload user: %v", err)
		}
		if updated.WelcomeSoundURL != "" {
			t.Fatalf("expected an empty URL, got %q", updated.WelcomeSoundURL)
		}
	})

	t.Run("should report an unknown user", func(t *testing.T) {
		app, _ := setupUsersTestApp(t)

		resp, err := app.Test(httptest.NewRequest(http.MethodDelete, "/admin/users/nobody/welcome-sound", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("expected 404, got %d", resp.StatusCode)
		}
	})
}

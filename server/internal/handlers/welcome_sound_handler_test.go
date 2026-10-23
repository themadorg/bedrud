package handlers

import (
	"bytes"
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

const welcomeRoomSoundURL = "/uploads/welcome-sounds/creator-user.mp3?v=1"

type welcomeRoomFixture struct {
	app          *fiber.App
	settingsRepo *repository.SettingsRepository
	roomRepo     *repository.RoomRepository
	roomID       string
	// claims is read on every request, so a test can switch who is calling.
	claims *auth.Claims
}

// setupWelcomeRoom creates a room owned by "creator-user" (who has a welcome sound) with
// "other-user" in it, and wires the profile and both room settings endpoints.
func setupWelcomeRoom(t *testing.T, settings models.RoomSettings) *welcomeRoomFixture {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(testutil.NewMockRoomService(), &lkCfg, &config.ChatConfig{}, roomRepo, repository.NewUserRepository(db), nil, settingsRepo, nil, nil, nil)

	users := []models.User{
		{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, WelcomeSoundURL: welcomeRoomSoundURL},
		{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true},
	}
	for index := range users {
		db.Create(&users[index])
	}
	room, err := roomRepo.CreateRoom("creator-user", "welcome-room", true, "standard", 0, &settings)
	if err != nil {
		t.Fatal(err)
	}
	_ = roomRepo.AddParticipant(room.ID, "other-user")

	fixture := &welcomeRoomFixture{
		settingsRepo: settingsRepo,
		roomRepo:     roomRepo,
		roomID:       room.ID,
		claims:       &auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}},
	}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", fixture.claims)
		return c.Next()
	})
	app.Get("/room/:roomId/participant/:identity/profile", handler.GetParticipantProfile)
	app.Put("/room/:roomId/settings", handler.UpdateSettings)
	app.Put("/admin/rooms/:roomId", handler.AdminUpdateRoom)
	fixture.app = app
	return fixture
}

func (fixture *welcomeRoomFixture) putJSON(t *testing.T, path string, body map[string]any) {
	t.Helper()
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPut, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	resp, err := fixture.app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		responseBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("PUT %s: expected 200, got %d: %s", path, resp.StatusCode, responseBody)
	}
}

func (fixture *welcomeRoomFixture) storedAllowWelcomeSounds(t *testing.T) *bool {
	t.Helper()
	room, err := fixture.roomRepo.GetRoom(fixture.roomID)
	if err != nil || room == nil {
		t.Fatalf("reload room: %v", err)
	}
	return room.Settings.AllowWelcomeSounds
}

func TestRoomHandler_GetParticipantProfile_WelcomeSound(t *testing.T) {
	const soundURL = welcomeRoomSoundURL

	setup := func(t *testing.T) (*fiber.App, *repository.SettingsRepository, string) {
		t.Helper()
		fixture := setupWelcomeRoom(t, models.RoomSettings{})
		return fixture.app, fixture.settingsRepo, fixture.roomID
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

	t.Run("should hide the welcome sound in a room whose host turned them off", func(t *testing.T) {
		off := false
		fixture := setupWelcomeRoom(t, models.RoomSettings{AllowWelcomeSounds: &off})
		enableWelcomeSounds(t, fixture.settingsRepo)

		if url := fetchSoundURL(t, fixture.app, fixture.roomID); url != "" {
			t.Fatalf("expected an empty welcomeSoundUrl, got %v", url)
		}
	})
}

func TestRoomHandler_UpdateSettings_WelcomeSounds(t *testing.T) {
	ownerClaims := &auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}
	baseSettings := map[string]any{"allowChat": true, "allowVideo": true, "allowAudio": true}

	t.Run("should let the owner turn welcome sounds off", func(t *testing.T) {
		fixture := setupWelcomeRoom(t, models.RoomSettings{})
		fixture.claims = ownerClaims
		settings := map[string]any{"allowWelcomeSounds": false}
		for key, value := range baseSettings {
			settings[key] = value
		}

		fixture.putJSON(t, "/room/"+fixture.roomID+"/settings", map[string]any{"settings": settings})

		if stored := fixture.storedAllowWelcomeSounds(t); stored == nil || *stored {
			t.Fatalf("expected welcome sounds stored as off, got %v", stored)
		}
	})

	t.Run("should keep the stored choice when a client leaves the field out", func(t *testing.T) {
		off := false
		fixture := setupWelcomeRoom(t, models.RoomSettings{AllowWelcomeSounds: &off})
		fixture.claims = ownerClaims

		fixture.putJSON(t, "/room/"+fixture.roomID+"/settings", map[string]any{"settings": baseSettings})

		if stored := fixture.storedAllowWelcomeSounds(t); stored == nil || *stored {
			t.Fatalf("expected welcome sounds to stay off, got %v", stored)
		}
	})
}

func TestRoomHandler_AdminUpdateRoom_WelcomeSounds(t *testing.T) {
	adminClaims := &auth.Claims{UserID: "admin-user", Email: "admin@ex.com", Accesses: []string{"superadmin"}}

	t.Run("should let a superadmin turn welcome sounds off", func(t *testing.T) {
		fixture := setupWelcomeRoom(t, models.RoomSettings{})
		fixture.claims = adminClaims

		fixture.putJSON(t, "/admin/rooms/"+fixture.roomID, map[string]any{"settings": map[string]any{"allowWelcomeSounds": false}})

		if stored := fixture.storedAllowWelcomeSounds(t); stored == nil || *stored {
			t.Fatalf("expected welcome sounds stored as off, got %v", stored)
		}
	})

	t.Run("should leave welcome sounds alone when the field is left out", func(t *testing.T) {
		off := false
		fixture := setupWelcomeRoom(t, models.RoomSettings{AllowWelcomeSounds: &off})
		fixture.claims = adminClaims

		fixture.putJSON(t, "/admin/rooms/"+fixture.roomID, map[string]any{"settings": map[string]any{"isPersistent": true}})

		if stored := fixture.storedAllowWelcomeSounds(t); stored == nil || *stored {
			t.Fatalf("expected welcome sounds to stay off, got %v", stored)
		}
	})
}

func TestUsersHandler_GetUserDetail_WelcomeSound(t *testing.T) {
	t.Run("should include the user's welcome sound for moderation", func(t *testing.T) {
		app, userRepo := setupUsersTestApp(t)
		const soundURL = "/uploads/welcome-sounds/target-user.mp3?v=1"
		if err := userRepo.CreateUser(&models.User{ID: "target-user", Email: "target@ex.com", Name: "Target", Provider: "local", IsActive: true, WelcomeSoundURL: soundURL}); err != nil {
			t.Fatal(err)
		}

		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/admin/users/target-user", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()

		user, _ := decodeJSONBody(t, resp)["user"].(map[string]any)
		if user["welcomeSoundUrl"] != soundURL {
			t.Fatalf("expected %q, got %v", soundURL, user["welcomeSoundUrl"])
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

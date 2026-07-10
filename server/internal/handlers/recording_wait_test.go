package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

func setupRecordingWaitApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "mod-user",
			Email:    "mod@ex.com",
			Name:     "Mod",
			Accesses: []string{"moderator"},
		})
		return c.Next()
	})
	app.Get("/api/rooms/:id/recordings/:rid/wait", handler.WaitRecordingReady)
	return app, roomRepo, recordingRepo
}

func TestWaitRecordingReady_InvalidRoomID(t *testing.T) {
	app, _, _ := setupRecordingWaitApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/not-a-uuid/recordings/"+uuid.NewString()+"/wait", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestWaitRecordingReady_NotFound(t *testing.T) {
	app, roomRepo, _ := setupRecordingWaitApp(t)
	room, err := roomRepo.CreateRoom("mod-user", "wait-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings/"+uuid.NewString()+"/wait", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

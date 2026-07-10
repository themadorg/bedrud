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
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
)

func setupRoomGapsApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository, *auth.Claims) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, recordingRepo, nil, nil, nil, nil)

	claims := &auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Name: "Creator", Accesses: []string{"user"}}
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		// optional auth for presence (public)
		if c.Get("X-Skip-Auth") == "1" {
			return c.Next()
		}
		c.Locals("user", claims)
		return c.Next()
	})
	app.Get("/room/archived", handler.ListArchivedRooms)
	app.Post("/room/refresh-token", handler.RefreshLiveKitToken)
	app.Get("/room/:roomId/presence", handler.GetRoomPresence)
	app.Post("/admin/rooms/bulk/suspend", handler.BulkSuspendRooms)
	app.Post("/admin/rooms/bulk/close", handler.BulkCloseRooms)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	return app, roomRepo, recordingRepo, claims
}

func TestListArchivedRooms_Empty(t *testing.T) {
	app, _, _, _ := setupRoomGapsApp(t)
	req := httptest.NewRequest(http.MethodGet, "/room/archived", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestListArchivedRooms_WithArchived(t *testing.T) {
	app, roomRepo, _, _ := setupRoomGapsApp(t)
	room, err := roomRepo.CreateRoom("creator-user", "arch-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	if err := roomRepo.SoftDeleteRoom(room.ID); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/room/archived", http.NoBody)
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
	rooms, _ := result["rooms"].([]interface{})
	if len(rooms) < 1 {
		t.Fatalf("expected archived rooms, got %#v", result)
	}
}

func TestRefreshLiveKitToken_Success(t *testing.T) {
	app, roomRepo, _, _ := setupRoomGapsApp(t)
	if _, err := roomRepo.CreateRoom("creator-user", "tok-room", true, "standard", 0, &models.RoomSettings{}); err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string]string{"roomName": "tok-room"})
	req := httptest.NewRequest(http.MethodPost, "/room/refresh-token", bytes.NewReader(body))
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
	if result["token"] == nil || result["token"] == "" {
		t.Fatalf("expected token, got %#v", result)
	}
}

func TestRefreshLiveKitToken_NotFound(t *testing.T) {
	app, _, _, _ := setupRoomGapsApp(t)
	body, _ := json.Marshal(map[string]string{"roomName": "nope"})
	req := httptest.NewRequest(http.MethodPost, "/room/refresh-token", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetRoomPresence_NotFound(t *testing.T) {
	app, _, _, _ := setupRoomGapsApp(t)
	req := httptest.NewRequest(http.MethodGet, "/room/nonexistent/presence", http.NoBody)
	req.Header.Set("X-Skip-Auth", "1")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetRoomPresence_PublicEmpty(t *testing.T) {
	app, roomRepo, _, _ := setupRoomGapsApp(t)
	room, err := roomRepo.CreateRoom("creator-user", "pres-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	// Authenticated identity list
	req := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/presence", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}

	// Unauthenticated identity list denied
	req2 := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/presence", http.NoBody)
	req2.Header.Set("X-Skip-Auth", "1")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 unauth identity list, got %d", resp2.StatusCode)
	}

	// Public countOnly without auth OK
	req3 := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/presence?countOnly=1", http.NoBody)
	req3.Header.Set("X-Skip-Auth", "1")
	resp3, err := app.Test(req3, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp3.Body)
		t.Fatalf("countOnly status %d: %s", resp3.StatusCode, b)
	}
}

func TestBulkSuspendRooms_EmptyIDs(t *testing.T) {
	app, _, _, _ := setupRoomGapsApp(t)
	body, _ := json.Marshal(map[string][]string{"ids": {}})
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/bulk/suspend", bytes.NewReader(body))
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

func TestBulkSuspendRooms_Success(t *testing.T) {
	app, roomRepo, _, _ := setupRoomGapsApp(t)
	room, err := roomRepo.CreateRoom("creator-user", "bulk-sus", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string][]string{"ids": {room.ID}})
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/bulk/suspend", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestBulkCloseRooms_Success(t *testing.T) {
	app, roomRepo, _, _ := setupRoomGapsApp(t)
	room, err := roomRepo.CreateRoom("creator-user", "bulk-close", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	body, _ := json.Marshal(map[string][]string{"ids": {room.ID}})
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/bulk/close", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status %d: %s", resp.StatusCode, b)
	}
}

func TestParticipantDisplayName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	roomRepo := repository.NewRoomRepository(db)
	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	h := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, nil, nil, nil, nil, nil)

	if got := h.participantDisplayName(nil); got != "" {
		t.Fatalf("nil claims: %q", got)
	}
	if got := h.participantDisplayName(&auth.Claims{Name: "  Fallback  "}); got != "Fallback" {
		t.Fatalf("no user id: %q", got)
	}
	_ = userRepo.CreateUser(&models.User{ID: "u1", Email: "u1@ex.com", Name: "DB Name", Provider: "local", IsActive: true})
	if got := h.participantDisplayName(&auth.Claims{UserID: "u1", Name: "Claim Name"}); got != "DB Name" {
		t.Fatalf("db name: %q", got)
	}
	if got := h.participantDisplayName(&auth.Claims{UserID: "missing", Name: "Claim Only"}); got != "Claim Only" {
		t.Fatalf("missing user fallback: %q", got)
	}
}

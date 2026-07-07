package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupRoomTestApp builds a Fiber app wired to the RoomHandler with an in-memory DB.
// The LiveKit client is mocked for testing.
func setupRoomTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *auth.Claims) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{
		Host:      "http://localhost:9999",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, settingsRepo, nil, nil, nil)

	claims := &auth.Claims{
		UserID:   "creator-user",
		Email:    "creator@ex.com",
		Name:     "Creator",
		Accesses: []string{"user"},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})

	app.Get("/rooms", handler.ListRooms)
	app.Post("/rooms/guest-join", handler.GuestJoinRoom)
	app.Delete("/rooms/:roomId", handler.DeleteRoom)
	app.Get("/rooms/:roomId/participants", handler.AdminGetRoomParticipants)
	app.Get("/admin/rooms", handler.AdminListRooms)
	app.Put("/admin/rooms/:roomId", handler.AdminUpdateRoom)
	app.Post("/admin/rooms/:roomId/close", handler.AdminCloseRoom)
	app.Get("/online-count", handler.GetOnlineCount)

	// Seed a user so room creation via repo doesn't violate FK
	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	return app, roomRepo, claims
}

// setupAdminRoomTestApp builds a Fiber app with real cleanupSvc wired for
// testing AdminCloseRoom and AdminSuspendRoom endpoints.
func setupAdminRoomTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository) {
	return setupAdminRoomTestAppWithUserRepo(t, nil)
}

func setupAdminRoomTestAppWithUserRepo(t *testing.T, userRepo *repository.UserRepository) (*fiber.App, *repository.RoomRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)
	cleanupSvc := testCleanupSvc(t, roomRepo, uploadTracker)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{
		Host:      "http://localhost:9999",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, nil, settingsRepo, nil, uploadTracker, cleanupSvc)

	claims := &auth.Claims{
		UserID:   "admin-user",
		Email:    "admin@ex.com",
		Name:     "Admin",
		Accesses: []string{"superadmin"},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})

	app.Put("/admin/rooms/:roomId", handler.AdminUpdateRoom)
	app.Post("/admin/rooms/:roomId/close", handler.AdminCloseRoom)
	app.Post("/admin/rooms/:roomId/suspend", handler.AdminSuspendRoom)
	app.Post("/admin/rooms/:roomId/reactivate", handler.AdminReactivateRoom)
	app.Post("/admin/rooms/:roomId/token", handler.AdminGenerateToken)
	app.Get("/admin/livekit/stats", handler.AdminLiveKitStats)
	app.Post("/admin/rooms/:roomId/participants/:identity/kick", handler.AdminKickParticipant)
	app.Post("/admin/rooms/:roomId/participants/:identity/mute", handler.AdminMuteParticipant)
	app.Get("/admin/stats", handler.GetAdminStats)

	db.Create(&models.User{
		ID: "admin-user", Email: "admin@ex.com", Name: "Admin",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"superadmin"},
	})

	return app, roomRepo
}

func TestRoomHandler_ListRooms_Empty(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/rooms", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_ListRooms_WithRooms(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	_, _ = roomRepo.CreateRoom("creator-user", "my-room", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodGet, "/rooms", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var rooms []map[string]interface{}
	_ = json.Unmarshal(body, &rooms)
	if len(rooms) != 1 {
		t.Fatalf("expected 1 room, got %d", len(rooms))
	}
}

func TestRoomHandler_GuestJoinRoom_NotFound(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"roomName":  "nonexistent-room",
		"guestName": "Guest Bob",
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
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

func TestRoomHandler_GuestJoinRoom_EmptyName(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"roomName":  "some-room",
		"guestName": "   ", // whitespace only
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
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

func TestGuestJoinRoom_NullByteName(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"roomName":  "some-room",
		"guestName": "\x00",
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for null byte name, got %d", resp.StatusCode)
	}
}

func TestGuestJoinRoom_ControlCharsOnly(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"roomName":  "some-room",
		"guestName": "\x00\x1b\x07",
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for control chars only, got %d", resp.StatusCode)
	}
}

func TestGuestJoinRoom_NullByteInName(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]string{
		"roomName":  "nonexistent-room",
		"guestName": "Bob\x00Smith",
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// After sanitization: "BobSmith" (valid). Should pass validation and fail at room lookup with 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 (null byte stripped, name passed validation), got %d", resp.StatusCode)
	}
}

func TestRoomHandler_DeleteRoom_NotFound(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodDelete, "/rooms/nonexistent-room-id", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_DeleteRoom_Forbidden(t *testing.T) {
	_, roomRepo, _ := setupRoomTestApp(t)

	// Creator is "creator-user", but set claims to a different user
	room, _ := roomRepo.CreateRoom("creator-user", "owner-room", false, "standard", 0, &models.RoomSettings{})

	// Swap claims to a different non-superadmin user
	otherClaims := &auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	app2 := fiber.New()
	rr := roomRepo
	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, rr, nil, nil, nil, nil, nil, nil)
	app2.Use(func(c *fiber.Ctx) error { c.Locals("user", otherClaims); return c.Next() })
	app2.Delete("/rooms/:roomId", handler.DeleteRoom)

	req := httptest.NewRequest(http.MethodDelete, "/rooms/"+room.ID, http.NoBody)
	resp, _ := app2.Test(req, -1)
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminListRooms_Empty(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if result["rooms"] == nil {
		t.Fatal("expected 'rooms' key in response")
	}
}

func TestRoomHandler_AdminListRooms_WithRooms(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	_, _ = roomRepo.CreateRoom("creator-user", "admin-room-1", true, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodGet, "/admin/rooms", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminListRooms_Pagination(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	// Seed 5 rooms
	for i := range 5 {
		_, _ = roomRepo.CreateRoom("creator-user", fmt.Sprintf("page-room-%d", i), true, "standard", 0, &models.RoomSettings{})
	}

	t.Run("page=0 defaults to 1", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms?page=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("expected 200, got %d", resp.StatusCode)
		}
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		page, _ := body["page"].(float64)
		if page != 1 {
			t.Fatalf("expected page 1, got %f", page)
		}
	})

	t.Run("limit=0 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms?limit=0", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		rooms, _ := body["rooms"].([]any)
		if len(rooms) != 5 {
			t.Fatalf("expected 5 rooms with limit=0, got %d", len(rooms))
		}
	})

	t.Run("limit=-1 defaults to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms?limit=-1", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		rooms, _ := body["rooms"].([]any)
		if len(rooms) != 5 {
			t.Fatalf("expected 5 rooms with limit=-1, got %d", len(rooms))
		}
	})

	t.Run("limit > 100 clamped to 50", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms?limit=200", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		l, _ := body["limit"].(float64)
		if l != 50 {
			t.Fatalf("expected limit clamped to 50, got %f", l)
		}
	})

	t.Run("page > total returns empty", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/admin/rooms?page=999", http.NoBody)
		resp, err := app.Test(req, -1)
		if err != nil {
			t.Fatalf("app.Test failed: %v", err)
		}
		defer func() { _ = resp.Body.Close() }()
		var body map[string]any
		if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		rooms, _ := body["rooms"].([]any)
		if len(rooms) != 0 {
			t.Fatalf("expected empty rooms for page beyond total, got %d", len(rooms))
		}
	})
}

func TestRoomHandler_AdminUpdateRoom_NotFound(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	body, _ := json.Marshal(map[string]int{"maxParticipants": 50})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/nonexistent", bytes.NewReader(body))
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

func TestRoomHandler_AdminUpdateRoom_InvalidBody(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "upd-room", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader([]byte("{invalid")))
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

func TestRoomHandler_AdminUpdateRoom_Success(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "update-me", false, "standard", 0, &models.RoomSettings{})

	maxP := 75
	body, _ := json.Marshal(map[string]int{"maxParticipants": maxP})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminCloseRoom_NotFound(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/close", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_GetOnlineCount(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/online-count", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	_ = json.Unmarshal(body, &result)
	if result["count"] == nil {
		t.Fatal("expected 'count' in response")
	}
}

func TestRoomHandler_AdminGetRoomParticipants_NotFound(t *testing.T) {
	app, _, _ := setupRoomTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/rooms/nonexistent/participants", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminGetRoomParticipants_LiveKitUnavailable(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "part-room", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodGet, "/rooms/"+room.ID+"/participants", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// LiveKit is unavailable → returns empty participants list with 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestAdminSuspendRoom_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAdminSuspendRoom_Success(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "suspend-me", true, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestAdminSuspendRoom_AlreadyInactive(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "already-suspended", true, "standard", 0, &models.RoomSettings{})
	room.IsActive = false
	roomRepo.UpdateRoom(room)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 (already inactive), got %d", resp.StatusCode)
	}
}

func TestAdminReactivateRoom_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAdminReactivateRoom_Success(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "reactivate-me", true, "standard", 0, &models.RoomSettings{})
	room.IsActive = false
	roomRepo.UpdateRoom(room)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Verify room is active again
	updated, _ := roomRepo.GetRoom(room.ID)
	if updated == nil || !updated.IsActive {
		t.Fatal("expected room to be active after reactivation")
	}
}

func TestAdminReactivateRoom_AlreadyActive(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "already-active", true, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 (already active), got %d", resp.StatusCode)
	}
}

func TestAdminGenerateToken_NotImplemented(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "token-test", true, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/token", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("expected 501 (not implemented), got %d", resp.StatusCode)
	}
}

func TestAdminLiveKitStats_Success(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/admin/livekit/stats", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}
}

func TestAdminKickParticipant_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/participants/victim/kick", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAdminMuteParticipant_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/participants/victim/mute", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// setupJoinTestApp creates a minimal Fiber app with JoinRoom and GuestJoinRoom wired
// and the authenticated user set to the given claims.
func setupJoinTestApp(t *testing.T, claims *auth.Claims) (*fiber.App, *repository.RoomRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{
		Host:      "http://localhost:9999",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/rooms/join", handler.JoinRoom)
	app.Post("/rooms/guest-join", handler.GuestJoinRoom)

	// Seed the user record to satisfy FK constraints
	db.Create(&models.User{
		ID: claims.UserID, Email: claims.Email, Name: claims.Name,
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	})

	return app, roomRepo
}

func TestJoinRoom_BannedUserRejected(t *testing.T) {
	bannedClaims := &auth.Claims{
		UserID:   "banned-user",
		Email:    "banned@ex.com",
		Name:     "Banned",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, bannedClaims)

	// Create a room (seeded as a different creator so FK passes)
	room, err := roomRepo.CreateRoom("banned-user", "ban-test-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Ban the user by marking their participant record
	if err := roomRepo.KickParticipant(room.ID, "banned-user"); err != nil {
		t.Fatalf("failed to ban user: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"roomName": "ban-test-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for banned user, got %d", resp.StatusCode)
	}
}

func TestJoinRoom_NotBannedAllowed(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "normal-user",
		Email:    "normal@ex.com",
		Name:     "Normal",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	_, err := roomRepo.CreateRoom("normal-user", "open-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"roomName": "open-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// LiveKit token generation still works (no live server needed for signing)
	// so this should succeed with 200
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for non-banned user, got %d", resp.StatusCode)
	}
}

// setupModTestApp builds a Fiber app with moderation endpoints wired and
// an adjustable-claims middleware, returning the app, roomRepo, and a pointer
// that callers can swap to change who is "logged in".
func setupModTestApp(t *testing.T, claims *auth.Claims) (*fiber.App, *repository.RoomRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{
		Host:      "http://localhost:9999",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	// Wire the endpoints that use the "moderator" check
	app.Post("/rooms/:roomId/participants/:identity/block-chat", handler.BlockChat)
	app.Post("/rooms/:roomId/participants/:identity/deafen", handler.DeafenParticipant)
	app.Post("/rooms/:roomId/participants/:identity/spotlight", handler.SpotlightParticipant)

	// Seed user records to satisfy FK constraints
	db.Create(&models.User{ID: "owner-user", Email: "owner@ex.com", Name: "Owner", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	db.Create(&models.User{ID: "mod-user", Email: "mod@ex.com", Name: "Mod", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	db.Create(&models.User{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	return app, roomRepo
}

// TestModeratorCannotActInOtherRoom verifies that a JWT "moderator" claim does NOT
// grant moderation rights in a room where the user has not been promoted.
// Before the fix this returns 200 (wrong); after the fix it must return 403.
func TestModeratorCannotActInOtherRoom(t *testing.T) {
	// modUserClaims has the "moderator" JWT claim but is NOT the room owner
	// and has NOT been promoted in roomB via the DB.
	modClaims := &auth.Claims{
		UserID:   "mod-user",
		Email:    "mod@ex.com",
		Name:     "Mod",
		Accesses: []string{"user", "moderator"}, // global JWT claim
	}
	app, roomRepo := setupModTestApp(t, modClaims)

	// Create roomB owned by "owner-user".
	roomB, err := roomRepo.CreateRoom("owner-user", "room-b", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create roomB: %v", err)
	}

	// mod-user tries to spotlight a participant in roomB (a room they don't own
	// and haven't been promoted in).
	req := httptest.NewRequest(http.MethodPost, "/rooms/"+roomB.ID+"/participants/some-victim/spotlight", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// After the fix: must be 403. Before the fix: would be non-403 (LiveKit call
	// fails with 500 because the server is fake, but the auth check passes first).
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 (mod in other room), got %d — global moderator claim must not grant cross-room access", resp.StatusCode)
	}
}

// TestRoomModeratorCanActInOwnRoom verifies that a user promoted via the DB
// (is_moderator=true in room_participants) CAN moderate their assigned room.
func TestRoomModeratorCanActInOwnRoom(t *testing.T) {
	modClaims := &auth.Claims{
		UserID:   "mod-user",
		Email:    "mod@ex.com",
		Name:     "Mod",
		Accesses: []string{"user"}, // no global moderator JWT claim
	}
	app, roomRepo := setupModTestApp(t, modClaims)

	// Create roomA owned by "owner-user".
	roomA, err := roomRepo.CreateRoom("owner-user", "room-a", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create roomA: %v", err)
	}

	// Add mod-user as a participant and promote them.
	if err := roomRepo.AddParticipant(roomA.ID, "mod-user"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}
	if err := roomRepo.SetRoomModerator(roomA.ID, "mod-user", true); err != nil {
		t.Fatalf("failed to promote moderator: %v", err)
	}

	// mod-user tries to spotlight someone in roomA — should pass the auth gate.
	// The LiveKit call will fail (fake server), but we'll see 500 NOT 403.
	req := httptest.NewRequest(http.MethodPost, "/rooms/"+roomA.ID+"/participants/some-user/spotlight", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Auth passes → LiveKit call is attempted → fake server → 500 (not 403)
	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("room-scoped moderator should be allowed to act in their room, got 403")
	}
}

func TestGuestJoinRoom_PrivateRoomBlocked(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "room-creator",
		Email:    "creator@ex.com",
		Name:     "Creator",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	_, err := roomRepo.CreateRoom("room-creator", "private-room", false /* not public */, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	body, _ := json.Marshal(map[string]string{"roomName": "private-room", "guestName": "Visitor"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for guest joining private room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminUpdateRoom_SettingsIsPersistent(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "persist-admin-room", false, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]interface{}{
		"settings": map[string]interface{}{
			"allowChat":    true,
			"allowVideo":   true,
			"allowAudio":   true,
			"e2ee":         false,
			"isPersistent": true,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)
	settings := result["settings"].(map[string]interface{})
	if settings["isPersistent"] != true {
		t.Fatalf("expected isPersistent true, got %v", settings["isPersistent"])
	}
}

func TestRoomHandler_AdminUpdateRoom_SettingsIsPersistentOff(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "unpersist-admin-room", false, "standard", 0, &models.RoomSettings{
		IsPersistent: true,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"settings": map[string]interface{}{
			"allowChat":    true,
			"allowVideo":   true,
			"allowAudio":   true,
			"e2ee":         false,
			"isPersistent": false,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)
	settings := result["settings"].(map[string]interface{})
	if settings["isPersistent"] != false {
		t.Fatalf("expected isPersistent false, got %v", settings["isPersistent"])
	}
}

func TestRoomHandler_CreateRoom_StripsIsPersistent(t *testing.T) {
	_, roomRepo, _ := setupRoomTestApp(t)

	// Verify that direct repo create preserves the flag
	room, err := roomRepo.CreateRoom("creator-user", "create-persist-direct", false, "standard", 0, &models.RoomSettings{
		AllowChat:    true,
		AllowAudio:   true,
		AllowVideo:   true,
		IsPersistent: true,
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	found, _ := roomRepo.GetRoom(room.ID)
	if !found.Settings.IsPersistent {
		t.Fatal("repo CreateRoom should preserve isPersistent when set directly")
	}

	room2, _ := roomRepo.CreateRoom("creator-user", "create-no-persist", false, "standard", 0, &models.RoomSettings{})
	found2, _ := roomRepo.GetRoom(room2.ID)
	if found2.Settings.IsPersistent {
		t.Fatal("room created without isPersistent should default to false")
	}
}

func TestRoomHandler_UpdateSettings_StripsIsPersistent(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

	claims := &auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Name: "Creator", Accesses: []string{"user"}}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", claims); return c.Next() })
	app.Put("/room/:roomId/settings", handler.UpdateSettings)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	room, _ := roomRepo.CreateRoom("creator-user", "strip-test-room", false, "standard", 0, &models.RoomSettings{
		AllowChat:    true,
		AllowAudio:   true,
		AllowVideo:   true,
		IsPersistent: false,
	})

	// Room owner tries to set isPersistent via UpdateSettings — should be stripped
	body, _ := json.Marshal(map[string]interface{}{
		"settings": map[string]interface{}{
			"allowChat":    true,
			"allowAudio":   true,
			"allowVideo":   true,
			"isPersistent": true,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/room/"+room.ID+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)
	settings := result["settings"].(map[string]interface{})
	if settings["isPersistent"] == true {
		t.Fatal("UpdateSettings should strip isPersistent — room owner should not be able to set this flag")
	}
}

func TestRoomHandler_AdminUpdateRoom_PartialSettingsMerge(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "partial-merge-room", false, "standard", 0, &models.RoomSettings{
		AllowChat:       true,
		AllowVideo:      true,
		AllowAudio:      false,
		RequireApproval: true,
		E2EE:            false,
		IsPersistent:    false,
	})

	body, _ := json.Marshal(map[string]interface{}{
		"settings": map[string]interface{}{
			"isPersistent": true,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)
	settings := result["settings"].(map[string]interface{})

	if settings["isPersistent"] != true {
		t.Fatalf("expected isPersistent true, got %v", settings["isPersistent"])
	}
	if settings["allowChat"] != true {
		t.Fatalf("expected allowChat true (preserved), got %v", settings["allowChat"])
	}
	if settings["allowVideo"] != true {
		t.Fatalf("expected allowVideo true (preserved), got %v", settings["allowVideo"])
	}
	if settings["allowAudio"] != false {
		t.Fatalf("expected allowAudio false (preserved), got %v", settings["allowAudio"])
	}
	if settings["requireApproval"] != true {
		t.Fatalf("expected requireApproval true (preserved), got %v", settings["requireApproval"])
	}
	if settings["e2ee"] != false {
		t.Fatalf("expected e2ee false (preserved), got %v", settings["e2ee"])
	}
}

func TestRoomHandler_AdminUpdateRoom_SettingsAndMaxParticipants(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "both-update-room", false, "standard", 0, &models.RoomSettings{
		AllowChat:    false,
		AllowVideo:   false,
		AllowAudio:   false,
		IsPersistent: false,
	})

	maxP := 42
	body, _ := json.Marshal(map[string]interface{}{
		"maxParticipants": maxP,
		"settings": map[string]interface{}{
			"allowChat":    true,
			"allowVideo":   true,
			"allowAudio":   true,
			"isPersistent": true,
		},
	})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result map[string]interface{}
	bodyBytes, _ := io.ReadAll(resp.Body)
	_ = json.Unmarshal(bodyBytes, &result)

	settings := result["settings"].(map[string]interface{})
	if settings["isPersistent"] != true {
		t.Fatalf("expected isPersistent true, got %v", settings["isPersistent"])
	}
	if settings["allowChat"] != true {
		t.Fatalf("expected allowChat true, got %v", settings["allowChat"])
	}

	maxPResult := result["maxParticipants"]
	if maxPResult != float64(maxP) {
		t.Fatalf("expected maxParticipants %d, got %v", maxP, maxPResult)
	}
}

func TestRoomHandler_AdminSuspendRoom_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminSuspendRoom_AlreadyInactive(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "suspend-twice", false, "standard", 0, &models.RoomSettings{})

	// Manually deactivate room (suspend endpoint is now async)
	if err := roomRepo.UpdateRoom(&models.Room{ID: room.ID, IsActive: false}); err != nil {
		t.Fatal(err)
	}

	// Suspend should fail — room is already inactive
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for already inactive room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminSuspendRoom_Success(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "suspend-success", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestRoomHandler_AdminCloseRoom_Success(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "close-success", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/close", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

func TestRoomHandler_AdminCloseRoom_SuspendedRoom(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "close-suspended", false, "standard", 0, &models.RoomSettings{})

	// Close regardless of active state — now async
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/close", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusAccepted {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, string(bodyBytes))
	}
}

// ====== Phase 1: JoinRoom suspended room rejection ======

func TestRoomHandler_JoinRoom_SuspendedRoomRejected(t *testing.T) {
	claims := &auth.Claims{
		UserID: "susp-test-user", Email: "susp@ex.com", Name: "Susp",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	// Room created by a different user — susp-test-user is NOT the creator
	room, _ := roomRepo.CreateRoom("owner", "susp-room", true, "standard", 0, &models.RoomSettings{})
	roomRepo.SetRoomIdle(room.ID)

	body, _ := json.Marshal(map[string]string{"roomName": "susp-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410 for suspended room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_JoinRoom_ActiveRoomOk(t *testing.T) {
	claims := &auth.Claims{
		UserID: "active-test-user", Email: "active@ex.com", Name: "Active",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)
	_, _ = roomRepo.CreateRoom("active-test-user", "active-room", true, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]string{"roomName": "active-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for active room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminReactivateRoom_Success(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	// Create room, then manually suspend it via repo (suspend endpoint is now async)
	room, _ := roomRepo.CreateRoom("admin-user", "reactivate-me", false, "standard", 0, &models.RoomSettings{})
	if err := roomRepo.UpdateRoom(&models.Room{ID: room.ID, IsActive: false}); err != nil {
		t.Fatal(err)
	}

	// Reactivate
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200 on reactivate, got %d: %s", resp.StatusCode, string(bodyBytes))
	}

	updated, _ := roomRepo.GetRoom(room.ID)
	if updated == nil || !updated.IsActive {
		t.Fatal("room should be active after reactivation")
	}
}

func TestRoomHandler_AdminReactivateRoom_AlreadyActive(t *testing.T) {
	app, roomRepo := setupAdminRoomTestApp(t)

	room, _ := roomRepo.CreateRoom("admin-user", "already-active", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for already active room, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminReactivateRoom_NotFound(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/nonexistent/reactivate", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

// ====== Phase 6: Input validation + edge case tests ======

func TestRoomHandler_AdminUpdateRoom_NegativeMaxParticipants(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)
	room, _ := roomRepo.CreateRoom("creator-user", "negative-test", false, "standard", 0, &models.RoomSettings{})
	body, _ := json.Marshal(map[string]int{"maxParticipants": -5})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
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

func TestRoomHandler_AdminUpdateRoom_ZeroMaxParticipants(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)
	room, _ := roomRepo.CreateRoom("creator-user", "zero-test", false, "standard", 0, &models.RoomSettings{})
	body, _ := json.Marshal(map[string]int{"maxParticipants": 0})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
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

func TestRoomHandler_AdminUpdateRoom_OverMaxParticipants(t *testing.T) {
	app, roomRepo, _ := setupRoomTestApp(t)
	room, _ := roomRepo.CreateRoom("creator-user", "over-max-test", false, "standard", 0, &models.RoomSettings{})
	body, _ := json.Marshal(map[string]int{"maxParticipants": 1001})
	req := httptest.NewRequest(http.MethodPut, "/admin/rooms/"+room.ID, bytes.NewReader(body))
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

func TestRoomHandler_AdminCloseRoom_PathTraversal(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/../../etc/passwd/close", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// DB lookup by ID, not filesystem — should be 404
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for path traversal, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminSuspendRoom_EmptyRoomID(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms//suspend", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	// Fiber's route matching: empty param → 404
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRoomHandler_AdminCloseRoom_EmptyRoomID(t *testing.T) {
	app, _ := setupAdminRoomTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms//close", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetAdminStats_Success(t *testing.T) {
	db := testutil.SetupTestDB(t)
	userRepo := repository.NewUserRepository(db)
	app, roomRepo := setupAdminRoomTestAppWithUserRepo(t, userRepo)

	userRepo.CreateUser(&models.User{ID: "stat-u1", Email: "s1@ex.com", Name: "S1", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	userRepo.CreateUser(&models.User{ID: "stat-u2", Email: "s2@ex.com", Name: "S2", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	roomRepo.CreateRoom("stat-u1", "stat-room-1", true, "standard", 0, &models.RoomSettings{})
	roomRepo.CreateRoom("stat-u2", "stat-room-2", false, "standard", 0, &models.RoomSettings{})

	req := httptest.NewRequest(http.MethodGet, "/admin/stats", http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["totalRooms"] == nil || result["totalUsers"] == nil {
		t.Fatal("expected totalRooms and totalUsers in stats response")
	}
}

func TestResolveLiveKitHost(t *testing.T) {
	t.Parallel()

	serverHost := "wss://example.com/livekit"

	tests := []struct {
		name   string
		host   string
		origin string
		want   string
	}{
		{name: "server wss default", host: serverHost, want: serverHost},
		{name: "localhost still uses server", host: serverHost, origin: "http://localhost:7070", want: serverHost},
		{name: "https upgrades ws", host: "ws://example.com/livekit", origin: "https://example.com", want: "wss://example.com/livekit"},
		{name: "http public unchanged", host: "ws://example.com/livekit", origin: "http://example.com", want: "ws://example.com/livekit"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New()
			var got string
			app.Get("/", func(c *fiber.Ctx) error {
				if tc.origin != "" {
					c.Request().Header.Set("Origin", tc.origin)
				}
				got = resolveLiveKitHost(c, tc.host)
				return c.SendStatus(fiber.StatusOK)
			})
			resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/", http.NoBody), -1)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}

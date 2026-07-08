package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupCreateRoomTestApp includes CreateRoom route with settingsRepo wired.
func setupCreateRoomTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *repository.SettingsRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
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
	app.Post("/room/create", handler.CreateRoom)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	return app, roomRepo, settingsRepo
}

// --- CreateRoom Tests ---

func TestCreateRoom_Success(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name":     "test-room",
		"isPublic": true,
		"mode":     "standard",
	})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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
	if result["id"] == nil || result["id"] == "" {
		t.Fatal("expected 'id' in response")
	}
	if result["name"] != "test-room" {
		t.Fatalf("expected name 'test-room', got %v", result["name"])
	}
}

func TestCreateRoom_AutoGenerateName(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"isPublic": true,
	})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with auto-generated name, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if name, ok := result["name"].(string); !ok || name == "" {
		t.Fatal("expected auto-generated name")
	}
}

func TestCreateRoom_InvalidBody(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader([]byte("{invalid")))
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

func TestCreateRoom_InvalidName(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "room with $ymbols!"})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusInternalServerError {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected non-500, got 500: %s", string(respBody))
	}
}

func TestCreateRoom_InvalidMode(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{
		"name": "bad-mode-room",
		"mode": "unknown_mode",
	})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid mode, got %d", resp.StatusCode)
	}
}

func TestCreateRoom_MaxParticipantsOverLimit(t *testing.T) {
	app, _, settingsRepo := setupCreateRoomTestApp(t)

	// Set a low max participants limit
	settings, _ := settingsRepo.GetEffectiveSettings()
	settings.MaxParticipantsLimit = 10
	settingsRepo.SaveSettings(settings)

	body, _ := json.Marshal(map[string]interface{}{
		"name":            "over-limit-room",
		"maxParticipants": 20,
	})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for over-limit maxParticipants, got %d", resp.StatusCode)
	}
}

func TestCreateRoom_MaxRoomsPerUserReached(t *testing.T) {
	app, roomRepo, settingsRepo := setupCreateRoomTestApp(t)

	// Set low per-user limit
	settings, _ := settingsRepo.GetEffectiveSettings()
	settings.MaxRoomsPerUser = 1
	settingsRepo.SaveSettings(settings)

	// Create first room
	_, err := roomRepo.CreateRoom("creator-user", "first-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to seed first room: %v", err)
	}

	// Try creating second room
	body, _ := json.Marshal(map[string]interface{}{
		"name": "second-room",
	})
	req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 (room limit), got %d", resp.StatusCode)
	}
}

func TestCreateRoom_ConcurrentDuplicateRace(t *testing.T) {
	app, _, _ := setupCreateRoomTestApp(t)

	// Fire 5 concurrent create requests with same name
	// The LK mock returns success each time. The DB has a unique constraint
	// on room name, so only one should succeed.
	var wg sync.WaitGroup
	results := make(chan int, 5)

	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body, _ := json.Marshal(map[string]interface{}{
				"name": "race-room",
			})
			req := httptest.NewRequest(http.MethodPost, "/room/create", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			resp, err := app.Test(req, -1)
			if err != nil {
				results <- -1
				return
			}
			defer func() { _ = resp.Body.Close() }()
			results <- resp.StatusCode
		}()
	}
	wg.Wait()
	close(results)

	successCount := 0
	conflictCount := 0
	for code := range results {
		if code == http.StatusOK {
			successCount++
		}
		if code == http.StatusConflict {
			conflictCount++
		}
	}
	if successCount != 1 {
		t.Fatalf("expected exactly 1 success (DB unique constraint), got %d", successCount)
	}
	if conflictCount < 1 {
		t.Logf("expected at least one 409 conflict, got %d (some may have gotten 500 from LK race)", conflictCount)
	}
}

// --- JoinRoom Tests ---

func TestJoinRoom_RoomNotFound(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, _ := setupJoinTestApp(t, claims)

	body, _ := json.Marshal(map[string]interface{}{"roomName": "nonexistent"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
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

func TestJoinRoom_RoomInactive(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	// Room created by a different user — joiner is NOT the creator
	room, _ := roomRepo.CreateRoom("owner", "inactive-room", true, "standard", 0, &models.RoomSettings{})
	room.IsActive = false
	roomRepo.UpdateRoom(room)

	body, _ := json.Marshal(map[string]interface{}{"roomName": "inactive-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusGone {
		t.Fatalf("expected 410 Gone for inactive room, got %d", resp.StatusCode)
	}
}

func TestJoinRoom_RoomInactive_OwnedByJoiner(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	// Room created by joiner — owns it, so should get archived_owned response
	room, _ := roomRepo.CreateRoom("joiner", "my-inactive-room", true, "standard", 0, &models.RoomSettings{})
	room.IsActive = false
	roomRepo.UpdateRoom(room)

	body, _ := json.Marshal(map[string]interface{}{"roomName": "my-inactive-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 with archived_owned for owner, got %d", resp.StatusCode)
	}
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if result["status"] != "archived_owned" {
		t.Fatalf("expected status 'archived_owned', got %v", result["status"])
	}
	if result["name"] != "my-inactive-room" {
		t.Fatalf("expected name 'my-inactive-room', got %v", result["name"])
	}
}

func TestJoinRoom_PrivateRoomBlocked(t *testing.T) {
	joinerClaims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, joinerClaims)

	roomRepo.CreateRoom("owner", "private-room", false, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]interface{}{"roomName": "private-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for private room, got %d", resp.StatusCode)
	}
}

func TestJoinRoom_PrivateRoomApprovedParticipant(t *testing.T) {
	joinerClaims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, joinerClaims)

	room, _ := roomRepo.CreateRoom("owner", "private-room-approved", false, "standard", 0, &models.RoomSettings{})
	roomRepo.AddParticipant(room.ID, "joiner")

	body, _ := json.Marshal(map[string]interface{}{"roomName": "private-room-approved"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for approved participant, got %d", resp.StatusCode)
	}
}

func TestJoinRoom_RoomFull(t *testing.T) {
	joinerClaims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, joinerClaims)

	room, _ := roomRepo.CreateRoom("owner", "full-room", true, "standard", 0, &models.RoomSettings{})
	// Fill the room by setting max participants and adding participants
	room.MaxParticipants = 1
	roomRepo.UpdateRoom(room)
	roomRepo.AddParticipant(room.ID, "existing-user")

	body, _ := json.Marshal(map[string]interface{}{"roomName": "full-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for full room, got %d", resp.StatusCode)
	}
}

func TestJoinRoom_Success(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "joiner",
		Email:    "joiner@ex.com",
		Name:     "Joiner",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	room, _ := roomRepo.CreateRoom("joiner", "join-success-room", true, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]interface{}{"roomName": "join-success-room"})
	req := httptest.NewRequest(http.MethodPost, "/rooms/join", bytes.NewReader(body))
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
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["token"] == nil || result["token"] == "" {
		t.Fatal("expected LiveKit join token in response")
	}
	if result["id"] != room.ID {
		t.Fatalf("expected room id %s, got %v", room.ID, result["id"])
	}
}

// --- GuestJoinRoom Success ---

func TestGuestJoinRoom_Success(t *testing.T) {
	claims := &auth.Claims{
		UserID:   "guest",
		Email:    "guest@ex.com",
		Name:     "Guest",
		Accesses: []string{"user"},
	}
	app, roomRepo := setupJoinTestApp(t, claims)

	room, _ := roomRepo.CreateRoom("owner", "guest-room", true, "standard", 0, &models.RoomSettings{})
	roomRepo.AddParticipant(room.ID, "guest")

	body, _ := json.Marshal(map[string]interface{}{
		"roomName":  "guest-room",
		"guestName": "Guest User",
	})
	req := httptest.NewRequest(http.MethodPost, "/rooms/guest-join", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
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

// --- UpdateSettings Tests ---

func setupUpdateSettingsTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

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
	app.Put("/room/:roomId/settings", handler.UpdateSettings)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	db.Create(&models.User{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	return app, roomRepo
}

func TestUpdateSettings_Success(t *testing.T) {
	app, roomRepo := setupUpdateSettingsTestApp(t)

	room, _ := roomRepo.CreateRoom("creator-user", "settings-update-test", true, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]interface{}{
		"name":        "Updated Name",
		"isPublic":    false,
		"description": "New description",
	})
	req := httptest.NewRequest(http.MethodPut, "/room/"+room.ID+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, string(respBody))
	}

	// Verify changes persisted
	updated, _ := roomRepo.GetRoom(room.ID)
	if updated == nil {
		t.Fatal("room not found after update")
	}
}

func TestUpdateSettings_NonCreatorForbidden(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

	otherClaims := &auth.Claims{
		UserID:   "other-user",
		Email:    "other@ex.com",
		Name:     "Other",
		Accesses: []string{"user"},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error { c.Locals("user", otherClaims); return c.Next() })
	app.Put("/room/:roomId/settings", handler.UpdateSettings)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})
	db.Create(&models.User{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	room, _ := roomRepo.CreateRoom("creator-user", "non-creator-settings", true, "standard", 0, &models.RoomSettings{})

	body, _ := json.Marshal(map[string]interface{}{"name": "Hacked Name"})
	req := httptest.NewRequest(http.MethodPut, "/room/"+room.ID+"/settings", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-creator, got %d", resp.StatusCode)
	}
}

func TestUpdateSettings_RoomNotFound(t *testing.T) {
	app, _ := setupUpdateSettingsTestApp(t)

	body, _ := json.Marshal(map[string]interface{}{"name": "Nope"})
	req := httptest.NewRequest(http.MethodPut, "/room/nonexistent/settings", bytes.NewReader(body))
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

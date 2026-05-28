package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
)

// mockEgressClient implements livekit.Egress for testing.
type mockEgressClient struct {
	startErr error
	stopErr  error
}

func (m *mockEgressClient) StartRoomCompositeEgress(_ context.Context, req *livekit.RoomCompositeEgressRequest) (*livekit.EgressInfo, error) {
	if m.startErr != nil {
		return nil, m.startErr
	}
	return &livekit.EgressInfo{
		EgressId: uuid.NewString(),
		RoomName: req.RoomName,
		Status:   livekit.EgressStatus_EGRESS_STARTING,
	}, nil
}

func (m *mockEgressClient) StartWebEgress(_ context.Context, _ *livekit.WebEgressRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) StartParticipantEgress(_ context.Context, _ *livekit.ParticipantEgressRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) StartTrackCompositeEgress(_ context.Context, _ *livekit.TrackCompositeEgressRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) StartTrackEgress(_ context.Context, _ *livekit.TrackEgressRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) UpdateLayout(_ context.Context, _ *livekit.UpdateLayoutRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) UpdateStream(_ context.Context, _ *livekit.UpdateStreamRequest) (*livekit.EgressInfo, error) {
	return nil, nil
}

func (m *mockEgressClient) ListEgress(_ context.Context, _ *livekit.ListEgressRequest) (*livekit.ListEgressResponse, error) {
	return nil, nil
}

func (m *mockEgressClient) StopEgress(_ context.Context, req *livekit.StopEgressRequest) (*livekit.EgressInfo, error) {
	if m.stopErr != nil {
		return nil, m.stopErr
	}
	return &livekit.EgressInfo{
		EgressId: req.EgressId,
		Status:   livekit.EgressStatus_EGRESS_ENDING,
	}, nil
}

// setupRecordingTestApp creates a test app with a "mod-user" with "moderator" access.
func setupRecordingTestApp(t *testing.T, mockEgress livekit.Egress) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository) {
	t.Helper()
	return setupRecordingTestAppAs(t, mockEgress, "mod-user", "moderator")
}

// setupRecordingTestAppAs creates a test app with the given user parameters.
func setupRecordingTestAppAs(t *testing.T, mockEgress livekit.Egress, userID string, accesses ...string) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "test-key", "test-secret-1234567890123456")

	settings := &models.SystemSettings{
		RecordingsEnabled:   true,
		RegistrationEnabled: true,
	}
	_ = settingsRepo.SaveSettings(settings)

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   userID,
			Email:    userID + "@example.com",
			Name:     userID,
			Accesses: accesses,
		})
		c.Locals("userID", userID)
		return c.Next()
	})

	app.Post("/api/rooms/:id/recording/start", handler.StartRecording)
	app.Post("/api/rooms/:id/recording/stop", handler.StopRecording)
	app.Get("/api/rooms/:id/recordings", handler.ListRecordings)
	app.Get("/api/rooms/:id/recordings/:rid", handler.GetRecording)

	return app, roomRepo, recordingRepo
}

func createTestRoom(t *testing.T, roomRepo *repository.RoomRepository) *models.Room {
	t.Helper()
	room, err := roomRepo.CreateRoom("mod-user", "test-room-"+uuid.NewString()[:8], false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create test room: %v", err)
	}
	return room
}

// setupRecordingTestAppWithMiddleware builds an app that includes RecordingsEnabled middleware.
func setupRecordingTestAppWithMiddleware(t *testing.T, mockEgress livekit.Egress, settingsRepo *repository.SettingsRepository) (*fiber.App, *repository.RoomRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "test-key", "test-secret-1234567890123456")

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "mod-user",
			Email:    "mod-user@example.com",
			Name:     "mod-user",
			Accesses: []string{"moderator"},
		})
		c.Locals("userID", "mod-user")
		return c.Next()
	})

	app.Post("/api/rooms/:id/recording/start", middleware.RecordingsEnabled(settingsRepo), handler.StartRecording)
	app.Post("/api/rooms/:id/recording/stop", middleware.RecordingsEnabled(settingsRepo), handler.StopRecording)
	return app, roomRepo
}

func recordingTestSkipped(t *testing.T) {
	t.Skip("TODO oncoming feature")
}

func TestRecordingHandler_RecordingsDisabled_Returns403(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	settings := &models.SystemSettings{RecordingsEnabled: false}
	_ = settingsRepo.SaveSettings(settings)

	app, roomRepo := setupRecordingTestAppWithMiddleware(t, &mockEgressClient{}, settingsRepo)
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when recordings disabled, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/stop", nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 when recordings disabled, got %d", resp2.StatusCode)
	}
}

func TestRecordingHandler_Start_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["id"] == nil || body["id"] == "" {
		t.Fatal("expected non-empty recording ID")
	}
	if body["status"] != "started" {
		t.Fatalf("expected status 'started', got %v", body["status"])
	}
}

func TestRecordingHandler_Start_AlreadyActive(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for first start, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate start, got %d", resp2.StatusCode)
	}
}

func TestRecordingHandler_Stop_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for start, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/stop", nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "processing" {
		t.Fatalf("expected status 'processing', got %v", body["status"])
	}
}

func TestRecordingHandler_Stop_NoActive(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/stop", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 when no active recording, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Start_Denied_NonModerator(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestAppAs(t, &mockEgressClient{}, "stranger", "user")
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-moderator, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_List_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for start, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	recordings, ok := body["recordings"].([]any)
	if !ok || len(recordings) == 0 {
		t.Fatal("expected at least one recording")
	}
	if body["total"].(float64) < 1 {
		t.Fatalf("expected total >= 1, got %v", body["total"])
	}
}

func TestRecordingHandler_List_NonParticipant(t *testing.T) {
	recordingTestSkipped(t)
	_, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	appStranger, _, _ := setupRecordingTestAppAs(t, &mockEgressClient{}, "stranger", "user")
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := appStranger.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for non-participant, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_List_Empty(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	recordings, ok := body["recordings"].([]any)
	if !ok || len(recordings) != 0 {
		t.Fatal("expected empty recordings list")
	}
	if body["total"].(float64) != 0 {
		t.Fatalf("expected total 0, got %v", body["total"])
	}
}

func TestRecordingHandler_Get_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for start, got %d", resp.StatusCode)
	}

	var startBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startBody); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}
	recordingID, ok := startBody["id"].(string)
	if !ok || recordingID == "" {
		t.Fatal("expected recording ID from start")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings/"+recordingID, nil)
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var getBody map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&getBody); err != nil {
		t.Fatalf("failed to decode get response: %v", err)
	}
	if getBody["id"] != recordingID {
		t.Fatalf("expected recording ID %s, got %v", recordingID, getBody["id"])
	}
}

func TestRecordingHandler_Get_NonParticipant(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for start, got %d", resp.StatusCode)
	}

	var startBody map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&startBody); err != nil {
		t.Fatalf("failed to decode start response: %v", err)
	}
	recordingID, _ := startBody["id"].(string)

	appStranger, _, _ := setupRecordingTestAppAs(t, &mockEgressClient{}, "stranger", "user")
	req2 := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings/"+recordingID, nil)
	resp2, _ := appStranger.Test(req2, -1)
	if resp2.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for non-participant, got %d", resp2.StatusCode)
	}
}

func TestRecordingHandler_Get_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings/nonexistent-id", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent recording, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Start_NonExistentRoom(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+uuid.NewString()+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent room, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Start_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/nonexistent-room/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_OwnerAllowedStart(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for room owner, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_SuperadminBypass(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, _ := setupRecordingTestAppAs(t, &mockEgressClient{}, "admin", "superadmin")
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for superadmin, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Stop_NonExistentRoom(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+uuid.NewString()+"/recording/stop", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent room, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Stop_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodPost, "/api/rooms/nonexistent-room/recording/stop", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

// mockRecordingStore implements storage.RecordingStore for testing Clear endpoints.
type mockRecordingStore struct {
	deleteKeys []string
	deleteErr  error
}

func (m *mockRecordingStore) Delete(_ context.Context, key string) error {
	m.deleteKeys = append(m.deleteKeys, key)
	if m.deleteErr != nil {
		return m.deleteErr
	}
	return nil
}

func (m *mockRecordingStore) Store(_ context.Context, _ string, _ io.Reader, _ int64) (*storage.RecordingAttachment, error) {
	return nil, nil
}

// --- Admin tests ---

func setupAdminRecordingTestApp(t *testing.T, mockEgress livekit.Egress) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository) {
	t.Helper()
	return setupAdminRecordingTestAppWithStore(t, mockEgress, nil)
}

func setupAdminRecordingTestAppWithStore(t *testing.T, mockEgress livekit.Egress, recStore storage.RecordingStore) (*fiber.App, *repository.RoomRepository, *repository.RecordingRepository) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "test-key", "test-secret-1234567890123456")

	settings := &models.SystemSettings{
		RecordingsEnabled:   true,
		RegistrationEnabled: true,
	}
	_ = settingsRepo.SaveSettings(settings)

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, recStore)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "admin-user",
			Email:    "admin@example.com",
			Name:     "Admin",
			Accesses: []string{"superadmin"},
		})
		c.Locals("userID", "admin-user")
		return c.Next()
	})

	app.Get("/api/admin/recordings", handler.AdminListRecordings)
	app.Post("/api/admin/recordings/bulk/delete", handler.BulkDeleteRecordings)
	app.Delete("/api/rooms/:id/recordings", handler.ClearRoomRecordings)
	app.Delete("/api/rooms/:id/recordings/:recordingId", handler.ClearSingleRecording)

	return app, roomRepo, recordingRepo
}

func TestRecordingHandler_AdminList_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, recordingRepo := setupAdminRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	// Create a recording directly in the same DB
	rec := &models.Recording{
		ID:     uuid.NewString(),
		RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite",
		Status:        models.RecordingStarted,
		CreatedBy:     "admin-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	recordings, ok := body["recordings"].([]any)
	if !ok || len(recordings) == 0 {
		t.Fatal("expected at least one recording")
	}
}

func TestRecordingHandler_AdminList_FilterByRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/admin/recordings?roomId=nonexistent-room", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["total"].(float64) != 0 {
		t.Fatalf("expected total 0 for nonexistent room filter, got %v", body["total"])
	}
}

func TestRecordingHandler_AdminList_FilterByStatus(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, recordingRepo := setupAdminRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	// Create a recording directly in the same DB
	rec := &models.Recording{
		ID:     uuid.NewString(),
		RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite",
		Status:        models.RecordingStarted,
		CreatedBy:     "admin-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/admin/recordings?status=started", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	recordings, ok := body["recordings"].([]any)
	if !ok || len(recordings) == 0 {
		t.Fatal("expected at least one recording with status 'started'")
	}
}

func TestRecordingHandler_AdminBulkDelete_Success(t *testing.T) {
	recordingTestSkipped(t)
	app, roomRepo, recordingRepo := setupAdminRecordingTestApp(t, &mockEgressClient{})
	room := createTestRoom(t, roomRepo)

	// Create a recording directly in the same DB
	recordingID := uuid.NewString()
	rec := &models.Recording{
		ID:     recordingID,
		RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite",
		Status:        models.RecordingStarted,
		CreatedBy:     "admin-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	deleteReq := BulkIDsRequest{IDs: []string{recordingID}}
	body, _ := json.Marshal(deleteReq)
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recordings/bulk/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_AdminBulkDelete_EmptyIDs(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestApp(t, &mockEgressClient{})

	body, _ := json.Marshal(BulkIDsRequest{IDs: []string{}})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recordings/bulk/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty IDs, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_AdminBulkDelete_TooMany(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestApp(t, &mockEgressClient{})

	ids := make([]string, 501)
	for i := range ids {
		ids[i] = uuid.NewString()
	}
	body, _ := json.Marshal(BulkIDsRequest{IDs: ids})
	req := httptest.NewRequest(http.MethodPost, "/api/admin/recordings/bulk/delete", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for too many IDs, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_AdminList_Denied_NonAdmin(t *testing.T) {
	recordingTestSkipped(t)
	// Build a non-superadmin app with admin routes
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})
	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID:   "regular-user",
			Email:    "regular@example.com",
			Name:     "Regular",
			Accesses: []string{"user"},
		})
		c.Locals("userID", "regular-user")
		return c.Next()
	})
	app.Get("/api/admin/recordings", handler.AdminListRecordings)
	app.Post("/api/admin/recordings/bulk/delete", handler.BulkDeleteRecordings)

	req := httptest.NewRequest(http.MethodGet, "/api/admin/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin, got %d", resp.StatusCode)
	}

	req2 := httptest.NewRequest(http.MethodPost, "/api/admin/recordings/bulk/delete",
		bytes.NewReader([]byte(`{"ids":["`+uuid.NewString()+`"]}`)))
	req2.Header.Set("Content-Type", "application/json")
	resp2, _ := app.Test(req2, -1)
	if resp2.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-admin bulk delete, got %d", resp2.StatusCode)
	}
}

func TestRecordingHandler_List_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/rooms/invalid-uuid/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_Get_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupRecordingTestApp(t, &mockEgressClient{})

	req := httptest.NewRequest(http.MethodGet, "/api/rooms/invalid-uuid/recordings/some-recording", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

// ── Gap 4: ClearRoomRecordings / ClearSingleRecording tests ────────────────

func TestRecordingHandler_ClearRoomRecordings_Success(t *testing.T) {
	recordingTestSkipped(t)
	mockStore := &mockRecordingStore{}
	app, roomRepo, recordingRepo := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, mockStore)
	room := createTestRoom(t, roomRepo)

	// Create two recordings with files — use unique EgressIDs to avoid UNIQUE constraint
	for i := 0; i < 2; i++ {
		rec := &models.Recording{
			ID: uuid.NewString(), EgressID: uuid.NewString(),
			RoomID: room.ID, RoomName: room.Name,
			RecordingType: "composite",
			Status:        models.RecordingCompleted,
			FileURL:       "https://storage.example.com/recordings/" + uuid.NewString() + ".mp4",
			FileSize:      1024,
			DurationMs:    60000,
			CreatedBy:     "admin-user",
		}
		if err := recordingRepo.Create(rec); err != nil {
			t.Fatalf("failed to create recording: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// DB records deleted
	_, total, _ := recordingRepo.ListByRoomID(room.ID, 0, 10)
	if total != 0 {
		t.Fatalf("expected 0 recordings after clear, got %d", total)
	}

	// Store delete called for both recordings
	if len(mockStore.deleteKeys) != 2 {
		t.Fatalf("expected 2 store Delete calls, got %d", len(mockStore.deleteKeys))
	}
}

func TestRecordingHandler_ClearRoomRecordings_Denied_NonOwner(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	// Create room as "room-creator"
	room, err := roomRepo.CreateRoom("room-creator", "test-room-owner", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "stranger", Email: "stranger@example.com", Name: "Stranger",
			Accesses: []string{"user"},
		})
		c.Locals("userID", "stranger")
		return c.Next()
	})
	app.Delete("/api/rooms/:id/recordings", handler.ClearRoomRecordings)

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_ClearRoomRecordings_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, &mockRecordingStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+uuid.NewString()+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent room, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_ClearRoomRecordings_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, &mockRecordingStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/invalid-uuid/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_ClearSingleRecording_Success(t *testing.T) {
	recordingTestSkipped(t)
	mockStore := &mockRecordingStore{}
	app, roomRepo, recordingRepo := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, mockStore)
	room := createTestRoom(t, roomRepo)

	rec := &models.Recording{
		ID: uuid.NewString(), RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite",
		Status:        models.RecordingCompleted,
		FileURL:       "https://storage.example.com/recordings/rec.mp4",
		FileSize:      2048,
		DurationMs:    30000,
		CreatedBy:     "admin-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings/"+rec.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify deleted from DB
	_, err := recordingRepo.GetByID(rec.ID)
	if err == nil {
		t.Fatal("expected recording to be deleted from DB")
	}

	// Store delete called
	if len(mockStore.deleteKeys) != 1 {
		t.Fatalf("expected 1 store Delete call, got %d", len(mockStore.deleteKeys))
	}
}

func TestRecordingHandler_ClearSingleRecording_Denied_NonOwner(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	room, err := roomRepo.CreateRoom("room-creator", "test-room-owner", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	rec := &models.Recording{
		ID: uuid.NewString(), RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite", Status: models.RecordingCompleted,
		FileURL: "https://storage.example.com/rec.mp4", FileSize: 1024, DurationMs: 60000,
		CreatedBy: "room-creator",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "stranger", Email: "stranger@example.com", Name: "Stranger",
			Accesses: []string{"user"},
		})
		c.Locals("userID", "stranger")
		return c.Next()
	})
	app.Delete("/api/rooms/:id/recordings/:recordingId", handler.ClearSingleRecording)

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings/"+rec.ID, nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_ClearSingleRecording_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	mockStore := &mockRecordingStore{}
	app, roomRepo, _ := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, mockStore)
	room := createTestRoom(t, roomRepo)

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings/nonexistent-recording", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for nonexistent recording, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_ClearSingleRecording_InvalidRoomID(t *testing.T) {
	recordingTestSkipped(t)
	app, _, _ := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, &mockRecordingStore{})

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/invalid-uuid/recordings/some-rec", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid room ID, got %d", resp.StatusCode)
	}
}

// mockFailingStore fails every Delete call after the first success.
type mockFailingStore struct {
	deleteErr error
	callCount int
}

func (m *mockFailingStore) Delete(_ context.Context, _ string) error {
	m.callCount++
	if m.callCount > 1 {
		return io.ErrUnexpectedEOF
	}
	return m.deleteErr
}

func (m *mockFailingStore) Store(_ context.Context, _ string, _ io.Reader, _ int64) (*storage.RecordingAttachment, error) {
	return nil, nil
}

func TestRecordingHandler_ClearRoomRecordings_PartialFileDelete(t *testing.T) {
	recordingTestSkipped(t)
	store := &mockFailingStore{}
	app, roomRepo, recordingRepo := setupAdminRecordingTestAppWithStore(t, &mockEgressClient{}, store)
	room := createTestRoom(t, roomRepo)

	// Create two recordings with files
	for i := 0; i < 2; i++ {
		rec := &models.Recording{
			ID: uuid.NewString(), EgressID: uuid.NewString(),
			RoomID: room.ID, RoomName: room.Name,
			RecordingType: "composite",
			Status:        models.RecordingCompleted,
			FileURL:       "https://storage.example.com/recordings/" + uuid.NewString() + ".mp4",
			FileSize:      1024,
			DurationMs:    60000,
			CreatedBy:     "admin-user",
		}
		if err := recordingRepo.Create(rec); err != nil {
			t.Fatalf("failed to create recording: %v", err)
		}
	}

	req := httptest.NewRequest(http.MethodDelete, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusMultiStatus {
		t.Fatalf("expected 207 for partial file delete failure, got %d", resp.StatusCode)
	}

	// DB records still deleted despite file delete failure
	_, total, _ := recordingRepo.ListByRoomID(room.ID, 0, 10)
	if total != 0 {
		t.Fatalf("expected 0 recordings after clear despite file error, got %d", total)
	}
}

// ── Gap 5: ListRecordings archived/deleted room tests ────────────────────

func TestRecordingHandler_List_DeletedRoom_CreatorWithRecordings(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	room, err := roomRepo.CreateRoom("mod-user", "test-room-archived", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Create a recording as the room creator
	rec := &models.Recording{
		ID: uuid.NewString(), RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite", Status: models.RecordingCompleted,
		FileSize: 1024, DurationMs: 60000, CreatedBy: "mod-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	// Soft-delete (archive) the room
	db.Delete(&models.Room{}, "id = ?", room.ID)

	// Build app as mod-user
	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "mod-user", Email: "mod-user@example.com", Name: "mod-user",
			Accesses: []string{"moderator"},
		})
		c.Locals("userID", "mod-user")
		return c.Next()
	})
	app.Get("/api/rooms/:id/recordings", handler.ListRecordings)

	// Creator should still see their recordings
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for creator of deleted room, got %d", resp.StatusCode)
	}

	var body map[string]any
	json.NewDecoder(resp.Body).Decode(&body)
	if body["total"].(float64) != 1 {
		t.Fatalf("expected 1 recording, got %v", body["total"])
	}
}

func TestRecordingHandler_List_DeletedRoom_NonCreator(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	room, err := roomRepo.CreateRoom("mod-user", "test-room-archived-2", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Create recording by room owner
	rec := &models.Recording{
		ID: uuid.NewString(), RoomID: room.ID, RoomName: room.Name,
		RecordingType: "composite", Status: models.RecordingCompleted,
		FileSize: 1024, DurationMs: 60000, CreatedBy: "mod-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}

	// Soft-delete (archive) the room
	db.Delete(&models.Room{}, "id = ?", room.ID)

	// Build stranger app
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	appStranger := fiber.New()
	appStranger.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "stranger", Email: "stranger@example.com", Name: "Stranger",
			Accesses: []string{"user"},
		})
		c.Locals("userID", "stranger")
		return c.Next()
	})
	appStranger.Get("/api/rooms/:id/recordings", handler.ListRecordings)

	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := appStranger.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for stranger of deleted room, got %d", resp.StatusCode)
	}
}

func TestRecordingHandler_List_DeletedRoom_CreatorNoRecordings(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	room, err := roomRepo.CreateRoom("mod-user", "test-room-no-rec", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Soft-delete (archive) the room — no recordings exist
	db.Delete(&models.Room{}, "id = ?", room.ID)

	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "mod-user", Email: "mod-user@example.com", Name: "mod-user",
			Accesses: []string{"moderator"},
		})
		c.Locals("userID", "mod-user")
		return c.Next()
	})
	app.Get("/api/rooms/:id/recordings", handler.ListRecordings)

	// Creator should get 404 (no recordings to return)
	req := httptest.NewRequest(http.MethodGet, "/api/rooms/"+room.ID+"/recordings", nil)
	resp, _ := app.Test(req, -1)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 for deleted room with no recordings, got %d", resp.StatusCode)
	}
}

// ── Gap 7: StopRecording by different moderator ───────────────────────────

func TestRecordingHandler_Stop_DifferentModerator(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, &mockEgressClient{}, "test-key", "test-secret-1234567890123456")
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true, RegistrationEnabled: true})

	room, err := roomRepo.CreateRoom("owner-user", "test-room-diff-mod", false, "standard", 0, &models.RoomSettings{RecordingsAllowed: true})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Add mod-b as a participant with is_moderator=true
	_ = roomRepo.AddParticipant(room.ID, "mod-b")
	db.Model(&models.RoomParticipant{}).
		Where("room_id = ? AND user_id = ?", room.ID, "mod-b").
		Update("is_moderator", true)

	// Build app as mod-b
	handler := NewRecordingHandler(roomRepo, recordingService, recordingRepo, nil)
	appModB := fiber.New()
	appModB.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{
			UserID: "mod-b", Email: "mod-b@example.com", Name: "ModB",
			Accesses: []string{"moderator"},
		})
		c.Locals("userID", "mod-b")
		return c.Next()
	})
	appModB.Post("/api/rooms/:id/recording/start", handler.StartRecording)
	appModB.Post("/api/rooms/:id/recording/stop", handler.StopRecording)

	// Moderator B starts the recording
	req := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/start", nil)
	resp, _ := appModB.Test(req, -1)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 for mod-b start, got %d", resp.StatusCode)
	}

	// Moderator B can stop it too
	req2 := httptest.NewRequest(http.MethodPost, "/api/rooms/"+room.ID+"/recording/stop", nil)
	resp2, _ := appModB.Test(req2, -1)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for mod-b stopping own recording, got %d", resp.StatusCode)
	}
}

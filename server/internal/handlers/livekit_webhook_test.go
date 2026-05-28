package handlers

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
)

const (
	whTestAPIKey    = "webhook-test-key"
	whTestAPISecret = "webhook-test-secret"
)

func TestLiveKitWebhook_InvalidSignature(t *testing.T) {
	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, nil, nil, nil)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestLiveKitWebhook_NotConfigured(t *testing.T) {
	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{}, nil, nil, nil, nil)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	req := httptest.NewRequest(http.MethodPost, "/webhook", nil)
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", resp.StatusCode)
	}
}

func TestLiveKitWebhook_ParticipantDisconnected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	db.Create(&models.User{ID: "wh-u1", Email: "wh1@ex.com", Name: "WH1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "wh-u2", Email: "wh2@ex.com", Name: "WH2", Provider: "local", IsActive: true})

	room, _ := roomRepo.CreateRoom("wh-u1", "wh-disco-room", false, "standard", 0, &models.RoomSettings{})
	_ = roomRepo.AddParticipant(room.ID, "wh-u1")
	_ = roomRepo.AddParticipant(room.ID, "wh-u2")

	event := &livekit.WebhookEvent{
		Event: "participant_left",
		Room:  &livekit.Room{Name: "wh-disco-room"},
		Participant: &livekit.ParticipantInfo{
			Identity: "wh-u2",
		},
	}

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, roomRepo, nil, nil, db)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify wh-u2 was deactivated but wh-u1 remains active
	var active []models.RoomParticipant
	db.Where("room_id = ? AND is_active = ?", room.ID, true).Find(&active)
	if len(active) != 1 {
		t.Fatalf("expected 1 active participant, got %d", len(active))
	}
	if active[0].UserID != "wh-u1" {
		t.Fatalf("expected wh-u1 to remain active, got %s", active[0].UserID)
	}
}

func TestLiveKitWebhook_RoomFinished(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	db.Create(&models.User{ID: "whr-u1", Email: "whr1@ex.com", Name: "WHR1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "whr-u2", Email: "whr2@ex.com", Name: "WHR2", Provider: "local", IsActive: true})

	room, _ := roomRepo.CreateRoom("whr-u1", "wh-finished-room", false, "standard", 0, &models.RoomSettings{})
	_ = roomRepo.AddParticipant(room.ID, "whr-u1")
	_ = roomRepo.AddParticipant(room.ID, "whr-u2")

	event := &livekit.WebhookEvent{
		Event: "room_finished",
		Room:  &livekit.Room{Name: "wh-finished-room"},
	}

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, roomRepo, nil, nil, db)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// All participants deactivated
	var active []models.RoomParticipant
	db.Where("room_id = ? AND is_active = ?", room.ID, true).Find(&active)
	if len(active) != 0 {
		t.Fatalf("expected 0 active participants, got %d", len(active))
	}

	// Room deactivated
	var r models.Room
	db.First(&r, "id = ?", room.ID)
	if r.IsActive {
		t.Fatal("expected room to be inactive after room_finished")
	}
}

func TestLiveKitWebhook_UnknownRoom(t *testing.T) {
	event := &livekit.WebhookEvent{
		Event: "participant_left",
		Room:  &livekit.Room{Name: "nonexistent-room"},
		Participant: &livekit.ParticipantInfo{
			Identity: "some-user",
		},
	}

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, nil, nil, nil)

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should still return 200 (no error) — unknown room is a soft fail
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLiveKitWebhook_MissingParticipantInEvent(t *testing.T) {
	event := &livekit.WebhookEvent{
		Event: "participant_left",
		Room:  &livekit.Room{Name: "some-room"},
		// No Participant
	}

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, nil, nil, nil)

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLiveKitWebhook_MissingRoomInEvent(t *testing.T) {
	event := &livekit.WebhookEvent{
		Event: "room_finished",
		// No Room
	}

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, nil, nil, nil)

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestLiveKitWebhook_UnhandledEvent(t *testing.T) {
	event := &livekit.WebhookEvent{
		Event: "track_published",
	}

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, nil, nil, nil)

	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// Egress webhook event tests
// ---------------------------------------------------------------------------

func newEgressEvent(egressID, event string, status livekit.EgressStatus) *livekit.WebhookEvent {
	return &livekit.WebhookEvent{
		Event: event,
		Room:  &livekit.Room{Name: "egress-test-room", Sid: "RM_test"},
		EgressInfo: &livekit.EgressInfo{
			EgressId: egressID,
			RoomName: "egress-test-room",
			Status:   status,
			RoomId:   "RM_test",
			FileResults: []*livekit.FileInfo{{
				Filename: "https://storage.example.com/recordings/test.mp4",
				Size:     1024 * 1024,
				Duration: 2 * 60 * 1e9, // 2 minutes in nanoseconds
			}},
		},
	}
}

func createEgressRecording(t *testing.T, recordingRepo *repository.RecordingRepository, egressID, roomID string, status models.RecordingStatus) *models.Recording {
	t.Helper()
	rec := &models.Recording{
		ID:            uuid.NewString(),
		RoomID:        roomID,
		RoomName:      "egress-test-room",
		EgressID:      egressID,
		RecordingType: "composite",
		Status:        status,
		CreatedBy:     "mod-user",
	}
	if err := recordingRepo.Create(rec); err != nil {
		t.Fatalf("failed to create recording: %v", err)
	}
	return rec
}

func TestLiveKitWebhook_EgressStarted(t *testing.T) {
	t.Skip("TODO oncoming feature")
	db := testutil.SetupTestDB(t)
	recordingRepo := repository.NewRecordingRepository(db)

	egressID := uuid.NewString()
	_ = createEgressRecording(t, recordingRepo, egressID, "room-1", models.RecordingPending)

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, recordingRepo, nil, db)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	event := newEgressEvent(egressID, "egress_started", livekit.EgressStatus_EGRESS_ACTIVE)
	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	rec, _ := recordingRepo.GetByEgressID(egressID)
	if rec.Status != models.RecordingStarted {
		t.Fatalf("expected status 'started', got '%s'", rec.Status)
	}
}

func TestLiveKitWebhook_EgressEnded(t *testing.T) {
	t.Skip("TODO oncoming feature")
	db := testutil.SetupTestDB(t)
	recordingRepo := repository.NewRecordingRepository(db)

	egressID := uuid.NewString()
	_ = createEgressRecording(t, recordingRepo, egressID, "room-1", models.RecordingStarted)

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, recordingRepo, nil, db)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	event := newEgressEvent(egressID, "egress_ended", livekit.EgressStatus_EGRESS_COMPLETE)
	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	rec, _ := recordingRepo.GetByEgressID(egressID)
	if rec.Status != models.RecordingProcessing {
		t.Fatalf("expected status 'processing', got '%s'", rec.Status)
	}
}

func TestLiveKitWebhook_EgressFailed(t *testing.T) {
	t.Skip("TODO oncoming feature")
	db := testutil.SetupTestDB(t)
	recordingRepo := repository.NewRecordingRepository(db)

	egressID := uuid.NewString()
	_ = createEgressRecording(t, recordingRepo, egressID, "room-1", models.RecordingStarted)

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, recordingRepo, nil, db)

	// egress_failed is reported via egress_ended event with EGRESS_FAILED status
	event := newEgressEvent(egressID, "egress_ended", livekit.EgressStatus_EGRESS_FAILED)
	event.EgressInfo.Error = "egress failed: test error"
	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	rec, _ := recordingRepo.GetByEgressID(egressID)
	if rec.Status != models.RecordingFailed {
		t.Fatalf("expected status 'failed', got '%s'", rec.Status)
	}
	if rec.Error != "egress failed: test error" {
		t.Fatalf("expected error 'egress failed: test error', got '%s'", rec.Error)
	}
}

func TestLiveKitWebhook_EgressEnded_Duplicate(t *testing.T) {
	t.Skip("TODO oncoming feature")
	db := testutil.SetupTestDB(t)
	recordingRepo := repository.NewRecordingRepository(db)

	egressID := uuid.NewString()
	_ = createEgressRecording(t, recordingRepo, egressID, "room-1", models.RecordingStarted)

	h := NewLiveKitWebhookHandler(&config.LiveKitConfig{
		APIKey:    whTestAPIKey,
		APISecret: whTestAPISecret,
	}, nil, recordingRepo, nil, db)

	app := fiber.New()
	app.Post("/webhook", h.Handle)

	event := newEgressEvent(egressID, "egress_ended", livekit.EgressStatus_EGRESS_COMPLETE)
	req := newWebhookRequest(t, whTestAPIKey, whTestAPISecret, event)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	rec, _ := recordingRepo.GetByEgressID(egressID)
	if rec.Status != models.RecordingProcessing {
		t.Fatalf("expected status 'processing' (unchanged), got '%s'", rec.Status)
	}
}

// newWebhookRequest creates a signed HTTP request simulating a LiveKit webhook.
func newWebhookRequest(t *testing.T, apiKey, apiSecret string, event *livekit.WebhookEvent) *http.Request {
	t.Helper()

	body, err := json.Marshal(event)
	if err != nil {
		t.Fatalf("failed to marshal event: %v", err)
	}

	// Compute SHA256 of body
	sha := sha256.Sum256(body)
	hash := base64.StdEncoding.EncodeToString(sha[:])

	// Generate JWT with sha256 claim
	at := auth.NewAccessToken(apiKey, apiSecret)
	at.SetSha256(hash)
	token, err := at.ToJWT()
	if err != nil {
		t.Fatalf("failed to generate webhook token: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// LiveKit sends the raw JWT (no "Bearer " prefix).
	req.Header.Set("Authorization", token)
	return req
}

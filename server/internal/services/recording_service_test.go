package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/livekit/protocol/livekit"
	"gorm.io/gorm"
)

func recordingTestSkipped(t *testing.T) {
	t.Skip("TODO oncoming feature")
}

func TestRecordingService_gateSystem_Enabled(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true})

	svc := NewRecordingService(settingsRepo, nil, nil, nil, "", "")
	err := svc.gateSystem()
	if err != nil {
		t.Fatalf("expected no error when recordings enabled, got: %v", err)
	}
}

func TestRecordingService_gateSystem_Disabled(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: false})

	svc := NewRecordingService(settingsRepo, nil, nil, nil, "", "")
	err := svc.gateSystem()
	if err == nil {
		t.Fatal("expected error when recordings disabled, got nil")
	}
	if err != ErrRecordingsDisabled {
		t.Fatalf("expected ErrRecordingsDisabled, got: %v", err)
	}
}

func TestRecordingService_gateSystem_DefaultDisabled(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	// Don't seed — default RecordingsEnabled is false

	svc := NewRecordingService(settingsRepo, nil, nil, nil, "", "")
	err := svc.gateSystem()
	if err == nil {
		t.Fatal("expected error when recordings default-disabled, got nil")
	}
}

func TestRecordingService_gateRoom_Allowed(t *testing.T) {
	recordingTestSkipped(t)
	svc := NewRecordingService(nil, nil, nil, nil, "", "")
	room := &models.Room{
		Settings: models.RoomSettings{RecordingsAllowed: true},
	}
	err := svc.gateRoom(room)
	if err != nil {
		t.Fatalf("expected no error when room allows recordings, got: %v", err)
	}
}

func TestRecordingService_gateRoom_NotAllowed(t *testing.T) {
	recordingTestSkipped(t)
	svc := NewRecordingService(nil, nil, nil, nil, "", "")
	room := &models.Room{
		Settings: models.RoomSettings{RecordingsAllowed: false},
	}
	err := svc.gateRoom(room)
	if err == nil {
		t.Fatal("expected error when room disallows recordings, got nil")
	}
	if err != ErrRecordingsNotAllowed {
		t.Fatalf("expected ErrRecordingsNotAllowed, got: %v", err)
	}
}

// ── helpers ─────────────────────────────────────────────────────────────────

// seedRoom inserts a room directly via DB (RoomRepository has no generic Create).
func seedRoom(t *testing.T, db *gorm.DB, room *models.Room) {
	t.Helper()
	if err := db.Create(room).Error; err != nil {
		t.Fatalf("seed room: %v", err)
	}
}

// seedRecording inserts a recording directly via DB.
func seedRecording(t *testing.T, db *gorm.DB, rec *models.Recording) {
	t.Helper()
	if err := db.Create(rec).Error; err != nil {
		t.Fatalf("seed recording: %v", err)
	}
}

// setupStopRecordingTest creates repos, seeds a room + recording in the given
// status, and returns everything needed for a StopRecording test.
func setupStopRecordingTest(t *testing.T, recordingStatus models.RecordingStatus) (
	*RecordingService,
	*repository.RecordingRepository,
	*testutil.MockEgress,
	models.Room,
	models.Recording,
) {
	t.Helper()
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)

	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true})
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)

	room := models.Room{
		ID:       "room-stop-" + t.Name(),
		Name:     "stop-test-room-" + t.Name(),
		IsActive: true,
		Settings: models.RoomSettings{RecordingsAllowed: true},
	}
	seedRoom(t, db, &room)

	rec := models.Recording{
		ID:            "rec-stop-" + t.Name(),
		RoomID:        room.ID,
		RoomName:      room.Name,
		RecordingType: "composite",
		Status:        recordingStatus,
		EgressID:      "egress-stop-" + t.Name(),
		CreatedBy:     "user-creator",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	seedRecording(t, db, &rec)

	mockEgress := testutil.NewMockEgress()
	svc := NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "test-key", "test-secret")

	return svc, recordingRepo, mockEgress, room, rec
}

// ── StopRecording tests ────────────────────────────────────────────────────

func TestRecordingService_StopRecording_HappyPath(t *testing.T) {
	recordingTestSkipped(t)
	svc, recordingRepo, mockEgress, room, rec := setupStopRecordingTest(t, models.RecordingStarted)

	recordingID, err := svc.StopRecording(context.Background(), room.ID)
	if err != nil {
		t.Fatalf("StopRecording failed: %v", err)
	}
	if recordingID != rec.ID {
		t.Fatalf("expected recording ID %s, got %s", rec.ID, recordingID)
	}

	// StopEgress was called on the mock
	if mockEgress.StopEgressCalls.Load() != 1 {
		t.Fatalf("expected 1 StopEgress call, got %d", mockEgress.StopEgressCalls.Load())
	}

	// Recording status was NOT changed — still "started"
	updated, err := recordingRepo.GetByID(rec.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != models.RecordingStarted {
		t.Fatalf("expected recording status to remain 'started', got %q", updated.Status)
	}
}

func TestRecordingService_StopRecording_NoActiveRecording(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true})
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)

	// Seed a room with no recordings
	room := models.Room{
		ID:       "room-no-rec-" + t.Name(),
		Name:     "no-rec-room-" + t.Name(),
		IsActive: true,
		Settings: models.RoomSettings{RecordingsAllowed: true},
	}
	seedRoom(t, db, &room)

	mockEgress := testutil.NewMockEgress()
	svc := NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "", "")

	_, err := svc.StopRecording(context.Background(), room.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrNoActiveRecording) {
		t.Fatalf("expected ErrNoActiveRecording, got: %v", err)
	}

	// StopEgress was NOT called
	if mockEgress.StopEgressCalls.Load() != 0 {
		t.Fatalf("expected 0 StopEgress calls, got %d", mockEgress.StopEgressCalls.Load())
	}
}

func TestRecordingService_StopRecording_WrongStatus(t *testing.T) {
	recordingTestSkipped(t)
	for _, status := range []models.RecordingStatus{
		models.RecordingPending,
		models.RecordingProcessing,
		models.RecordingCompleted,
		models.RecordingFailed,
	} {
		t.Run(string(status), func(t *testing.T) {
			svc, _, mockEgress, room, _ := setupStopRecordingTest(t, status)

			_, err := svc.StopRecording(context.Background(), room.ID)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			// StopEgress was NOT called for non-started recording
			if mockEgress.StopEgressCalls.Load() != 0 {
				t.Fatalf("expected 0 StopEgress calls for non-started recording, got %d", mockEgress.StopEgressCalls.Load())
			}
		})
	}
}

func TestRecordingService_StopRecording_EgressClientNil(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true})
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)

	room := models.Room{
		ID:       "room-nil-egress-" + t.Name(),
		Name:     "nil-egress-room-" + t.Name(),
		IsActive: true,
		Settings: models.RoomSettings{RecordingsAllowed: true},
	}
	seedRoom(t, db, &room)

	rec := models.Recording{
		ID:            "rec-nil-egress-" + t.Name(),
		RoomID:        room.ID,
		RoomName:      room.Name,
		RecordingType: "composite",
		Status:        models.RecordingStarted,
		EgressID:      "egress-nil-test-" + t.Name(),
		CreatedBy:     "user",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	seedRecording(t, db, &rec)

	// Pass nil egress client
	svc := NewRecordingService(settingsRepo, recordingRepo, roomRepo, nil, "", "")

	_, err := svc.StopRecording(context.Background(), room.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, ErrEgressClientNotReady) {
		t.Fatalf("expected ErrEgressClientNotReady, got: %v", err)
	}
}

func TestRecordingService_StopRecording_EmptyEgressID(t *testing.T) {
	recordingTestSkipped(t)
	config.SetForTest(&config.Config{})
	db := testutil.SetupTestDB(t)
	settingsRepo := repository.NewSettingsRepository(db)
	_ = settingsRepo.SaveSettings(&models.SystemSettings{RecordingsEnabled: true})
	roomRepo := repository.NewRoomRepository(db)
	recordingRepo := repository.NewRecordingRepository(db)

	room := models.Room{
		ID:       "room-empty-egressid-" + t.Name(),
		Name:     "empty-egressid-room-" + t.Name(),
		IsActive: true,
		Settings: models.RoomSettings{RecordingsAllowed: true},
	}
	seedRoom(t, db, &room)

	rec := models.Recording{
		ID:            "rec-empty-egressid-" + t.Name(),
		RoomID:        room.ID,
		RoomName:      room.Name,
		RecordingType: "composite",
		Status:        models.RecordingStarted,
		EgressID:      "", // empty
		CreatedBy:     "user",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}
	seedRecording(t, db, &rec)

	mockEgress := testutil.NewMockEgress()
	svc := NewRecordingService(settingsRepo, recordingRepo, roomRepo, mockEgress, "", "")

	_, err := svc.StopRecording(context.Background(), room.ID)
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// StopEgress NOT called — rejected before reaching it
	if mockEgress.StopEgressCalls.Load() != 0 {
		t.Fatalf("expected 0 StopEgress calls, got %d", mockEgress.StopEgressCalls.Load())
	}

	// Recording marked as failed with appropriate error
	updated, err := recordingRepo.GetByID(rec.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != models.RecordingFailed {
		t.Fatalf("expected recording to be failed, got %q", updated.Status)
	}
	if updated.Error == "" {
		t.Fatal("expected error message on recording, got empty")
	}
}

func TestRecordingService_StopRecording_StopEgressFails(t *testing.T) {
	recordingTestSkipped(t)
	svc, recordingRepo, mockEgress, room, rec := setupStopRecordingTest(t, models.RecordingStarted)

	// Make StopEgress return an error
	mockEgress.OnStopEgress = func(ctx context.Context, req *livekit.StopEgressRequest) (*livekit.EgressInfo, error) {
		return nil, errors.New("lk unreachable")
	}

	// Should still return success — stopEgress is best-effort, webhook handles finalization
	recordingID, err := svc.StopRecording(context.Background(), room.ID)
	if err != nil {
		t.Fatalf("StopRecording should return success even when StopEgress fails: %v", err)
	}
	if recordingID != rec.ID {
		t.Fatalf("expected recording ID %s, got %s", rec.ID, recordingID)
	}

	// StopEgress was called (even though it failed)
	if mockEgress.StopEgressCalls.Load() != 1 {
		t.Fatalf("expected 1 StopEgress call, got %d", mockEgress.StopEgressCalls.Load())
	}

	// Recording status still "started" — webhook will handle
	updated, err := recordingRepo.GetByID(rec.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if updated.Status != models.RecordingStarted {
		t.Fatalf("expected recording status to remain 'started', got %q", updated.Status)
	}
}

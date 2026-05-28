package repository

import (
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

func newTestRecording(roomID string) *models.Recording {
	return &models.Recording{
		ID:            uuid.NewString(),
		RoomID:        roomID,
		RoomName:      "test-room",
		EgressID:      uuid.NewString(),
		RecordingType: "composite",
		DurationMs:    0,
		Status:        models.RecordingPending,
		CreatedBy:     "user1",
	}
}

func recordingTestSkipped(t *testing.T) {
	t.Skip("TODO oncoming feature")
}

func TestRecordingRepository_Create(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	err := repo.Create(rec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64
	db.Model(&models.Recording{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 recording, got %d", count)
	}
}

func TestRecordingRepository_Create_InvalidType(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.RecordingType = "unknown-type"
	err := repo.Create(rec)
	if err == nil {
		t.Fatal("expected error for invalid recording type")
	}
}

func TestRecordingRepository_Create_EmptyRoomID(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("")
	rec.RoomID = ""
	err := repo.Create(rec)
	if err == nil {
		t.Fatal("expected error for empty room ID")
	}
}

func TestRecordingRepository_Create_EmptyCreatedBy(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.CreatedBy = ""
	err := repo.Create(rec)
	if err == nil {
		t.Fatal("expected error for empty created_by")
	}
}

func TestRecordingRepository_GetByID_Found(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	_ = repo.Create(rec)

	found, err := repo.GetByID(rec.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found.RoomID != rec.RoomID {
		t.Fatalf("expected room %s, got %s", rec.RoomID, found.RoomID)
	}
}

func TestRecordingRepository_GetByID_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	_, err := repo.GetByID("nonexistent")
	if err != ErrRecordingNotFound {
		t.Fatalf("expected ErrRecordingNotFound, got %v", err)
	}
}

func TestRecordingRepository_GetByEgressID_Found(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	_ = repo.Create(rec)

	found, err := repo.GetByEgressID(rec.EgressID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected recording, got nil")
	}
	if found.ID != rec.ID {
		t.Fatalf("expected recording %s, got %s", rec.ID, found.ID)
	}
}

func TestRecordingRepository_GetByEgressID_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	found, err := repo.GetByEgressID("nonexistent-egress")
	if !errors.Is(err, ErrRecordingNotFound) {
		t.Fatalf("expected ErrRecordingNotFound, got: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for missing egress ID")
	}
}

func TestRecordingRepository_GetActiveByRoom_Found(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingStarted
	_ = repo.Create(rec)

	active, err := repo.GetActiveByRoom("room-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active == nil {
		t.Fatal("expected active recording, got nil")
	}
	if active.ID != rec.ID {
		t.Fatalf("expected recording %s, got %s", rec.ID, active.ID)
	}
}

func TestRecordingRepository_GetActiveByRoom_NotFound(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	// Completed recording — not active
	rec := newTestRecording("room-1")
	rec.Status = models.RecordingCompleted
	_ = repo.Create(rec)

	active, err := repo.GetActiveByRoom("room-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if active != nil {
		t.Fatal("expected nil, got completed recording")
	}
}

func TestRecordingRepository_HasActiveRecording(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingStarted
	_ = repo.Create(rec)

	has, err := repo.HasActiveRecording("room-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !has {
		t.Fatal("expected room-1 to have active recording")
	}

	has, err = repo.HasActiveRecording("room-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if has {
		t.Fatal("expected room-2 to have no active recording")
	}
}

func TestRecordingRepository_ListByRoomID(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	for range 3 {
		rec := newTestRecording("room-1")
		_ = repo.Create(rec)
	}

	// Other room recording (shouldn't appear)
	other := newTestRecording("room-2")
	_ = repo.Create(other)

	recordings, total, err := repo.ListByRoomID("room-1", 0, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(recordings) != 3 {
		t.Fatalf("expected 3 recordings, got %d", len(recordings))
	}
}

func TestRecordingRepository_ListByRoomID_Pagination(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	for range 5 {
		rec := newTestRecording("room-1")
		_ = repo.Create(rec)
	}

	recordings, total, err := repo.ListByRoomID("room-1", 0, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Fatalf("expected total 5, got %d", total)
	}
	if len(recordings) != 2 {
		t.Fatalf("expected 2 recordings (page 1), got %d", len(recordings))
	}
}

func TestRecordingRepository_UpdateStatus_Success(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingPending
	_ = repo.Create(rec)

	err := repo.UpdateStatus(rec.ID, models.RecordingPending, models.RecordingStarted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.Status != models.RecordingStarted {
		t.Fatalf("expected status 'started', got '%s'", saved.Status)
	}
}

func TestRecordingRepository_UpdateStatus_OptimisticLock(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingStarted
	_ = repo.Create(rec)

	err := repo.UpdateStatus(rec.ID, models.RecordingPending, models.RecordingStarted)
	if err != ErrOptimisticLock {
		t.Fatalf("expected ErrOptimisticLock, got %v", err)
	}
}

func TestRecordingRepository_UpdateEgressID(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	_ = repo.Create(rec)

	newEgressID := uuid.NewString()
	err := repo.UpdateEgressID(rec.ID, newEgressID, models.RecordingStarted)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.EgressID != newEgressID {
		t.Fatalf("expected egress %s, got %s", newEgressID, saved.EgressID)
	}
	if saved.Status != models.RecordingStarted {
		t.Fatalf("expected status 'started', got '%s'", saved.Status)
	}
}

func TestRecordingRepository_UpdateError(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingStarted
	_ = repo.Create(rec)

	err := repo.UpdateError(rec.ID, "egress failed: timeout")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.Status != models.RecordingFailed {
		t.Fatalf("expected status 'failed', got '%s'", saved.Status)
	}
	if saved.Error != "egress failed: timeout" {
		t.Fatalf("expected error 'egress failed: timeout', got '%s'", saved.Error)
	}
}

func TestRecordingRepository_UpdateCompleted(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	_ = repo.Create(rec)

	now := time.Now()
	err := repo.UpdateCompleted(rec.ID, "https://storage.example.com/recording.mp4", 1024*1024, 120000, now)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.Status != models.RecordingCompleted {
		t.Fatalf("expected status 'completed', got '%s'", saved.Status)
	}
	if saved.FileSize != 1024*1024 {
		t.Fatalf("expected file size %d, got %d", 1024*1024, saved.FileSize)
	}
	if saved.DurationMs != 120000 {
		t.Fatalf("expected duration %d, got %d", 120000, saved.DurationMs)
	}
}

// ── Gap 6: Race tests for optimistic locking ───────────────────────────────

func TestRecordingRepository_UpdateStatus_RaceCondition(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-race")
	rec.Status = models.RecordingPending
	_ = repo.Create(rec)

	const goroutines = 10
	var wg sync.WaitGroup
	success := make(chan bool, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.UpdateStatus(rec.ID, models.RecordingPending, models.RecordingStarted)
			success <- err == nil
		}()
	}
	wg.Wait()
	close(success)

	var okCount int
	for s := range success {
		if s {
			okCount++
		}
	}

	if okCount != 1 {
		t.Fatalf("expected exactly 1 success (optimistic lock), got %d", okCount)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.Status != models.RecordingStarted {
		t.Fatalf("expected final status 'started', got '%s'", saved.Status)
	}
}

func TestRecordingRepository_UpdateEgressID_RaceCondition(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	repo := NewRecordingRepository(db)

	rec := newTestRecording("room-race-egress")
	rec.Status = models.RecordingPending
	_ = repo.Create(rec)

	const goroutines = 10
	var wg sync.WaitGroup
	success := make(chan bool, goroutines)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := repo.UpdateEgressID(rec.ID, uuid.NewString(), models.RecordingStarted)
			success <- err == nil
		}()
	}
	wg.Wait()
	close(success)

	var okCount int
	for s := range success {
		if s {
			okCount++
		}
	}

	if okCount != 1 {
		t.Fatalf("expected exactly 1 success (optimistic lock), got %d", okCount)
	}

	var saved models.Recording
	db.First(&saved, "id = ?", rec.ID)
	if saved.Status != models.RecordingStarted {
		t.Fatalf("expected final status 'started', got '%s'", saved.Status)
	}
}

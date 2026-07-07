package queue

import (
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/google/uuid"
)

// mockStore implements storage.RecordingStore for testing.
type mockStore struct {
	mu       sync.Mutex
	storeErr error
	stored   []mockStoreEntry
}

type mockStoreEntry struct {
	key  string
	data []byte
}

func (m *mockStore) Delete(ctx context.Context, key string) error {
	return nil
}

func (m *mockStore) Store(ctx context.Context, key string, src io.Reader, size int64) (*storage.RecordingAttachment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.storeErr != nil {
		return nil, m.storeErr
	}
	data, _ := io.ReadAll(src)
	m.stored = append(m.stored, mockStoreEntry{key: key, data: data})
	return &storage.RecordingAttachment{
		URL:  "https://storage.example.com/recordings/recording.mp4",
		Size: int64(len(data)),
	}, nil
}

func testRecordingJob(t *testing.T, fileURL string, durationMs int64) *models.Job {
	t.Helper()
	payload, _ := json.Marshal(ProcessRecordingPayload{
		RoomID:        "room-1",
		RoomName:      "test-room",
		EgressID:      uuid.NewString(),
		FileURL:       fileURL,
		FileSize:      1024 * 1024,
		RecordingType: "composite",
		DurationMs:    durationMs,
	})
	return &models.Job{
		ID:      uuid.NewString(),
		Type:    "process_recording",
		Payload: string(payload),
	}
}

func recordingTestSkipped(t *testing.T) {
	t.Skip("TODO oncoming feature")
}

func TestProcessRecording_Success(t *testing.T) {
	recordingTestSkipped(t)
	testData := []byte("fake-recording-data-12345")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer srv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{}

	// Create a recording in "processing" status
	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	rec.DurationMs = 120000
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify recording is completed
	saved, _ := recRepo.GetByID(rec.ID)
	if saved.Status != models.RecordingCompleted {
		t.Fatalf("expected status 'completed', got '%s'", saved.Status)
	}
	if saved.FileURL == "" {
		t.Fatal("expected file URL to be set")
	}

	// Verify data was stored
	if len(store.stored) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.stored))
	}
}

func TestProcessRecording_Idempotency(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{}

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingCompleted // already completed
	rec.EgressID = uuid.NewString()
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, "http://example.com/file", 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err != nil {
		t.Fatalf("expected nil error (skip), got %v", err)
	}

	// Should not have stored anything
	if len(store.stored) != 0 {
		t.Fatal("expected no store calls for already-completed recording")
	}
}

func TestProcessRecording_DownloadFailure(t *testing.T) {
	recordingTestSkipped(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{}

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err == nil {
		t.Fatal("expected error on download failure")
	}

	// Recording should be marked failed
	saved, _ := recRepo.GetByID(rec.ID)
	if saved.Status != models.RecordingFailed {
		t.Fatalf("expected status 'failed', got '%s'", saved.Status)
	}
	if saved.Error == "" {
		t.Fatal("expected error message to be set")
	}
}

func TestProcessRecording_StoreFailure(t *testing.T) {
	recordingTestSkipped(t)
	testData := []byte("fake-recording-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer srv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{storeErr: assertAnError}

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err == nil {
		t.Fatal("expected error on store failure")
	}

	// Recording should be marked failed
	saved, _ := recRepo.GetByID(rec.ID)
	if saved.Status != models.RecordingFailed {
		t.Fatalf("expected status 'failed', got '%s'", saved.Status)
	}
}

func TestProcessRecording_TempFileCleanup(t *testing.T) {
	recordingTestSkipped(t)
	testData := []byte("recording-data")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer srv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{}

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	_ = recRepo.Create(rec)

	// Count temp files before
	before, _ := filepath.Glob(os.TempDir() + "/recording-*")

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Count temp files after — same or fewer
	after, _ := filepath.Glob(os.TempDir() + "/recording-*")
	if len(after) > len(before) {
		t.Fatal("temp file was not cleaned up after success")
	}
}

func TestProcessRecording_MalformedURL(t *testing.T) {
	recordingTestSkipped(t)
	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	store := &mockStore{}

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, nil, "", "", "", "", store)
	job := testRecordingJob(t, "://invalid", 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}

	// Should be marked failed
	saved, _ := recRepo.GetByID(rec.ID)
	if saved.Status != models.RecordingFailed {
		t.Fatalf("expected status 'failed', got '%s'", saved.Status)
	}
}

// updateEgressIDInPayload replaces the egress ID in the JSON payload with the given ID.
func updateEgressIDInPayload(t *testing.T, payload, egressID string) string {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(payload), &data); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}
	data["egress_id"] = egressID
	updated, _ := json.Marshal(data)
	return string(updated)
}

// assertAnError is a non-nil error for testing store failures.
var assertAnError = &storeTestError{"store failed"}

type storeTestError struct{ msg string }

func (e *storeTestError) Error() string { return e.msg }

// newTestRecording creates a recording for testing with the given room ID.
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

// ─── resolveDownloadURL tests ────────────────────────────────────────────────

func TestResolveDownloadURL_Cloud_ReturnsAsIs(t *testing.T) {
	recordingTestSkipped(t)
	url := "https://storage.example.com/recordings/video.mp4"
	result, err := resolveDownloadURL(url, "lk.example.com:7880", "", "key", "secret", "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != url {
		t.Fatalf("expected URL unchanged for cloud, got %q", result)
	}
}

func TestResolveDownloadURL_Embedded_AddsAccessToken(t *testing.T) {
	recordingTestSkipped(t)
	url := "http://lk.example.com:7880/egress/uploads/recording.mp4"
	result, err := resolveDownloadURL(url, "lk.example.com:7880", "", "test-key", "test-secret", "test-room")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "access_token=") {
		t.Fatalf("expected access_token in URL, got %q", result)
	}
	if !strings.HasPrefix(result, url) {
		t.Fatalf("expected URL to keep original prefix, got %q", result)
	}
}

func TestResolveDownloadURL_EmbeddedInternalHost(t *testing.T) {
	recordingTestSkipped(t)
	url := "http://10.0.0.1:7880/egress/uploads/recording.mp4"
	// InternalHost matches, Host does not
	result, err := resolveDownloadURL(url, "public.lk.com:7880", "10.0.0.1:7880", "key", "secret", "test-room")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "access_token=") {
		t.Fatalf("expected access_token for InternalHost match, got %q", result)
	}
}

func TestResolveDownloadURL_EmptyKeys_ReturnsAsIs(t *testing.T) {
	recordingTestSkipped(t)
	url := "http://lk.example.com:7880/egress/uploads/recording.mp4"
	result, err := resolveDownloadURL(url, "lk.example.com:7880", "", "", "", "")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != url {
		t.Fatalf("expected URL unchanged when keys empty, got %q", result)
	}
}

func TestResolveDownloadURL_MalformedURL_ReturnsOriginal(t *testing.T) {
	recordingTestSkipped(t)
	url := "://invalid"
	result, err := resolveDownloadURL(url, "lk.example.com:7880", "", "key", "secret", "")
	if err == nil {
		t.Fatal("expected error for malformed URL")
	}
	if result != url {
		t.Fatalf("expected original URL on error, got %q", result)
	}
}

func TestResolveDownloadURL_PreservesExistingQuery(t *testing.T) {
	recordingTestSkipped(t)
	// URL with existing query params should preserve them + add access_token
	url := "http://lk.example.com:7880/egress/uploads/recording.mp4?foo=bar"
	result, err := resolveDownloadURL(url, "lk.example.com:7880", "", "test-key", "test-secret", "test-room")
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if !strings.Contains(result, "access_token=") {
		t.Fatalf("expected access_token in result, got %q", result)
	}
	if !strings.Contains(result, "foo=bar") {
		t.Fatalf("expected existing query params preserved, got %q", result)
	}
}

// ─── Webhook dispatch tests ────────────────────────────────────────────────

func TestProcessRecording_WebhookDispatch(t *testing.T) {
	recordingTestSkipped(t)
	testData := []byte("recording-webhook-test")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer srv.Close()

	subSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer subSrv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	whRepo := repository.NewWebhookRepository(db)
	store := &mockStore{}

	// Create an active webhook subscribed to recording.completed
	wh := &models.Webhook{
		ID:       uuid.NewString(),
		Name:     "test-webhook",
		URL:      subSrv.URL,
		Secret:   "test-secret",
		Events:   []string{models.EventRecordingCompleted},
		IsActive: true,
	}
	_ = whRepo.Create(wh)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	rec.DurationMs = 120000
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, whRepo, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify recording completed
	saved, _ := recRepo.GetByID(rec.ID)
	if saved.Status != models.RecordingCompleted {
		t.Fatalf("expected status 'completed', got '%s'", saved.Status)
	}

	// Verify dispatch_webhook job was enqueued
	var jobs []models.Job
	db.Where("type = ?", "dispatch_webhook").Find(&jobs)
	if len(jobs) != 1 {
		t.Fatalf("expected 1 dispatch_webhook job, got %d", len(jobs))
	}

	// Verify payload shape
	var wp WebhookPayload
	json.Unmarshal([]byte(jobs[0].Payload), &wp)
	if wp.Event != models.EventRecordingCompleted {
		t.Fatalf("expected event recording.completed, got %q", wp.Event)
	}
	if wp.URL != subSrv.URL {
		t.Fatalf("expected URL %q, got %q", subSrv.URL, wp.URL)
	}
	if _, ok := wp.Body["recordingId"]; !ok {
		t.Fatal("expected recordingId in body")
	}
}

func TestProcessRecording_NoWebhookSubscribers_SkipsEnqueue(t *testing.T) {
	recordingTestSkipped(t)
	testData := []byte("recording-no-webhook")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(testData)
	}))
	defer srv.Close()

	db := testutil.SetupTestDB(t)
	recRepo := repository.NewRecordingRepository(db)
	whRepo := repository.NewWebhookRepository(db)
	store := &mockStore{}

	// Create an active webhook but subscribed to a different event
	wh := &models.Webhook{
		ID:       uuid.NewString(),
		Name:     "other-webhook",
		URL:      "http://example.com/webhook",
		Secret:   "secret",
		Events:   []string{"room.created"},
		IsActive: true,
	}
	_ = whRepo.Create(wh)

	rec := newTestRecording("room-1")
	rec.Status = models.RecordingProcessing
	rec.EgressID = uuid.NewString()
	rec.DurationMs = 120000
	_ = recRepo.Create(rec)

	handler := NewProcessRecordingHandler(recRepo, whRepo, "", "", "", "", store)
	job := testRecordingJob(t, srv.URL, 120000)
	job.Payload = updateEgressIDInPayload(t, job.Payload, rec.EgressID)

	err := handler(context.Background(), db, job)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	// Verify no dispatch_webhook jobs
	var count int64
	db.Model(&models.Job{}).Where("type = ?", "dispatch_webhook").Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 dispatch_webhook jobs, got %d", count)
	}
}

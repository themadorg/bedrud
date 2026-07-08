package storage

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type mockObjectDeleter struct {
	mu     sync.Mutex
	calls  []string
	failOn string
}

func (m *mockObjectDeleter) DeleteObject(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, key)
	if m.failOn != "" && key == m.failOn {
		return fmt.Errorf("mock error for %s", key)
	}
	return nil
}

func (m *mockObjectDeleter) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := make([]string, len(m.calls))
	copy(r, m.calls)
	return r
}

// ─── Category B: Record ──────────────────────────────────────────────────

func TestRecord_SetsStorageBackend(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)

	if err := tracker.Record("room1", "user1", "hash1", ".png", 1024, "s3"); err != nil {
		t.Fatal(err)
	}
	var u models.ChatUpload
	if err := db.Where("file_hash = ?", "hash1").First(&u).Error; err != nil {
		t.Fatal(err)
	}
	if u.StorageBackend != "s3" {
		t.Fatalf("expected storage_backend 's3', got %q", u.StorageBackend)
	}
}

func TestRecord_DiskDefault(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)

	if err := tracker.Record("room1", "user1", "hash2", ".jpg", 2048, "disk"); err != nil {
		t.Fatal(err)
	}
	var u models.ChatUpload
	if err := db.Where("file_hash = ?", "hash2").First(&u).Error; err != nil {
		t.Fatal(err)
	}
	if u.StorageBackend != "disk" {
		t.Fatalf("expected storage_backend 'disk', got %q", u.StorageBackend)
	}
}

func TestRecord_InlineTracked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)

	if err := tracker.Record("room1", "user1", "hash3", ".webp", 512, "inline"); err != nil {
		t.Fatal(err)
	}
	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 inline record, got %d", count)
	}
	var u models.ChatUpload
	db.Where("file_hash = ?", "hash3").First(&u)
	if u.StorageBackend != "inline" {
		t.Fatalf("expected storage_backend 'inline', got %q", u.StorageBackend)
	}
}

// ─── Category C: DeleteByRoom ────────────────────────────────────────────

func TestDeleteByRoom_Disk_CleansFile(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, dir, nil)

	if err := tracker.Record("room1", "user1", "abc", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "abc.png")
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("disk file should have been removed")
	}
	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("DB records should be deleted")
	}
}

func TestDeleteByRoom_Disk_CrossRoomSafety(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, dir, nil)

	// Same hash in two rooms
	if err := tracker.Record("room1", "u1", "shared", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Record("room2", "u1", "shared", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "shared.png")
	if err := os.WriteFile(path, []byte("shared"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	// File should still exist because room2 references it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("file should survive cross-room reference")
	}
}

func TestDeleteByRoom_S3_CallsDeleter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	if err := tracker.Record("room1", "u1", "abc", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	keys := mockDel.Keys()
	if len(keys) != 1 || keys[0] != "abc.png" {
		t.Fatalf("expected [abc.png], got %v", keys)
	}
}

func TestDeleteByRoom_S3_CrossRoomSafety(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	if err := tracker.Record("room1", "u1", "shared", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Record("room2", "u1", "shared", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	if len(mockDel.Keys()) != 0 {
		t.Fatal("S3 delete should not be called when another room references the file")
	}
}

func TestDeleteByRoom_S3_DeleterNil(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)

	if err := tracker.Record("room1", "u1", "abc", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}
	// Should not panic or error when deleter is nil
	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteByRoom_S3_DeleterError(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{failOn: "abc.png"}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	if err := tracker.Record("room1", "u1", "abc", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}
	// Should not error — DeleteByRoom logs warning but continues
	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
}

func TestDeleteByRoom_Inline_NoFileOp(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	if err := tracker.Record("room1", "u1", "abc", ".png", 100, "inline"); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	// No disk os.Remove should happen, no S3 delete should happen
	if len(mockDel.Keys()) != 0 {
		t.Fatal("inline uploads should not trigger any file deletion")
	}
}

func TestDeleteByRoom_MixedBackends(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, dir, mockDel)

	if err := tracker.Record("room1", "u1", "disk-f", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Record("room1", "u1", "s3-f", ".jpg", 200, "s3"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Record("room1", "u1", "inline-f", ".gif", 50, "inline"); err != nil {
		t.Fatal(err)
	}

	diskPath := filepath.Join(dir, "disk-f.png")
	if err := os.WriteFile(diskPath, []byte("d"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}

	// Disk file cleaned
	if _, err := os.Stat(diskPath); !os.IsNotExist(err) {
		t.Fatal("disk file should be removed")
	}
	// S3 deleter called
	keys := mockDel.Keys()
	if len(keys) != 1 || keys[0] != "s3-f.jpg" {
		t.Fatalf("expected s3-f.jpg deleted, got %v", keys)
	}
}

func TestDeleteByRoom_EmptyRoom(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, dir, nil)

	if err := tracker.DeleteByRoom("nonexistent"); err != nil {
		t.Fatal(err)
	}
}

// ─── Category D: s3Store.deleteObject ─────────────────────────────────────

func TestS3DeleteObject_HTTPSignature(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Fatalf("expected DELETE, got %s", r.Method)
		}
		if r.Header.Get("x-amz-date") == "" {
			t.Fatal("missing x-amz-date header")
		}
		if r.Header.Get("x-amz-content-sha256") == "" {
			t.Fatal("missing x-amz-content-sha256 header")
		}
		if r.Header.Get("Authorization") == "" {
			t.Fatal("missing Authorization header")
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := &s3Store{cfg: config.ChatUploadS3Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "AKID",
		SecretKey: "secret",
		Region:    "auto",
	}}

	if err := store.deleteObject("abc.png"); err != nil {
		t.Fatal(err)
	}
}

func TestS3DeleteObject_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	store := &s3Store{cfg: config.ChatUploadS3Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "AKID",
		SecretKey: "secret",
		Region:    "auto",
	}}

	if err := store.deleteObject("abc.png"); err == nil {
		t.Fatal("expected error for 500 response")
	}
}

func TestS3DeleteObject_Success(t *testing.T) {
	var capturedMethod, capturedPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		capturedPath = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	store := &s3Store{cfg: config.ChatUploadS3Config{
		Endpoint:  srv.URL,
		Bucket:    "test-bucket",
		AccessKey: "AKID",
		SecretKey: "secret",
		Region:    "us-east-1",
	}}

	if err := store.deleteObject("abc.png"); err != nil {
		t.Fatal(err)
	}
	if capturedMethod != "DELETE" {
		t.Fatalf("expected DELETE, got %s", capturedMethod)
	}
	if capturedPath != "/test-bucket/abc.png" {
		t.Fatalf("expected /test-bucket/abc.png, got %s", capturedPath)
	}
}

func TestNewS3Deleter(t *testing.T) {
	cfg := config.ChatUploadS3Config{
		Endpoint:  "https://s3.example.com",
		Bucket:    "bucket",
		AccessKey: "key",
		SecretKey: "secret",
	}
	d := NewS3Deleter(cfg)
	if d == nil {
		t.Fatal("NewS3Deleter returned nil")
	}
	// Verify the deleter is properly constructed by checking deleteObject works
	// (it should fail because no server is running, not because of config errors)
	err := d.DeleteObject("test-key")
	if err == nil {
		t.Fatal("expected error from no server, not nil")
	}
}

// ─── FileHash content consistency ─────────────────────────────────────────

// TestContentHash_Deterministic verifies ContentHash produces consistent results.
func TestContentHash_Deterministic(t *testing.T) {
	a := ContentHash([]byte("hello"))
	b := ContentHash([]byte("hello"))
	if a != b {
		t.Fatal("ContentHash should be deterministic")
	}
	h := sha256.Sum256([]byte("hello"))
	expected := hex.EncodeToString(h[:])
	if a != expected {
		t.Fatalf("ContentHash(%q) = %q, want %q", "hello", a, expected)
	}
}

// TestRecord_QuotaTracking verifies all backends contribute to quota totals.
func TestRecord_QuotaTracking(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)

	_ = tracker.Record("r1", "u1", "h1", ".png", 1000, "disk")
	_ = tracker.Record("r1", "u1", "h2", ".jpg", 2000, "s3")
	_ = tracker.Record("r1", "u1", "h3", ".gif", 500, "inline")

	total, err := tracker.GetUserUploadBytes("u1")
	if err != nil {
		t.Fatal(err)
	}
	if total != 3500 {
		t.Fatalf("expected 3500 total bytes, got %d", total)
	}

	global, err := tracker.GetTotalUploadBytes()
	if err != nil {
		t.Fatal(err)
	}
	if global != 3500 {
		t.Fatalf("expected 3500 global bytes, got %d", global)
	}
}

// TestDeleteByRoom_Partial ensures that when DeleteByRoom encounters multiple
// records, a failure in one doesn't block others (best-effort cleanup).
func TestDeleteByRoom_S3_OneFailsOtherSucceeds(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{failOn: "bad-key.png"}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	_ = tracker.Record("room1", "u1", "bad-key", ".png", 100, "s3")
	_ = tracker.Record("room1", "u1", "good-key", ".jpg", 200, "s3")

	// Should not return error (best-effort)
	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}

	// Both DB records should be gone regardless
	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("all DB records should be deleted regardless of S3 errors")
	}
}

// TestNewChatUploadTracker_DefaultDir verifies the default chatDir is used when empty.
func TestNewChatUploadTracker_DefaultDir(t *testing.T) {
	db := testutil.SetupTestDB(t)
	_ = testutil.SetupTestDB(t)

	var tracker *ChatUploadTracker

	// Create a separate function to capture initialization
	initTracker := func() {
		tracker = NewChatUploadTracker(db, "", nil)
	}
	initTracker()

	if tracker.chatDir != "./data/uploads/chat" {
		t.Fatalf("expected default dir, got %q", tracker.chatDir)
	}
}

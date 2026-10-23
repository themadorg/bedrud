package storage

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/testutil"

	"github.com/google/uuid"
	"gorm.io/gorm"
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
	path := filepath.Join(dir, "room1", "user1", "abc.png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
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

func TestDeleteByRoom_Disk_ScopedKeysIndependent(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, dir, nil)

	// Same hash in two rooms → distinct keys room/user/hash
	if err := tracker.Record("room1", "u1", "shared", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	if err := tracker.Record("room2", "u1", "shared", ".png", 100, "disk"); err != nil {
		t.Fatal(err)
	}
	path1 := filepath.Join(dir, "room1", "u1", "shared.png")
	path2 := filepath.Join(dir, "room2", "u1", "shared.png")
	for _, p := range []string{path1, path2} {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("shared"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	if err := tracker.DeleteByRoom("room1"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path1); !os.IsNotExist(err) {
		t.Fatal("room1 object should be removed")
	}
	if _, err := os.Stat(path2); os.IsNotExist(err) {
		t.Fatal("room2 object should remain")
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
	if len(keys) != 1 || keys[0] != "room1/u1/abc.png" {
		t.Fatalf("expected [room1/u1/abc.png], got %v", keys)
	}
}

func TestDeleteByRoom_S3_ScopedKeysIndependent(t *testing.T) {
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
	keys := mockDel.Keys()
	if len(keys) != 1 || keys[0] != "room1/u1/shared.png" {
		t.Fatalf("expected only room1 key deleted, got %v", keys)
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
	mockDel := &mockObjectDeleter{failOn: "room1/u1/abc.png"}
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

	diskPath := filepath.Join(dir, "room1", "u1", "disk-f.png")
	if err := os.MkdirAll(filepath.Dir(diskPath), 0o755); err != nil {
		t.Fatal(err)
	}
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
	if len(keys) != 1 || keys[0] != "room1/u1/s3-f.jpg" {
		t.Fatalf("expected room1/u1/s3-f.jpg deleted, got %v", keys)
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

// ─── Category C2: DeleteOlderThan ────────────────────────────────────────

// seedUploadAt inserts an upload row with an explicit created_at, which Record
// cannot do, and returns its ID. GORM only fills autoCreateTime when it is zero.
func seedUploadAt(t *testing.T, db *gorm.DB, roomID, userID, hash, backend string, createdAt time.Time) string {
	t.Helper()
	upload := &models.ChatUpload{
		ID:             uuid.New().String(),
		RoomID:         roomID,
		UploadedBy:     userID,
		FileHash:       hash,
		Extension:      ".png",
		FileSize:       100,
		StorageBackend: backend,
		CreatedAt:      createdAt,
	}
	if err := db.Create(upload).Error; err != nil {
		t.Fatal(err)
	}
	return upload.ID
}

// writeUploadFile creates the on-disk object for room/user/hash.png and returns its path.
func writeUploadFile(t *testing.T, dir, roomID, userID, hash string) string {
	t.Helper()
	path := filepath.Join(dir, roomID, userID, hash+".png")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertUploadIDs(t *testing.T, db *gorm.DB, want ...string) {
	t.Helper()
	var ids []string
	if err := db.Model(&models.ChatUpload{}).Order("id").Pluck("id", &ids).Error; err != nil {
		t.Fatal(err)
	}
	sort.Strings(want)
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("remaining uploads = %v, want %v", ids, want)
	}
}

func TestDeleteOlderThan_Disk_RemovesExpiredKeepsFresh(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, dir, nil)
	now := time.Now()

	seedUploadAt(t, db, "room1", "u1", "old", "disk", now.Add(-72*time.Hour))
	freshID := seedUploadAt(t, db, "room1", "u1", "fresh", "disk", now.Add(-1*time.Hour))
	oldPath := writeUploadFile(t, dir, "room1", "u1", "old")
	freshPath := writeUploadFile(t, dir, "room1", "u1", "fresh")

	deleted, err := tracker.DeleteOlderThan(now.Add(-48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 1 {
		t.Fatalf("deleted = %d, want 1", deleted)
	}
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatal("expired file should have been removed")
	}
	if _, err := os.Stat(freshPath); err != nil {
		t.Fatalf("fresh file should remain: %v", err)
	}
	assertUploadIDs(t, db, freshID)
}

// The same image uploaded twice by one user in one room maps to one object key.
// Expiring the older row must not take the object out from under the newer one.
func TestDeleteOlderThan_KeepsObjectStillReferencedByFreshRow(t *testing.T) {
	dir := t.TempDir()
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, dir, mockDel)
	now := time.Now()

	seedUploadAt(t, db, "room1", "u1", "dup", "disk", now.Add(-72*time.Hour))
	freshID := seedUploadAt(t, db, "room1", "u1", "dup", "disk", now.Add(-1*time.Hour))
	seedUploadAt(t, db, "room1", "u1", "dups3", "s3", now.Add(-72*time.Hour))
	freshS3ID := seedUploadAt(t, db, "room1", "u1", "dups3", "s3", now.Add(-1*time.Hour))
	path := writeUploadFile(t, dir, "room1", "u1", "dup")

	deleted, err := tracker.DeleteOlderThan(now.Add(-48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 2 {
		t.Fatalf("deleted = %d, want 2", deleted)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("shared file should remain: %v", err)
	}
	if keys := mockDel.Keys(); len(keys) != 0 {
		t.Fatalf("shared S3 object should remain, deleted %v", keys)
	}
	assertUploadIDs(t, db, freshID, freshS3ID)
}

func TestDeleteOlderThan_S3_CallsDeleter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	mockDel := &mockObjectDeleter{}
	tracker := NewChatUploadTracker(db, t.TempDir(), mockDel)

	seedUploadAt(t, db, "room1", "u1", "abc", "s3", time.Now().Add(-72*time.Hour))

	if _, err := tracker.DeleteOlderThan(time.Now().Add(-48 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	keys := mockDel.Keys()
	if len(keys) != 1 || keys[0] != "room1/u1/abc.png" {
		t.Fatalf("deleted keys = %v, want [room1/u1/abc.png]", keys)
	}
}

func TestDeleteOlderThan_NothingExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := NewChatUploadTracker(db, t.TempDir(), nil)
	freshID := seedUploadAt(t, db, "room1", "u1", "fresh", "inline", time.Now())

	deleted, err := tracker.DeleteOlderThan(time.Now().Add(-48 * time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0", deleted)
	}
	assertUploadIDs(t, db, freshID)
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
	d := NewS3Deleter(&cfg)
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
	mockDel := &mockObjectDeleter{failOn: "room1/u1/bad-key.png"}
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

func TestS3PresignGet_Shape(t *testing.T) {
	store := &s3Store{cfg: config.ChatUploadS3Config{
		Endpoint:  "https://s3.example.com",
		Bucket:    "bucket",
		AccessKey: "AKID",
		SecretKey: "secret",
		Region:    "us-east-1",
	}}
	url, err := store.PresignGet("room1/u1/abc.png", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(url, "https://s3.example.com/bucket/room1/u1/abc.png?") {
		t.Fatalf("unexpected url prefix: %s", url)
	}
	for _, want := range []string{
		"X-Amz-Algorithm=AWS4-HMAC-SHA256",
		"X-Amz-Credential=",
		"X-Amz-Date=",
		"X-Amz-Expires=3600",
		"X-Amz-SignedHeaders=host",
		"X-Amz-Signature=",
	} {
		if !strings.Contains(url, want) {
			t.Fatalf("missing %q in %s", want, url)
		}
	}
}

func TestS3Store_ReturnsProxyURL(t *testing.T) {
	// Minimal 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53, 0xde, 0x00, 0x00, 0x00,
		0x0c, 0x49, 0x44, 0x41, 0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x03, 0x00, 0x01, 0x00, 0x05, 0xfe, 0xd4, 0xef, 0x00, 0x00,
		0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}
	var putPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		putPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	store := &s3Store{
		cfg: config.ChatUploadS3Config{
			Endpoint:  srv.URL,
			Bucket:    "b",
			AccessKey: "AKID",
			SecretKey: "secret",
			Region:    "us-east-1",
		},
		inlineMaxBytes: 1, // force S3 path
		diskFallback:   &diskStore{dir: t.TempDir()},
	}
	att, err := store.Store("room1", "user1", png)
	if err != nil {
		t.Fatal(err)
	}
	wantPrefix := "/uploads/chat/room1/user1/"
	if !strings.HasPrefix(att.URL, wantPrefix) {
		t.Fatalf("expected proxy URL under %s, got %s", wantPrefix, att.URL)
	}
	if att.StorageBackend != uploadBackendS3 {
		t.Fatalf("expected s3 backend, got %s", att.StorageBackend)
	}
	wantPut := "/b/room1/user1/" + filepath.Base(att.URL)
	if putPath != wantPut {
		t.Fatalf("put path %q want %q", putPath, wantPut)
	}
}

func TestChatUploadRoomID(t *testing.T) {
	if room, ok := ChatUploadRoomID("r1/u1/h.png"); !ok || room != "r1" {
		t.Fatalf("got room=%q ok=%v", room, ok)
	}
	for _, bad := range []string{"", "a", "a/b", "../x/y", "a/b/c/d", "a//b.png"} {
		if _, ok := ChatUploadRoomID(bad); ok {
			t.Fatalf("expected reject %q", bad)
		}
	}
}

func TestResolveChatUpload_DiskAndPresign(t *testing.T) {
	dir := t.TempDir()
	key := "room1/user1/deadbeef.png"
	path := filepath.Join(dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("img"), 0o644); err != nil {
		t.Fatal(err)
	}
	file, redir, err := ResolveChatUpload(key, dir, nil)
	if err != nil || redir != "" || file != path {
		t.Fatalf("disk: file=%q redir=%q err=%v", file, redir, err)
	}
	if _, _, err := ResolveChatUpload("../etc/passwd", dir, nil); err == nil {
		t.Fatal("expected traversal reject")
	}
	missing := "room1/user1/missing.png"
	presigner := &s3Store{cfg: config.ChatUploadS3Config{
		Endpoint: "https://s3.example.com", Bucket: "b", AccessKey: "a", SecretKey: "s", Region: "us-east-1",
	}}
	file, redir, err = ResolveChatUpload(missing, dir, presigner)
	if err != nil || file != "" || !strings.Contains(redir, "X-Amz-Signature=") {
		t.Fatalf("presign: file=%q redir=%q err=%v", file, redir, err)
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

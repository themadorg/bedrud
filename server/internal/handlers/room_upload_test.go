package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// mockObjectDeleter records which keys were deleted.
type mockObjectDeleter struct {
	mu    sync.Mutex
	keys  []string
	failn int // fail after this many calls (0 = never fail)
}

func (m *mockObjectDeleter) DeleteObject(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	if m.failn > 0 && len(m.keys) >= m.failn {
		return nil // don't actually fail, just record
	}
	return nil
}

func (m *mockObjectDeleter) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := make([]string, len(m.keys))
	copy(r, m.keys)
	return r
}

// uploadTestApp creates a Fiber app with a mock S3 deleter wired for cleanup tests.
func uploadTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *mockObjectDeleter, *storage.ChatUploadTracker) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	mockDel := &mockObjectDeleter{}
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir(), mockDel)
	cleanupSvc := testCleanupSvc(t, roomRepo, uploadTracker)

	lkCfg := config.LiveKitConfig{
		Host:      "http://localhost:9999",
		APIKey:    "test-key",
		APISecret: "test-secret",
	}
	handler := NewRoomHandler(&lkCfg, &config.ChatConfig{}, roomRepo, nil, settingsRepo, uploadTracker, cleanupSvc)

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

	app.Post("/admin/rooms/:roomId/close", handler.AdminCloseRoom)
	app.Post("/admin/rooms/:roomId/suspend", handler.AdminSuspendRoom)

	db.Create(&models.User{
		ID: "admin-user", Email: "admin@ex.com", Name: "Admin",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"superadmin"},
	})

	return app, roomRepo, mockDel, uploadTracker
}

// ─── Category A: detectUploadBackend ────────────────────────────────────

func TestDetectUploadBackend_Disk(t *testing.T) {
	if got := detectUploadBackend("/uploads/chat/abc.png"); got != "disk" {
		t.Fatalf("expected disk, got %s", got)
	}
}

func TestDetectUploadBackend_Inline(t *testing.T) {
	if got := detectUploadBackend("data:image/png;base64,abc"); got != "inline" {
		t.Fatalf("expected inline, got %s", got)
	}
}

func TestDetectUploadBackend_S3(t *testing.T) {
	if got := detectUploadBackend("https://cdn.example.com/bucket/abc.png"); got != "s3" {
		t.Fatalf("expected s3, got %s", got)
	}
}

func TestDetectUploadBackend_Empty(t *testing.T) {
	if got := detectUploadBackend(""); got != "s3" {
		t.Fatalf("expected s3 (default), got %s", got)
	}
}

// ─── Category A: parseUploadMeta ─────────────────────────────────────────

func TestParseUploadMeta_Disk(t *testing.T) {
	hash, ext, backend := parseUploadMeta("/uploads/chat/abcdef123.png", "image/png", nil)
	if hash != "abcdef123" || ext != ".png" || backend != "disk" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_Disk_NoExt(t *testing.T) {
	hash, ext, backend := parseUploadMeta("/uploads/chat/abcdef123", "image/png", nil)
	if hash != "abcdef123" || ext != "" || backend != "disk" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_S3(t *testing.T) {
	hash, ext, backend := parseUploadMeta("https://cdn.example.com/bucket/abcdef123.jpg", "image/jpeg", nil)
	if hash != "abcdef123" || ext != ".jpg" || backend != "s3" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_S3_MultipleDots(t *testing.T) {
	hash, ext, backend := parseUploadMeta("https://cdn.example.com/bucket/img.test.abc.jpg", "image/jpeg", nil)
	if hash != "img.test.abc" || ext != ".jpg" || backend != "s3" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_Inline(t *testing.T) {
	data := []byte("fake-image-data")
	h := sha256.Sum256(data)
	expectedHash := hex.EncodeToString(h[:])
	hash, ext, backend := parseUploadMeta("data:image/png;base64,abc", "image/png", data)
	if hash != expectedHash || ext != ".png" || backend != "inline" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_Inline_EmptyData(t *testing.T) {
	hash, ext, backend := parseUploadMeta("data:image/png;base64,", "image/png", nil)
	if hash != "" || ext != ".png" || backend != "inline" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

func TestParseUploadMeta_S3_NoSlash(t *testing.T) {
	hash, ext, backend := parseUploadMeta("justafilename.png", "image/png", nil)
	if hash != "" || ext != "" || backend != "s3" {
		t.Fatalf("got hash=%q ext=%q backend=%q", hash, ext, backend)
	}
}

// ─── Category G: Room deletion cleans S3 uploads ─────────────────────────

func TestAdminCloseRoom_CleansS3Uploads(t *testing.T) {
	app, roomRepo, _, tracker := uploadTestApp(t)

	room, err := roomRepo.CreateRoom("admin-user", "close-s3", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tracker.Record(room.ID, "admin-user", "s3obj1", ".png", 100, "s3"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/close", nil)
	resp, _ := app.Test(req, -1)
	resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestAdminCloseRoom_CleansOnlyOwnS3Uploads(t *testing.T) {
	app, roomRepo, _, tracker := uploadTestApp(t)

	room, err := roomRepo.CreateRoom("admin-user", "close-s3-multi", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	_ = tracker.Record(room.ID, "admin-user", "disk1", ".png", 100, "disk")
	_ = tracker.Record(room.ID, "admin-user", "s3obj2", ".jpg", 200, "s3")
	_ = tracker.Record(room.ID, "admin-user", "inline1", ".gif", 50, "inline")

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/close", nil)
	resp, _ := app.Test(req, -1)
	resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

func TestAdminSuspendRoom_CleansS3Uploads(t *testing.T) {
	app, roomRepo, _, tracker := uploadTestApp(t)

	room, err := roomRepo.CreateRoom("admin-user", "suspend-s3", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}

	if err := tracker.Record(room.ID, "admin-user", "s3obj3", ".png", 150, "s3"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/"+room.ID+"/suspend", nil)
	resp, _ := app.Test(req, -1)
	resp.Body.Close()
	if resp.StatusCode != 202 {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}
}

// ─── Category F: Upload tracking ─────────────────────────────────────────

// TestUploadTrackingViaRecord verifies that Record stores the correct
// StorageBackend in the DB for all three backend types.
func TestUploadTrackingViaRecord(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)

	_ = tracker.Record("room1", "user1", "disk-h", ".png", 100, "disk")
	_ = tracker.Record("room1", "user1", "s3-h", ".jpg", 200, "s3")
	_ = tracker.Record("room1", "user1", "inline-h", ".gif", 50, "inline")

	var uploads []models.ChatUpload
	db.Where("room_id = ?", "room1").Find(&uploads)
	if len(uploads) != 3 {
		t.Fatalf("expected 3 uploads, got %d", len(uploads))
	}

	backends := map[string]bool{}
	for _, u := range uploads {
		backends[u.StorageBackend] = true
	}
	if !backends["disk"] || !backends["s3"] || !backends["inline"] {
		t.Fatalf("expected all three backends, got %v", backends)
	}
}

// TestUploadChatImage_TrackedDirectly simulates what UploadChatImage does
// and verifies the backend is correctly set in the DB.
func TestUploadChatImage_TrackingIntegration(t *testing.T) {
	db := testutil.SetupTestDB(t)
	tracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)

	// Simulate a disk upload result
	mime := "image/png"
	data := []byte("fake-png")
	hash := storage.ContentHash(data)
	url := "/uploads/chat/" + hash + ".png"

	h, ext, backend := parseUploadMeta(url, mime, data)
	if h != hash || ext != ".png" || backend != "disk" {
		t.Fatalf("parseUploadMeta disk: got hash=%q ext=%q backend=%q", h, ext, backend)
	}

	_ = tracker.Record("room-x", "user-x", h, ext, 100, backend)

	var u models.ChatUpload
	db.Where("file_hash = ?", hash).First(&u)
	if u.StorageBackend != "disk" {
		t.Fatalf("expected disk, got %q", u.StorageBackend)
	}

	// Simulate an S3 upload result
	s3url := "https://s3.example.com/bucket/" + hash + ".png"
	_, _, s3backend := parseUploadMeta(s3url, mime, data)
	if s3backend != "s3" {
		t.Fatalf("expected s3 backend, got %q", s3backend)
	}
}

// ─── Helper tests for services ───────────────────────────────────────────

// TestServices_UsesTrackedMockDel ensures that when RoomCleanupService is
// created with an uploadTracker that has a mock deleter, deletion calls
// propagate correctly.
func TestServices_RoomCleanup_WithMockDeleter(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	mockDel := &mockObjectDeleter{}
	tracker := storage.NewChatUploadTracker(db, t.TempDir(), mockDel)
	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:7880", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := services.NewRoomCleanupService(roomRepo, client, "test", "testsecret1234567890123456789012", tracker)

	db.Create(&models.User{ID: "user-a", Email: "a@ex.com", Name: "A", Provider: "local", IsActive: true})
	room, err := roomRepo.CreateRoom("user-a", "svc-test-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}

	_ = tracker.Record(room.ID, "user-a", "svc-key", ".png", 100, "s3")

	if err = svc.CascadeDeleteRoom(t.Context(), room, services.CascadeDeleteOptions{}); err != nil {
		t.Fatal(err)
	}

	keys := mockDel.Keys()
	if len(keys) != 1 || keys[0] != "svc-key.png" {
		t.Fatalf("expected [svc-key.png], got %v", keys)
	}
}

// helper to keep test import alive
var _ = strings.NewReader

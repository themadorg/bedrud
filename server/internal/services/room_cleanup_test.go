package services

import (
	"bedrud/config"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"context"
	"sync"
	"testing"
)

type mockDeleter struct {
	mu   sync.Mutex
	keys []string
}

func (m *mockDeleter) DeleteObject(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keys = append(m.keys, key)
	return nil
}

func (m *mockDeleter) Keys() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	r := make([]string, len(m.keys))
	copy(r, m.keys)
	return r
}

func TestRoomCleanupService_SuspendRoom_CleansUploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir(), nil)

	db.Create(&models.User{ID: "suspend-upload-user", Email: "suu@ex.com", Name: "SUU", Provider: "local", IsActive: true})

	// Create room
	room, _ := roomRepo.CreateRoom("suspend-upload-user", "suspend-upload-test", false, "standard", 0, &models.RoomSettings{})

	// Record an upload
	_ = uploadTracker.Record(room.ID, "suspend-upload-user", "abc123hash", ".png", 1024, "disk")

	// Verify upload exists
	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 1 {
		t.Fatal("expected 1 upload before suspend")
	}

	// Suspend — with fake LK client
	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", uploadTracker)
	_ = svc.SuspendRoom(context.Background(), room)

	// Verify uploads cleaned up
	count = 0
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 uploads after suspend")
	}
}

// ─── Category E: S3 cleanup tests ────────────────────────────────────────

func cleanupSvcWithDeleter(t *testing.T, roomRepo *repository.RoomRepository) (*storage.ChatUploadTracker, *mockDeleter) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	del := &mockDeleter{}
	tracker := storage.NewChatUploadTracker(db, t.TempDir(), del)
	return tracker, del
}

func TestCascadeDeleteRoom_CleansS3Uploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	tracker, del := cleanupSvcWithDeleter(t, roomRepo)

	db.Create(&models.User{ID: "cd-s3-user", Email: "cds3@ex.com", Name: "S3U", Provider: "local", IsActive: true})

	room, _ := roomRepo.CreateRoom("cd-s3-user", "cascade-s3", false, "standard", 0, &models.RoomSettings{})
	_ = tracker.Record(room.ID, "cd-s3-user", "cascade-key", ".png", 200, "s3")

	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", tracker)
	_ = svc.CascadeDeleteRoom(context.Background(), room, CascadeDeleteOptions{})

	keys := del.Keys()
	if len(keys) != 1 || keys[0] != "cascade-key.png" {
		t.Fatalf("expected [cascade-key.png], got %v", keys)
	}

	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 upload records after cascade delete")
	}
}

func TestCascadeDeleteRoom_CleansMixedUploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	tracker, del := cleanupSvcWithDeleter(t, roomRepo)

	db.Create(&models.User{ID: "mix-user", Email: "mix@ex.com", Name: "Mix", Provider: "local", IsActive: true})

	room, _ := roomRepo.CreateRoom("mix-user", "cascade-mix", false, "standard", 0, &models.RoomSettings{})
	_ = tracker.Record(room.ID, "mix-user", "disk-h", ".png", 100, "disk")
	_ = tracker.Record(room.ID, "mix-user", "s3-h", ".jpg", 200, "s3")
	_ = tracker.Record(room.ID, "mix-user", "inline-h", ".gif", 50, "inline")

	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", tracker)
	_ = svc.CascadeDeleteRoom(context.Background(), room, CascadeDeleteOptions{})

	keys := del.Keys()
	if len(keys) != 1 || keys[0] != "s3-h.jpg" {
		t.Fatalf("expected only S3 key deleted, got %v", keys)
	}

	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 upload records after cascade delete")
	}
}

func TestSuspendRoom_CleansS3Uploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	tracker, del := cleanupSvcWithDeleter(t, roomRepo)

	db.Create(&models.User{ID: "s3-suspend-user", Email: "s3su@ex.com", Name: "S3SU", Provider: "local", IsActive: true})

	room, _ := roomRepo.CreateRoom("s3-suspend-user", "suspend-s3-test", false, "standard", 0, &models.RoomSettings{})
	_ = tracker.Record(room.ID, "s3-suspend-user", "suspend-s3-key", ".png", 300, "s3")

	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", tracker)
	_ = svc.SuspendRoom(context.Background(), room)

	keys := del.Keys()
	if len(keys) != 1 || keys[0] != "suspend-s3-key.png" {
		t.Fatalf("expected [suspend-s3-key.png], got %v", keys)
	}

	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 upload records after suspend")
	}
}

func TestDeleteUserRooms_CleansS3Uploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	tracker, del := cleanupSvcWithDeleter(t, roomRepo)

	db.Create(&models.User{ID: "del-user", Email: "del@ex.com", Name: "Del", Provider: "local", IsActive: true})

	room1, _ := roomRepo.CreateRoom("del-user", "del-s3-1", false, "standard", 0, &models.RoomSettings{})
	room2, _ := roomRepo.CreateRoom("del-user", "del-s3-2", false, "standard", 0, &models.RoomSettings{})
	_ = tracker.Record(room1.ID, "del-user", "del-key-1", ".png", 100, "s3")
	_ = tracker.Record(room2.ID, "del-user", "del-key-2", ".jpg", 200, "s3")

	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, nil, client, nil, "test", "testsecret1234567890123456789012", tracker)
	_ = svc.DeleteUserRooms(context.Background(), []models.Room{*room1, *room2}, "del-user")

	keys := del.Keys()
	if len(keys) != 2 {
		t.Fatalf("expected 2 S3 keys deleted, got %v", keys)
	}

	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 upload records after user delete")
	}
}

package services

import (
	"bedrud/config"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
	"context"
	"testing"
)

func TestRoomCleanupService_SuspendRoom_CleansUploads(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	uploadTracker := storage.NewChatUploadTracker(db, t.TempDir())

	db.Create(&models.User{ID: "suspend-upload-user", Email: "suu@ex.com", Name: "SUU", Provider: "local", IsActive: true})

	// Create room
	room, _ := roomRepo.CreateRoom("suspend-upload-user", "suspend-upload-test", false, "standard", 0, &models.RoomSettings{})

	// Record an upload
	_ = uploadTracker.Record(room.ID, "abc123hash", ".png")

	// Verify upload exists
	var count int64
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 1 {
		t.Fatal("expected 1 upload before suspend")
	}

	// Suspend — with fake LK client
	client := lkutil.NewClient(&config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "test", APISecret: "testsecret1234567890123456789012"})
	svc := NewRoomCleanupService(roomRepo, client, "test", "testsecret1234567890123456789012", uploadTracker)
	_ = svc.SuspendRoom(context.Background(), room)

	// Verify uploads cleaned up
	count = 0
	db.Model(&models.ChatUpload{}).Count(&count)
	if count != 0 {
		t.Fatal("expected 0 uploads after suspend")
	}
}

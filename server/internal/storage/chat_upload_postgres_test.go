package storage

import (
	"os"
	"testing"
	"time"

	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/testutil/pgtest"

	"gorm.io/gorm/logger"
)

// The rest of the storage suite runs on SQLite, where created_at is text and
// the cutoff compares lexicographically. Postgres stores timestamptz and
// compares instants, and it also enforces the chat_uploads → rooms foreign key
// that SQLite leaves off, so the age sweep is checked on the real dialect here.
func TestPostgres_DeleteOlderThan_RemovesExpiredKeepsFresh(t *testing.T) {
	db := pgtest.Open(t, database.GormConfig(logger.Silent))
	database.SetForTest(db)
	t.Cleanup(database.ResetForTest)
	if err := database.RunMigrations(); err != nil {
		t.Fatalf("migrate postgres schema: %v", err)
	}

	user := &models.User{ID: "user-1", Email: "owner@example.com", Name: "Owner", Provider: "local"}
	if err := db.Create(user).Error; err != nil {
		t.Fatal(err)
	}
	room := &models.Room{ID: "room-1", Name: "ttl-room", CreatedBy: user.ID, AdminID: user.ID, IsActive: true}
	if err := db.Create(room).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	tracker := NewChatUploadTracker(db, dir, nil)
	now := time.Now()
	seedUploadAt(t, db, room.ID, user.ID, "old", "disk", now.Add(-72*time.Hour))
	freshID := seedUploadAt(t, db, room.ID, user.ID, "fresh", "disk", now.Add(-1*time.Hour))
	oldPath := writeUploadFile(t, dir, room.ID, user.ID, "old")

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
	assertUploadIDs(t, db, freshID)
}

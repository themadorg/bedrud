package testutil

import (
	"bedrud/internal/database"
	"bedrud/internal/models"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB creates an in-memory SQLite database for testing
// with all required tables migrated
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// Run migrations
	err = db.AutoMigrate(
		&models.User{},
		&models.BlockedRefreshToken{},
		&models.Room{},
		&models.RoomParticipant{},
		&models.RoomPermissions{},
		&models.Passkey{},
		&models.SystemSettings{},
		&models.InviteToken{},
		&models.UserPreferences{},
		&models.ChatUpload{},
		&models.Job{},
		&models.Webhook{},
		&models.Recording{},
	)
	if err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	// SQLite :memory: is per-connection. Limit to 1 to prevent pool from
	// opening a new connection that sees a fresh (empty) database.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Register as global database for handlers that call database.GetDB()
	database.SetForTest(db)

	return db
}

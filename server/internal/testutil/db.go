package testutil

import (
	"os"
	"testing"

	"bedrud/internal/database"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// SetupTestDB creates an in-memory SQLite database for testing, built by the
// same code path production uses (database.RunMigrations).
//
// Tests deliberately do not keep their own AutoMigrate list. A private list
// drifts from RunMigrations silently, so a schema change can pass every test
// while breaking a real deployment — and index/constraint work that lives only
// in RunMigrations (such as idx_rooms_active_name) never gets exercised at all.
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if os.Getenv("BEDRUD_SKIP_MIGRATE") == "1" {
		t.Fatal("BEDRUD_SKIP_MIGRATE=1 is set: test databases would be created with no schema. Unset it before running tests.")
	}

	db, err := gorm.Open(sqlite.Open(":memory:"), database.GormConfig(logger.Silent))
	if err != nil {
		t.Fatalf("failed to open test database: %v", err)
	}

	// SQLite :memory: is per-connection. Limit the pool to 1 *before* migrating
	// so the schema and every later query share one connection; otherwise the
	// pool can hand out a fresh (empty) database.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	// Register as the global handle first: RunMigrations resolves its target
	// through database.GetDB(), as do handlers under test.
	database.SetForTest(db)

	// Without this the global handle outlives the test, pointing at a closed
	// database, so a later test that forgets to set one up fails obscurely
	// instead of at the point of the mistake.
	t.Cleanup(database.ResetForTest)

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("failed to migrate test database: %v", err)
	}

	return db
}

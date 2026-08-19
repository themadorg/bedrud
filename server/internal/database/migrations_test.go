package database

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"bedrud/internal/models"

	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// dialect pairs a name with a way to open an empty database registered as the
// global handle. RunMigrations branches on the dialect in four places, so the
// tests covering those branches run against both engines. Production defaults
// to Postgres; without this the suite would only ever see SQLite.
type dialect struct {
	name string
	open func(t *testing.T) *gorm.DB
}

func dialects() []dialect {
	return []dialect{
		{DBTypeSQLite, newMemoryDB},
		{DBTypePostgres, newPostgresDB},
	}
}

func newMemoryDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("open in-memory sqlite: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying sql.DB: %v", err)
	}
	sqlDB.SetMaxOpenConns(1) // :memory: is per-connection

	SetForTest(db)
	t.Cleanup(ResetForTest)
	return db
}

// newPostgresDB gives each test a private schema on a shared server, so tests
// can run in any order without seeing tables left by another.
//
// Skips when BEDRUD_TEST_POSTGRES_DSN is unset so `go test ./...` still works
// on a machine with no Postgres. CI sets it, and the skip is visible there.
func newPostgresDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := os.Getenv("BEDRUD_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("BEDRUD_TEST_POSTGRES_DSN not set — skipping Postgres dialect coverage")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("connect to test postgres: %v", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("underlying sql.DB: %v", err)
	}
	// search_path is per-connection; a single connection keeps every statement
	// inside this test's schema.
	sqlDB.SetMaxOpenConns(1)

	schema := fmt.Sprintf("bedrud_test_%d", time.Now().UnixNano())
	if err := db.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	if err := db.Exec("SET search_path TO " + schema).Error; err != nil {
		t.Fatalf("set search_path: %v", err)
	}

	SetForTest(db)
	t.Cleanup(func() {
		if err := db.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Logf("drop test schema %s: %v", schema, err)
		}
		ResetForTest()
	})
	return db
}

// roomNameIndexIsUnique reports whether idx_rooms_name is currently UNIQUE.
// A missing index reads as "not unique"; callers pair this with HasIndex to
// tell the two apart. The Postgres query deliberately differs from the one in
// RunMigrations, so this is an independent check rather than a restatement of
// the code it verifies.
func roomNameIndexIsUnique(t *testing.T, db *gorm.DB) bool {
	t.Helper()

	switch db.Dialector.Name() {
	case DBTypeSQLite:
		rows, err := db.Raw("PRAGMA index_list('rooms')").Rows()
		if err != nil {
			t.Fatalf("PRAGMA index_list: %v", err)
		}
		defer rows.Close()

		for rows.Next() {
			var seq, unique int
			var name, origin, partial string
			if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
				t.Fatalf("scan index_list row: %v", err)
			}
			if name == "idx_rooms_name" {
				return unique == 1
			}
		}
		return false

	case DBTypePostgres:
		var isUnique bool
		err := db.Raw(`
			SELECT i.indisunique
			FROM pg_class c
			JOIN pg_index i ON i.indexrelid = c.oid
			WHERE c.relname = 'idx_rooms_name'
			  AND c.relnamespace = current_schema()::regnamespace
		`).Scan(&isUnique).Error
		if err != nil {
			t.Fatalf("query pg_index: %v", err)
		}
		return isUnique

	default:
		t.Fatalf("unsupported dialect %q", db.Dialector.Name())
		return false
	}
}

// assertInactive guards the fixture itself: a silently-active "archived" room
// would make these tests pass or fail for the wrong reason.
func assertInactive(t *testing.T, db *gorm.DB, id string) {
	t.Helper()

	var isActive bool
	if err := db.Raw("SELECT is_active FROM rooms WHERE id = ?", id).Scan(&isActive).Error; err != nil {
		t.Fatalf("read is_active for %q: %v", id, err)
	}
	if isActive {
		t.Fatalf("fixture %q was stored as active; the archived-room setup is not doing what it claims", id)
	}
}

// seedArchivedRoom inserts a room in the archived state: not active, soft-deleted.
//
// It inserts under a scratch name and then deactivates and renames, which is
// the order production follows when archiving. Building it in one Create does
// not work: IsActive=false is a zero value, so GORM omits the column and the
// default:true tag silently stores the room as active. Updates with a map
// writes zero values, so the state is unambiguous.
func seedArchivedRoom(t *testing.T, db *gorm.DB, id, name string) {
	t.Helper()

	scratch := newRoom(id + "-scratch")
	scratch.ID = id
	if err := db.Create(scratch).Error; err != nil {
		t.Fatalf("seed room %q: %v", id, err)
	}

	archivedAt := time.Now().Add(-time.Hour)
	if err := db.Model(&models.Room{}).Where("id = ?", id).Updates(map[string]any{
		"is_active":  false,
		"deleted_at": archivedAt,
		"name":       name,
	}).Error; err != nil {
		t.Fatalf("archive room %q: %v", id, err)
	}

	assertInactive(t, db, id)
}

// seedOwner creates the user that room fixtures point at. On Postgres
// RunMigrations adds fk_rooms_created_by and fk_rooms_admin_id, so a room with
// a dangling owner is rejected there while SQLite accepts it — the kind of gap
// that only shows up once both dialects are covered.
func seedOwner(t *testing.T, db *gorm.DB) {
	t.Helper()

	owner := &models.User{
		ID:    "user-1",
		Email: "owner@example.test",
		Name:  "Room Owner",
	}
	if err := db.Create(owner).Error; err != nil {
		t.Fatalf("seed owner user: %v", err)
	}
}

// newRoom builds an active room owned by seedOwner's user. Archived fixtures
// must go through seedArchivedRoom — see the note there about zero values and
// column defaults.
func newRoom(name string) *models.Room {
	return &models.Room{
		ID:              name + "-id",
		Name:            name,
		CreatedBy:       "user-1",
		AdminID:         "user-1",
		IsActive:        true,
		MaxParticipants: 20,
		Mode:            "standard",
	}
}

func TestRunMigrations_FreshDatabaseCreatesFullSchema(t *testing.T) {
	for _, d := range dialects() {
		t.Run(d.name, func(t *testing.T) {
			db := d.open(t)

			if err := RunMigrations(); err != nil {
				t.Fatalf("RunMigrations on an empty database: %v", err)
			}

			// Every model RunMigrations is responsible for. Compared by model
			// type rather than table name so GORM naming stays the source of
			// truth.
			want := []any{
				&models.User{}, &models.BlockedRefreshToken{}, &models.BlockedAccessToken{},
				&models.Room{}, &models.RoomParticipant{}, &models.RoomPermissions{},
				&models.Passkey{}, &models.SystemSettings{}, &models.InviteToken{},
				&models.UserPreferences{}, &models.ChatUpload{}, &models.Job{},
				&models.VerificationEvent{}, &models.Webhook{}, &models.WebxdcPackage{},
				&models.WebxdcInstance{}, &models.WebxdcStatusUpdate{}, &models.Recording{},
			}
			for _, model := range want {
				if !db.Migrator().HasTable(model) {
					t.Errorf("table for %T missing after RunMigrations", model)
				}
			}

			// Created by RunMigrations alone — no model tag produces it, so a
			// test database built from a private AutoMigrate list would not
			// have it.
			if !db.Migrator().HasIndex(&models.Room{}, "idx_rooms_active_name") {
				t.Error("idx_rooms_active_name missing: active room name uniqueness is not enforced at the database level")
			}
		})
	}
}

func TestRunMigrations_IsIdempotent(t *testing.T) {
	for _, d := range dialects() {
		t.Run(d.name, func(t *testing.T) {
			db := d.open(t)

			if err := RunMigrations(); err != nil {
				t.Fatalf("first RunMigrations: %v", err)
			}
			// Every service restart re-runs migrations against an already-migrated database.
			if err := RunMigrations(); err != nil {
				t.Fatalf("second RunMigrations: %v", err)
			}

			if !db.Migrator().HasIndex(&models.Room{}, "idx_rooms_active_name") {
				t.Error("idx_rooms_active_name lost on the second run")
			}
			if roomNameIndexIsUnique(t, db) {
				t.Error("idx_rooms_name became UNIQUE on the second run")
			}
		})
	}
}

// The upgrade path this migration exists for: older deployments carried a
// UNIQUE index on rooms.name, so an archived room held its name forever.
func TestRunMigrations_ReplacesLegacyUniqueRoomNameIndex(t *testing.T) {
	for _, d := range dialects() {
		t.Run(d.name, func(t *testing.T) {
			db := d.open(t)

			// Rebuild the pre-upgrade schema. users comes along because the
			// room owner has to exist before RunMigrations adds the owner
			// foreign keys on Postgres; without it those ALTER TABLEs fail
			// (only as a warning) and the test would pass for the wrong reason.
			if err := db.AutoMigrate(&models.User{}, &models.Room{}); err != nil {
				t.Fatalf("build legacy schema: %v", err)
			}
			seedOwner(t, db)
			if err := db.Migrator().DropIndex(&models.Room{}, "idx_rooms_name"); err != nil {
				t.Fatalf("drop generated index: %v", err)
			}
			if err := db.Exec("CREATE UNIQUE INDEX idx_rooms_name ON rooms(name)").Error; err != nil {
				t.Fatalf("create legacy unique index: %v", err)
			}
			if !roomNameIndexIsUnique(t, db) {
				t.Fatal("setup failed: idx_rooms_name is not UNIQUE, so this test would pass vacuously")
			}

			// An archived room sitting on the contested name.
			seedArchivedRoom(t, db, "standup-id", "standup")

			if err := RunMigrations(); err != nil {
				t.Fatalf("RunMigrations over the legacy schema: %v", err)
			}

			if roomNameIndexIsUnique(t, db) {
				t.Error("idx_rooms_name is still UNIQUE; an archived room's name can never be reused")
			}
			if !db.Migrator().HasIndex(&models.Room{}, "idx_rooms_name") {
				t.Error("idx_rooms_name was dropped without being recreated as a regular index")
			}

			// The behaviour the whole migration is for.
			if err := db.Create(newRoom("standup-new")).Error; err != nil {
				t.Fatalf("unrelated room should still be creatable: %v", err)
			}
			reused := newRoom("standup")
			reused.ID = "standup-reused"
			if err := db.Create(reused).Error; err != nil {
				t.Fatalf("an archived room name should be reusable after migration: %v", err)
			}
		})
	}
}

// idx_rooms_active_name is the database-level half of active-name uniqueness;
// the repository enforces the same rule in application code.
func TestRunMigrations_PartialIndexRejectsDuplicateActiveRoomNames(t *testing.T) {
	for _, d := range dialects() {
		t.Run(d.name, func(t *testing.T) {
			db := d.open(t)

			if err := RunMigrations(); err != nil {
				t.Fatalf("RunMigrations: %v", err)
			}
			seedOwner(t, db)

			if err := db.Create(newRoom("standup")).Error; err != nil {
				t.Fatalf("first active room: %v", err)
			}

			duplicate := newRoom("standup")
			duplicate.ID = "standup-duplicate"
			if err := db.Create(duplicate).Error; err == nil {
				t.Error("a second active room reused the name; idx_rooms_active_name is not enforcing uniqueness")
			}

			// The same name is fine once the holder is no longer active.
			seedArchivedRoom(t, db, "standup-archived", "standup")
		})
	}
}

func TestRunMigrations_SkipEnvVarLeavesSchemaUntouched(t *testing.T) {
	db := newMemoryDB(t)
	t.Setenv("BEDRUD_SKIP_MIGRATE", "1")

	if err := RunMigrations(); err != nil {
		t.Fatalf("RunMigrations with BEDRUD_SKIP_MIGRATE=1 should be a no-op, got: %v", err)
	}
	if db.Migrator().HasTable(&models.User{}) {
		t.Error("BEDRUD_SKIP_MIGRATE=1 still created tables")
	}
}

func TestRunMigrations_UninitializedDatabase(t *testing.T) {
	ResetForTest()
	t.Cleanup(ResetForTest)

	err := RunMigrations()
	if err == nil {
		t.Fatal("expected an error when the database handle is nil")
	}
	if !strings.Contains(err.Error(), "not initialized") {
		t.Errorf("error should name the missing initialization step, got: %v", err)
	}
}

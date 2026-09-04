// Package pgtest opens Postgres handles for tests that need the real dialect.
//
// It deliberately imports nothing from bedrud. internal/testutil imports
// internal/database, and internal/database's own tests are in package database,
// so a helper living in either of those cannot be shared with both. Keeping
// this a leaf package lets internal/database, internal/repository and anything
// else use one implementation of the schema-isolation logic below.
package pgtest

import (
	"fmt"
	"os"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DSNEnv and RequiredEnv name the variables CI sets for the Postgres service.
const (
	DSNEnv      = "BEDRUD_TEST_POSTGRES_DSN"
	RequiredEnv = "BEDRUD_TEST_POSTGRES_REQUIRED"
)

// Open builds a handle pointed at a schema of its own, or skips the test when
// no DSN is configured. The gorm.Config is a parameter because callers differ:
// most want the production config, and one test deliberately opens with a
// configuration production never uses.
//
// With BEDRUD_TEST_POSTGRES_REQUIRED=1 a missing DSN fails instead of skipping,
// so a misconfigured CI job cannot hide the whole Postgres dialect behind a
// green tick — go test does not print skips without -v.
func Open(t *testing.T, cfg *gorm.Config) *gorm.DB {
	t.Helper()

	dsn := os.Getenv(DSNEnv)
	if dsn == "" {
		// Read as ==1 to match how BEDRUD_SKIP_MIGRATE is read in migrations.go,
		// so REQUIRED=0 does not mean "required".
		if os.Getenv(RequiredEnv) == "1" {
			t.Fatalf("%s=1 but %s is empty: Postgres coverage would be skipped silently", RequiredEnv, DSNEnv)
		}
		t.Skipf("%s not set — skipping Postgres dialect coverage", DSNEnv)
	}

	// The schema has to exist before any connection that selects it, so create
	// it over the base DSN first, then hand every later connection a DSN that
	// already points at it.
	//
	// Setting search_path on one connection and pinning the pool to a single
	// connection is not enough: database/sql discards a connection on
	// driver.ErrBadConn and silently opens a fresh one, which lands in public.
	// Tables would leak into the shared schema and survive the DROP below.
	//
	// The admin handle only issues CREATE/DROP SCHEMA, so it takes a plain
	// silent config rather than the caller's.
	admin, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("connect to test postgres: %v", err)
	}
	schema := fmt.Sprintf("bedrud_test_%d", time.Now().UnixNano())
	if err := admin.Exec("CREATE SCHEMA " + schema).Error; err != nil {
		t.Fatalf("create test schema: %v", err)
	}

	db, err := gorm.Open(postgres.Open(dsn+" search_path="+schema), cfg)
	if err != nil {
		t.Fatalf("connect to test schema: %v", err)
	}

	t.Cleanup(func() {
		if sqlDB, err := db.DB(); err == nil {
			_ = sqlDB.Close()
		}
		if err := admin.Exec("DROP SCHEMA " + schema + " CASCADE").Error; err != nil {
			t.Logf("drop test schema %s: %v", schema, err)
		}
		if sqlDB, err := admin.DB(); err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// OpenWith is Open with extra space-separated settings appended to the DSN.
//
// The one setting that matters so far is TimeZone: it pins the *session*
// timezone, which is what PostgreSQL uses to resolve a date literal that
// carries no offset. Tests covering that resolution have to pin it rather than
// inherit whatever the local server is configured with, or they pass or fail by
// the hour.
//
// It does not control how the driver renders a value on the way back — that
// follows the local zone of the test process — so it is not a lever for
// anything on the read path.
//
// An unset DSN is left untouched so Open still skips (or fails under
// BEDRUD_TEST_POSTGRES_REQUIRED) rather than trying to dial a bare setting.
func OpenWith(t *testing.T, cfg *gorm.Config, extraDSN string) *gorm.DB {
	t.Helper()

	if base := os.Getenv(DSNEnv); base != "" && extraDSN != "" {
		t.Setenv(DSNEnv, base+" "+extraDSN)
	}
	return Open(t, cfg)
}

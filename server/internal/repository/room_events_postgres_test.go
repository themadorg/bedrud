package repository

import (
	"testing"
	"time"

	"bedrud/internal/database"
	"bedrud/internal/models"
	"bedrud/internal/testutil/pgtest"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// The rest of the repository suite runs on SQLite, which hides anything
// dialect-specific. GetRoomEventsFiltered built its dateTo clause from
// date(x, '+1 day'), a SQLite builtin that Postgres answers with "function
// date(unknown, unknown) does not exist" — so the endpoint returned 500 on
// every Postgres deployment while the SQLite suite stayed green.
//
// This file is deliberately narrow. It is not a port of the suite; it is the
// place the next dialect-specific bug lands.

func newPostgresRoomRepo(t *testing.T) (*RoomRepository, *gorm.DB) {
	t.Helper()

	db := pgtest.Open(t, database.GormConfig(logger.Silent))
	database.SetForTest(db)
	t.Cleanup(database.ResetForTest)

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("migrate postgres schema: %v", err)
	}
	return NewRoomRepository(db), db
}

// seedPostgresRoomEvents mirrors the SQLite seed in the handler tests: two
// rooms, each auto-joining its creator, plus one explicit join. Five events.
func seedPostgresRoomEvents(t *testing.T, repo *RoomRepository, db *gorm.DB) {
	t.Helper()

	for _, u := range []*models.User{
		{ID: "pg-owner", Email: "pg-owner@ex.com", Name: "PgOwner", Provider: "local", IsActive: true},
		{ID: "pg-joiner", Email: "pg-joiner@ex.com", Name: "PgJoiner", Provider: "local", IsActive: true},
	} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user %s: %v", u.ID, err)
		}
	}

	r1, err := repo.CreateRoom("pg-owner", "meeting-room", true, "standard", 10, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create meeting-room: %v", err)
	}
	if _, err := repo.CreateRoom("pg-owner", "call-room", true, "standard", 20, &models.RoomSettings{}); err != nil {
		t.Fatalf("create call-room: %v", err)
	}
	if err := repo.AddParticipant(r1.ID, "pg-joiner"); err != nil {
		t.Fatalf("add participant: %v", err)
	}
}

// TestPostgres_RoomEvents_DateToIsNotSQLiteOnly is the reason this file exists.
// The pre-fix dateTo clause did not merely return the wrong rows on Postgres —
// it failed to execute, so the handler answered 500 and every SQLite test that
// expected "zero events" was satisfied by the empty body.
func TestPostgres_RoomEvents_DateToIsNotSQLiteOnly(t *testing.T) {
	repo, db := newPostgresRoomRepo(t)
	seedPostgresRoomEvents(t, repo, db)

	today := time.Now().Format("2006-01-02")

	events, total, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{DateTo: today})
	if err != nil {
		t.Fatalf("dateTo query failed on Postgres: %v", err)
	}
	// dateTo is inclusive of its own day; every seeded event is from today or
	// earlier, so all five are in range.
	if len(events) != 5 {
		t.Fatalf("expected all 5 events through today, got %d", len(events))
	}
	if int(total) != len(events) {
		t.Fatalf("total %d disagrees with %d events on one page", total, len(events))
	}

	past, pastTotal, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{DateTo: "2010-01-01"})
	if err != nil {
		t.Fatalf("dateTo query failed on Postgres: %v", err)
	}
	if len(past) != 0 || pastTotal != 0 {
		t.Fatalf("expected nothing before 2010, got %d events and total %d", len(past), pastTotal)
	}
}

// TestPostgres_RoomEvents_TotalMatchesPage runs the SQLite assertion against the
// other dialect, so the count and the page cannot drift apart on one engine
// while agreeing on the other.
func TestPostgres_RoomEvents_TotalMatchesPage(t *testing.T) {
	repo, db := newPostgresRoomRepo(t)
	seedPostgresRoomEvents(t, repo, db)

	events, total, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{Search: "meeting"})
	if err != nil {
		t.Fatalf("search query failed: %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("expected 3 events matching 'meeting', got %d", len(events))
	}
	if int(total) != len(events) {
		t.Fatalf("total %d disagrees with %d events on one page", total, len(events))
	}
}

// TestPostgres_RoomEvents_TimestampsParse is an end-to-end check that the value
// this driver actually produces survives parseDBTime.
//
// It is not the primary guard for the layouts. What database/sql renders — and
// so whether the "Z" form appears at all — follows the local zone of the
// process running the test, so this reproduces the original bug on a UTC runner
// (CI, containers) but not on a developer machine elsewhere.
// TestParseDBTime_DialectLayouts pins the shapes themselves and fails in every
// zone; this one proves the wiring in between.
func TestPostgres_RoomEvents_TimestampsParse(t *testing.T) {
	repo, db := newPostgresRoomRepo(t)
	seedPostgresRoomEvents(t, repo, db)

	events, _, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{})
	if err != nil {
		t.Fatalf("list events: %v", err)
	}
	if len(events) == 0 {
		t.Fatal("expected seeded events")
	}
	for i, ev := range events {
		if ev.Timestamp.IsZero() {
			t.Fatalf("event %d (%s on %q) came back with a zero timestamp", i, ev.Type, ev.RoomName)
		}
	}

	recent, err := repo.GetRecentRoomEvents(10)
	if err != nil {
		t.Fatalf("recent events: %v", err)
	}
	for i, ev := range recent {
		if ev.Timestamp.IsZero() {
			t.Fatalf("recent event %d (%s on %q) came back with a zero timestamp", i, ev.Type, ev.RoomName)
		}
	}
}

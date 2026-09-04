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
	return newPostgresRoomRepoTZ(t, "")
}

// newPostgresRoomRepoTZ pins the server's session timezone when one is given.
// PostgreSQL resolves a date literal with no offset in that zone, so any test
// about the date bounds has to state it rather than inherit whichever zone the
// local server happens to run in.
func newPostgresRoomRepoTZ(t *testing.T, tz string) (*RoomRepository, *gorm.DB) {
	t.Helper()

	extra := ""
	if tz != "" {
		extra = "TimeZone=" + tz
	}
	db := pgtest.OpenWith(t, database.GormConfig(logger.Silent), extra)
	database.SetForTest(db)
	t.Cleanup(database.ResetForTest)

	if err := database.RunMigrations(); err != nil {
		t.Fatalf("migrate postgres schema: %v", err)
	}
	return NewRoomRepository(db), db
}

// seedDay is the UTC day every seeded event belongs to. Fixed rather than
// derived from time.Now(): a seed at "now" plus a bound at "today" makes the
// result depend on the hour the suite happens to run, which is how the first
// version of this file came to fail only in the last hours of a UTC day.
var seedDay = time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)

// seedPostgresRoomEvents mirrors the SQLite seed in the handler tests: two
// rooms, each auto-joining its creator, plus one explicit join. Five events.
//
// Every timestamp is then rewritten to a fixed instant inside seedDay: rooms
// early at 00:30Z, joins late at 23:59Z. Both ends matter, and they fail in
// opposite directions — a session east of UTC pulls the upper bound back before
// 23:59Z, a session west of UTC pushes the lower bound past 00:30Z, so one
// instant alone would catch only half of it.
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

	earlyInDay := seedDay.Add(30 * time.Minute)
	lateInDay := seedDay.Add(23*time.Hour + 59*time.Minute)
	if err := db.Exec("UPDATE rooms SET created_at = ?", earlyInDay).Error; err != nil {
		t.Fatalf("pin room timestamps: %v", err)
	}
	if err := db.Exec("UPDATE room_participants SET joined_at = ?", lateInDay).Error; err != nil {
		t.Fatalf("pin participant timestamps: %v", err)
	}
}

func seedDayString() string { return seedDay.Format("2006-01-02") }

// TestPostgres_RoomEvents_DateToIsNotSQLiteOnly is the reason this file exists.
// The pre-fix dateTo clause did not merely return the wrong rows on Postgres —
// it failed to execute, so the handler answered 500 and every SQLite test that
// expected "zero events" was satisfied by the empty body.
func TestPostgres_RoomEvents_DateToIsNotSQLiteOnly(t *testing.T) {
	repo, db := newPostgresRoomRepo(t)
	seedPostgresRoomEvents(t, repo, db)

	events, total, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{DateTo: seedDayString()})
	if err != nil {
		t.Fatalf("dateTo query failed on Postgres: %v", err)
	}
	// dateTo is inclusive of its own day, and every seeded event is inside it.
	if len(events) != 5 {
		t.Fatalf("expected all 5 events through %s, got %d", seedDayString(), len(events))
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

// TestPostgres_RoomEvents_DateBoundsIgnoreSessionTimezone is the reason the
// bounds are bound as instants rather than as bare date strings.
//
// PostgreSQL resolves a literal with no offset in the session's zone. With the
// session east of UTC, `timestamp < '2026-09-01'` means 2026-08-31 20:30Z, so a
// row at 23:59Z on 2026-08-31 — squarely inside the UTC day being asked for —
// falls outside the filter. The same shift moves the lower bound in the same
// direction, pulling in the tail of the previous day.
//
// A UTC session hides all of it, which is why this test states the zone instead
// of taking whatever the local server has, and why CI never saw it.
func TestPostgres_RoomEvents_DateBoundsIgnoreSessionTimezone(t *testing.T) {
	for _, tz := range []string{"UTC", "Asia/Tehran", "America/New_York"} {
		t.Run(tz, func(t *testing.T) {
			repo, db := newPostgresRoomRepoTZ(t, tz)
			seedPostgresRoomEvents(t, repo, db)

			day := seedDayString()

			// The upper bound: east of UTC this used to drop everything.
			within, total, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{
				DateFrom: day,
				DateTo:   day,
			})
			if err != nil {
				t.Fatalf("session %s: %v", tz, err)
			}
			if len(within) != 5 {
				t.Fatalf("session %s: expected all 5 events on %s, got %d", tz, day, len(within))
			}
			if int(total) != len(within) {
				t.Fatalf("session %s: total %d disagrees with %d events", tz, total, len(within))
			}

			// The lower bound: west of UTC the day before leaks in, so ask for
			// the day after and require nothing.
			next := seedDay.AddDate(0, 0, 1).Format("2006-01-02")
			after, afterTotal, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{DateFrom: next})
			if err != nil {
				t.Fatalf("session %s: %v", tz, err)
			}
			if len(after) != 0 || afterTotal != 0 {
				t.Fatalf("session %s: expected nothing from %s, got %d events and total %d",
					tz, next, len(after), afterTotal)
			}

			// And the day before must not reach forward into it.
			prev := seedDay.AddDate(0, 0, -1).Format("2006-01-02")
			before, beforeTotal, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{DateTo: prev})
			if err != nil {
				t.Fatalf("session %s: %v", tz, err)
			}
			if len(before) != 0 || beforeTotal != 0 {
				t.Fatalf("session %s: expected nothing through %s, got %d events and total %d",
					tz, prev, len(before), beforeTotal)
			}
		})
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

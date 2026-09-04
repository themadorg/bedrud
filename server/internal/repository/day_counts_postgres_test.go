package repository

import (
	"testing"
	"time"

	"bedrud/internal/models"

	"gorm.io/gorm"
)

// The daily-count series buckets rows in SQL, and the two dialects need
// different SQL for the same day. SQLite's DATE() normalises an offset-bearing
// value to UTC, ignores every zone setting, and returns text. PostgreSQL states
// nothing on its own: DATE(timestamptz) resolves in the *session* timezone, and
// the date it returns is rendered into a Go string by the session's DateStyle —
// so the Go-side axis, which is UTC and expects YYYY-MM-DD, has nothing to
// match on a server configured either way.
//
// Measured on 15.19. The zone, one instant across three sessions:
//
//	SET TimeZone='UTC';              DATE('2026-09-04 00:19:00+00') = 2026-09-04
//	SET TimeZone='America/New_York'; DATE('2026-09-04 00:19:00+00') = 2026-09-03
//	SET TimeZone='Asia/Tehran';      DATE('2026-09-03 21:30:00+00') = 2026-09-04
//
// And the layout, one value across three DateStyles:
//
//	ISO, MDY     (col AT TIME ZONE 'UTC')::date::text = 2026-09-01
//	SQL, DMY     (col AT TIME ZONE 'UTC')::date::text = 01/09/2026
//	German, DMY  (col AT TIME ZONE 'UTC')::date::text = 01.09.2026
//
// The SQLite suite cannot see any of this, and neither can a UTC runner with a
// default DateStyle.

// TestPostgres_DayCounts_BucketByUTCDayNotSessionZone seeds two instants that
// fail in opposite directions: 01:00Z, whose New York date is the day before,
// and 22:30Z, whose Tehran date is the day after. One alone would catch only
// half of it.
func TestPostgres_DayCounts_BucketByUTCDayNotSessionZone(t *testing.T) {
	for _, tz := range []string{"UTC", "America/New_York", "Asia/Tehran"} {
		t.Run(tz, func(t *testing.T) {
			repo, db := newPostgresRoomRepoTZ(t, tz)

			now := time.Now().UTC()
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
			// Early on one UTC day, late on another. Both sit inside a 7-day
			// window whichever hour the suite runs at.
			early := today.AddDate(0, 0, -2).Add(time.Hour)
			late := today.AddDate(0, 0, -3).Add(22*time.Hour + 30*time.Minute)

			seedPostgresDayCounts(t, repo, db, early, late)

			t.Run("rooms created", func(t *testing.T) {
				counts, err := repo.CountRoomsByDay(7)
				if err != nil {
					t.Fatalf("session %s: %v", tz, err)
				}
				assertDayCounts(t, counts, map[string]int{
					dayKey(early): 1,
					dayKey(late):  1,
				})
			})

			t.Run("active participants", func(t *testing.T) {
				counts, err := repo.CountActiveParticipantsByDay(7)
				if err != nil {
					t.Fatalf("session %s: %v", tz, err)
				}
				// Two distinct users joined on the early day, one on the late.
				assertDayCounts(t, counts, map[string]int{
					dayKey(early): 2,
					dayKey(late):  1,
				})
			})
		})
	}
}

// seedPostgresDayCounts puts one room and two joins on the early day and one
// room and one join on the late day. Postgres stores an instant, so unlike the
// SQLite seeds these writes do not depend on the zone the test process runs in.
func seedPostgresDayCounts(t *testing.T, repo *RoomRepository, db *gorm.DB, early, late time.Time) {
	t.Helper()

	for _, u := range []*models.User{
		{ID: "pgd-owner", Email: "pgd-owner@ex.com", Name: "PgdOwner", Provider: "local", IsActive: true},
		{ID: "pgd-joiner", Email: "pgd-joiner@ex.com", Name: "PgdJoiner", Provider: "local", IsActive: true},
	} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user %s: %v", u.ID, err)
		}
	}

	earlyRoom, err := repo.CreateRoom("pgd-owner", "early-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create early-room: %v", err)
	}
	lateRoom, err := repo.CreateRoom("pgd-owner", "late-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create late-room: %v", err)
	}
	if err := repo.AddParticipant(earlyRoom.ID, "pgd-joiner"); err != nil {
		t.Fatalf("add participant: %v", err)
	}

	for _, pin := range []struct {
		roomID string
		at     time.Time
	}{
		{earlyRoom.ID, early},
		{lateRoom.ID, late},
	} {
		if err := db.Exec("UPDATE rooms SET created_at = ? WHERE id = ?", pin.at, pin.roomID).Error; err != nil {
			t.Fatalf("pin room timestamp: %v", err)
		}
		if err := db.Exec("UPDATE room_participants SET joined_at = ? WHERE room_id = ?", pin.at, pin.roomID).Error; err != nil {
			t.Fatalf("pin participant timestamps: %v", err)
		}
	}
}

// assertDayCounts requires the series to cover the UTC days ending today and to
// carry want on those days and nothing anywhere else, so a series shifted by a
// day fails on the days it vacated as well as the ones it landed on.
func assertDayCounts(t *testing.T, counts []models.DayCount, want map[string]int) {
	t.Helper()

	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}
	days := wantDayKeys(len(counts))
	for i, c := range counts {
		key := dayKey(c.Date)
		if key != days[i] {
			t.Fatalf("day %d is %s, want %s (whole series: %s)", i, key, days[i], keysOf(counts))
		}
		if c.Count != want[key] {
			t.Fatalf("%s has %d, want %d (whole series: %s)", key, c.Count, want[key], keysOf(counts))
		}
	}
}

// TestPostgres_DayCounts_SurviveANonISODateStyle pins the other half of what
// the bucket expression has to state.
//
// A date reaching a Go string is rendered by the session's DateStyle. The
// default ISO gives "2026-09-01", which the "2006-01-02" layout parses; a
// database carrying ALTER DATABASE … SET DateStyle = 'German, DMY' gives
// "01.09.2026", which it does not. The driver passes the setting through rather
// than pinning a style of its own, so this is a deployment's to change and the
// query's to survive.
func TestPostgres_DayCounts_SurviveANonISODateStyle(t *testing.T) {
	repo, db := newPostgresRoomRepoTZ(t, "UTC")

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	early := today.AddDate(0, 0, -2).Add(time.Hour)
	late := today.AddDate(0, 0, -3).Add(22*time.Hour + 30*time.Minute)
	seedPostgresDayCounts(t, repo, db, early, late)

	// Seeded first, then the pool is pinned to one connection: SET applies to
	// the session it runs on, and the pool would otherwise hand the query a
	// connection that never saw it.
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("sql handle: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)
	if err := db.Exec("SET DateStyle = 'German, DMY'").Error; err != nil {
		t.Fatalf("set DateStyle: %v", err)
	}
	var style string
	if err := db.Raw("SHOW DateStyle").Row().Scan(&style); err != nil {
		t.Fatalf("read DateStyle: %v", err)
	}
	if style != "German, DMY" {
		t.Fatalf("premise gone: the session reports DateStyle %q, so this test is no longer about anything", style)
	}

	counts, err := repo.CountRoomsByDay(7)
	if err != nil {
		t.Fatalf("DateStyle %s: %v", style, err)
	}
	assertDayCounts(t, counts, map[string]int{
		dayKey(early): 1,
		dayKey(late):  1,
	})
}

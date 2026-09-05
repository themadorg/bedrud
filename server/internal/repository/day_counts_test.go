package repository

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/testutil"
)

// The daily-count series is bucketed by SQL and zero-filled in Go, and the two
// halves have to agree on what a day is. The buckets are UTC calendar days:
// SQLite's DATE() normalises an offset-bearing value to UTC and never consults
// the process zone, and the Postgres side is pinned to UTC explicitly (see
// day_counts_postgres_test.go). These tests pin the Go half — the axis the
// series is emitted on — which used to be built from a local time.Time and so
// shifted a whole day away from the buckets whenever the two calendars
// disagreed.
//
// The tests here state their expected days in UTC without calling the code
// under test, so they fail in every process zone rather than only in the
// offset-sized window that first exposed the bug.

// dayKey formats a day the way the series keys it, and the way the overview
// handler reads it back: in the value's own location, not converted first.
func dayKey(t time.Time) string { return t.Format("2006-01-02") }

// wantDayKeys is the window a days-long series should cover, oldest first: the
// UTC day containing now, and the days-1 UTC days before it.
func wantDayKeys(days int) []string {
	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	keys := make([]string, days)
	for i := range keys {
		keys[i] = dayKey(today.AddDate(0, 0, -(days - 1 - i)))
	}
	return keys
}

// TestDayCounts_SeriesCoversUTCDaysEndingToday covers both halves of the axis
// defect at once.
//
// The emitted days were local, so west of UTC between 00:00Z and the offset the
// whole series named yesterday's dates while the buckets were keyed by today's,
// every lookup missed, and the admin charts read zero. And the range ended a day
// early regardless of zone: rows created today were counted by the query and
// then dropped by the fill, so the last column was always yesterday.
//
// The seed goes through the production paths and is left at "now", so the rows
// carry whatever zone the process runs in — what a live server actually writes.
func TestDayCounts_SeriesCoversUTCDaysEndingToday(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := NewRoomRepository(db)
	userRepo := NewUserRepository(db)

	if err := db.Create(&models.User{
		ID: "dc-owner", Email: "dc-owner@ex.com", Name: "DcOwner",
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	// CreateRoom also joins the creator, which is what makes the two
	// participant series non-zero today.
	if _, err := roomRepo.CreateRoom("dc-owner", "dc-room", true, "standard", 0, &models.RoomSettings{}); err != nil {
		t.Fatalf("create room: %v", err)
	}

	series := []struct {
		name  string
		fetch func(int) ([]models.DayCount, error)
	}{
		{"rooms created", roomRepo.CountRoomsByDay},
		{"active participants", roomRepo.CountActiveParticipantsByDay},
		{"active rooms", roomRepo.CountActiveRoomsByDay},
		{"users", userRepo.CountUsersByDay},
	}

	want := wantDayKeys(7)
	for _, s := range series {
		t.Run(s.name, func(t *testing.T) {
			counts, err := s.fetch(7)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(counts) != len(want) {
				t.Fatalf("expected %d days, got %d", len(want), len(counts))
			}
			for i, c := range counts {
				if got := dayKey(c.Date); got != want[i] {
					t.Fatalf("day %d is %s, want %s (whole series: %s)", i, got, want[i], keysOf(counts))
				}
			}
			// The seeded row is today's, so the last bucket carries it. A zero
			// here with the days right means the lookup key and the SQL bucket
			// are still in different key spaces.
			if last := counts[len(counts)-1]; last.Count != 1 {
				t.Fatalf("expected 1 on %s, got %d", dayKey(last.Date), last.Count)
			}
		})
	}
}

// TestDayCounts_FirstBucketKeepsRowsStoredWestOfUTC pins the lower bound of the
// query against the storage format, which is not the same question as the axis.
//
// The bound is a UTC instant; on SQLite the column is text and the comparison is
// lexicographic, and the driver wrote each row with the offset its value carried
// (#126). A row at 01:00Z, written by a process four hours west, is stored as
// the previous day's 21:00 and sorts before a bound placed at the window's own
// start — even though its UTC day is the first day of the window and the series
// is about to display it.
func TestDayCounts_FirstBucketKeepsRowsStoredWestOfUTC(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	now := time.Now().UTC()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	start := today.AddDate(0, 0, -6)
	instant := start.Add(time.Hour)
	west := time.FixedZone("-0400", -4*3600)

	if err := db.Create(&models.User{
		ID: "dcw-owner", Email: "dcw-owner@ex.com", Name: "DcwOwner",
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	room, err := repo.CreateRoom("dcw-owner", "dcw-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	if err := db.Model(&models.Room{}).Where("id = ?", room.ID).
		Update("created_at", instant.In(west)).Error; err != nil {
		t.Fatalf("backdate room: %v", err)
	}

	// State the premise rather than assume it: what the driver stored has to
	// sort before the window start for this test to be about anything. CAST is
	// needed because the driver parses the text back into a time.Time on read
	// and hides the offset it actually wrote.
	var stored string
	if err := db.Raw("SELECT CAST(created_at AS TEXT) FROM rooms WHERE id = ?", room.ID).
		Row().Scan(&stored); err != nil {
		t.Fatalf("read stored timestamp: %v", err)
	}
	bound := start.Format("2006-01-02 15:04:05.999999999-07:00")
	if stored >= bound {
		t.Fatalf("premise gone: stored %q no longer sorts before the window start %q — "+
			"if writes have been normalised to UTC (#126) this test needs rewriting", stored, bound)
	}

	counts, err := repo.CountRoomsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}
	first := counts[0]
	if got, want := dayKey(first.Date), dayKey(start); got != want {
		t.Fatalf("first day is %s, want %s (whole series: %s)", got, want, keysOf(counts))
	}
	if first.Count != 1 {
		t.Fatalf("expected the 01:00Z room on %s, got %d", dayKey(first.Date), first.Count)
	}
}

// keysOf renders a series for a failure message as day=count pairs: both halves
// of a mismatch — the days emitted and where the counts landed — are the thing
// under test.
func keysOf(counts []models.DayCount) string {
	out := ""
	for i, c := range counts {
		if i > 0 {
			out += " "
		}
		out += fmt.Sprintf("%s=%d", dayKey(c.Date), c.Count)
	}
	return out
}

// TestDayWindowStart_IsUTCMidnightEndingOnTodaysUTCDay states the window rule
// on its own, away from any database, so both halves of it — the zone the axis
// is built in and the day it ends on — are pinned by something that cannot pass
// for the wrong reason.
//
// The cases give each instant an explicit location rather than moving the
// process: time.Local is a global resolved once at startup, t.Setenv("TZ", …)
// does not touch it, and assigning it races every other test under -race.
func TestDayWindowStart_IsUTCMidnightEndingOnTodaysUTCDay(t *testing.T) {
	west := time.FixedZone("-0400", -4*3600)
	east := time.FixedZone("+0330", 3*3600+1800)

	cases := []struct {
		name string
		now  time.Time
		days int
		want string
	}{
		// The instant #130 was filed at: 00:19Z, still the 3rd in New York.
		// A local axis names 2026-08-27..2026-09-02 here and misses the day
		// every bucket is keyed by.
		{"west of UTC, before the local date catches up", time.Date(2026, 9, 4, 0, 19, 0, 0, time.UTC).In(west), 7, "2026-08-29"},
		// The mirror: 21:30Z is already the 4th in Tehran, so a local axis
		// runs a day ahead instead.
		{"east of UTC, after the local date has moved on", time.Date(2026, 9, 3, 21, 30, 0, 0, time.UTC).In(east), 7, "2026-08-28"},
		{"already UTC", time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC), 7, "2026-08-29"},
		{"a single day is today", time.Date(2026, 9, 4, 12, 0, 0, 0, time.UTC), 1, "2026-09-04"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := dayWindowStart(tc.now, tc.days)

			if got.Location() != time.UTC {
				t.Fatalf("start is in %s, want UTC", got.Location())
			}
			if h, m, s := got.Clock(); h != 0 || m != 0 || s != 0 || got.Nanosecond() != 0 {
				t.Fatalf("start is %s, want midnight", got.Format(time.RFC3339Nano))
			}
			if key := dayKey(got); key != tc.want {
				t.Fatalf("window opens on %s, want %s", key, tc.want)
			}
			// The other end is the point: the window has to reach today, not
			// stop at yesterday and drop the rows the query already counted.
			last := got.AddDate(0, 0, tc.days-1)
			if key, today := dayKey(last), tc.now.UTC().Format("2006-01-02"); key != today {
				t.Fatalf("window closes on %s, want today's UTC day %s", key, today)
			}
		})
	}
}

// TestDayCounts_OneUnreadableRowDoesNotSinkTheSeries separates a bad row from a
// bad query.
//
// SQLite's column is text and its DATE() answers NULL for a value it cannot
// read as a date, which GORM scans into the zero string. That is one bucket's
// worth of unreadable rows; the rest of the window is still answerable, and the
// dashboard should show it rather than fail whole. A day that arrives non-empty
// and still will not parse is the other case — the query rendering something
// other than a day on this dialect — and that one stays an error.
func TestDayCounts_OneUnreadableRowDoesNotSinkTheSeries(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	if err := db.Create(&models.User{
		ID: "dcu-owner", Email: "dcu-owner@ex.com", Name: "DcuOwner",
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := repo.CreateRoom("dcu-owner", "dcu-good", true, "standard", 0, &models.RoomSettings{}); err != nil {
		t.Fatalf("create good room: %v", err)
	}
	corrupt, err := repo.CreateRoom("dcu-owner", "dcu-corrupt", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create corrupt room: %v", err)
	}
	// Written straight past the driver, the way a hand-edited database or an
	// import from another tool would leave it.
	if err := db.Exec("UPDATE rooms SET created_at = 'garbage' WHERE id = ?", corrupt.ID).Error; err != nil {
		t.Fatalf("corrupt the timestamp: %v", err)
	}
	var stored string
	if err := db.Raw("SELECT CAST(created_at AS TEXT) FROM rooms WHERE id = ?", corrupt.ID).
		Row().Scan(&stored); err != nil {
		t.Fatalf("read stored timestamp: %v", err)
	}
	if stored != "garbage" {
		t.Fatalf("premise gone: the corrupt row reads back as %q", stored)
	}

	counts, err := repo.CountRoomsByDay(7)
	if err != nil {
		t.Fatalf("one unreadable row took down the whole series: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}
	total := 0
	for _, c := range counts {
		total += c.Count
	}
	if last := counts[len(counts)-1]; last.Count != 1 || total != 1 {
		t.Fatalf("expected the readable room on %s and nothing else, got %s",
			dayKey(last.Date), keysOf(counts))
	}
}

// TestParseDayCounts_SkipsTheUnreadableAndRejectsTheUnexpected states the
// distinction on its own, without a database in the way.
func TestParseDayCounts_SkipsTheUnreadableAndRejectsTheUnexpected(t *testing.T) {
	t.Run("empty day is one unreadable bucket", func(t *testing.T) {
		got, err := parseDayCounts([]dayCountRow{
			{Date: "2026-09-01", Count: 2},
			{Date: "", Count: 3},
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(got) != 1 || got[0].Count != 2 || dayKey(got[0].Date) != "2026-09-01" {
			t.Fatalf("expected only 2026-09-01=2, got %s", keysOf(got))
		}
	})

	// What a dialect rendering something other than a day looks like: this is
	// the shape Postgres hands back for a bare date, and every bucket in the
	// series would carry it.
	t.Run("a day that is not a day is a broken query", func(t *testing.T) {
		_, err := parseDayCounts([]dayCountRow{{Date: "2026-09-01T00:00:00Z", Count: 1}})
		if err == nil {
			t.Fatal("expected an error for a bucket that is not a plain day")
		}
		if !strings.Contains(err.Error(), "2026-09-01T00:00:00Z") {
			t.Fatalf("error does not name the value it rejected: %v", err)
		}
	})
}

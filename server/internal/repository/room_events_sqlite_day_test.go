package repository

import (
	"testing"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/testutil"
)

// Binding the date bounds as instants makes the filter mean the UTC day on
// Postgres, where the column holds an instant. SQLite is not there yet: the
// column is text, the comparison is lexicographic, and the driver wrote each
// row with whatever offset the value carried. So the filter selects the
// *writing process's local day*, and only coincides with the UTC day while
// that process runs in UTC.
//
// This test characterises that, rather than asserting it is right. It pins the
// gap the deployment docs describe, and it is the test that flips when writes
// are normalised to UTC — at which point the "written in a non-UTC zone" case
// below starts being found and the expectation inverts.
//
// The seed gives each value an explicit location instead of leaning on the
// process zone: time.Local is a process global resolved once at startup, so
// t.Setenv("TZ", …) cannot move it and assigning it races other tests.
func TestSQLite_RoomEvents_DayFollowsTheWritingZone(t *testing.T) {
	// One instant, late in the UTC day 2026-08-31.
	instant := time.Date(2026, 8, 31, 23, 59, 0, 0, time.UTC)
	tehran := time.FixedZone("+0330", 3*3600+1800)

	cases := []struct {
		name string
		// The zone the value carries when it is written, standing in for the
		// zone the server process would be running in.
		writtenIn *time.Location
		// Whether a request for the UTC day 2026-08-31 finds it.
		wantFound bool
	}{
		{"written in UTC", time.UTC, true},
		// Stored as "2026-09-01 03:29:00+03:30", which sorts past the upper
		// bound even though the instant is inside the day being asked for.
		{"written east of UTC", tehran, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := testutil.SetupTestDB(t)
			repo := NewRoomRepository(db)

			if err := db.Create(&models.User{
				ID: "sq-owner", Email: "sq-owner@ex.com", Name: "SqOwner",
				Provider: "local", IsActive: true,
			}).Error; err != nil {
				t.Fatalf("create user: %v", err)
			}
			room, err := repo.CreateRoom("sq-owner", "zone-room", true, "standard", 0, &models.RoomSettings{})
			if err != nil {
				t.Fatalf("create room: %v", err)
			}
			if err := db.Model(&models.Room{}).Where("id = ?", room.ID).
				Update("created_at", instant.In(tc.writtenIn)).Error; err != nil {
				t.Fatalf("pin created_at: %v", err)
			}

			day := "2026-08-31"
			events, total, err := repo.GetRoomEventsFiltered(&RoomEventsFilterParams{
				Types:    []string{"room_created"},
				DateFrom: day,
				DateTo:   day,
			})
			if err != nil {
				t.Fatalf("filter: %v", err)
			}

			found := len(events) == 1
			if found != tc.wantFound {
				t.Fatalf("row written in %s, asking for the UTC day %s: found=%v, want %v",
					tc.writtenIn, day, found, tc.wantFound)
			}
			if int(total) != len(events) {
				t.Fatalf("total %d disagrees with %d events on one page", total, len(events))
			}
		})
	}
}

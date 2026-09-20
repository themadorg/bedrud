package repository

import (
	"testing"
	"time"

	"bedrud/internal/models"
	"bedrud/internal/testutil"
)

// The row that separates the three admin date filters is the one stored at
// exactly midnight. Every other row is inside one day by a comfortable margin
// and cannot tell an inclusive bound from an exclusive one.
//
// The recent-signups list used `created_at <= dateTo + 24h`, inclusive of the
// following midnight, so a user created at 00:00:00.000000000 on 2026-03-02 was
// returned for dateTo=2026-03-01 *and* for dateTo=2026-03-02. Room events
// already used the exclusive form; the rooms list parsed RFC3339 and had no day
// extension at all.
//
// Seeded in UTC deliberately. This is about which side of a boundary a row
// falls on, not about how the driver stores a zone — that is #126, and mixing
// the two would make this test fail for a reason it is not asking about.
func TestDayFilters_MidnightRowBelongsToExactlyOneDay(t *testing.T) {
	midnight := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	dayBefore := "2026-03-01"
	itsOwnDay := "2026-03-02"

	t.Run("recent signups", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewUserRepository(db)

		if err := db.Create(&models.User{
			ID: "midnight-user", Email: "midnight@ex.com", Name: "Midnight",
			Provider: "local", IsActive: true, CreatedAt: midnight, UpdatedAt: midnight,
		}).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
		// Guard the premise: the row has to actually be stored at midnight, or
		// the boundary this test is about is not the one being exercised.
		var stored models.User
		if err := db.Where("id = ?", "midnight-user").First(&stored).Error; err != nil {
			t.Fatalf("read back user: %v", err)
		}
		if !stored.CreatedAt.UTC().Equal(midnight) {
			t.Fatalf("premise gone: seeded %s, stored %s — the row is no longer on the boundary",
				midnight, stored.CreatedAt.UTC())
		}

		for _, tc := range []struct {
			dateTo string
			want   bool
		}{
			{dayBefore, false},
			{itsOwnDay, true},
		} {
			users, _, err := repo.GetRecentSignupsFiltered(&RecentSignupsFilterParams{Page: 1, Limit: 50, DateTo: tc.dateTo})
			if err != nil {
				t.Fatalf("dateTo=%s: %v", tc.dateTo, err)
			}
			got := containsUser(users, "midnight-user")
			if got != tc.want {
				t.Errorf("dateTo=%s found the midnight row = %v, want %v — a row on the boundary belongs to one day, not two",
					tc.dateTo, got, tc.want)
			}
		}

		// The lower bound is inclusive, so the same row is found from its own day.
		users, _, err := repo.GetRecentSignupsFiltered(&RecentSignupsFilterParams{Page: 1, Limit: 50, DateFrom: itsOwnDay})
		if err != nil {
			t.Fatalf("dateFrom=%s: %v", itsOwnDay, err)
		}
		if !containsUser(users, "midnight-user") {
			t.Errorf("dateFrom=%s did not find the midnight row — the lower bound must include the instant the day begins", itsOwnDay)
		}
	})

	t.Run("rooms list", func(t *testing.T) {
		db := testutil.SetupTestDB(t)
		repo := NewRoomRepository(db)

		if err := db.Create(&models.User{
			ID: "mr-owner", Email: "mr-owner@ex.com", Name: "MrOwner",
			Provider: "local", IsActive: true,
		}).Error; err != nil {
			t.Fatalf("seed owner: %v", err)
		}
		room, err := repo.CreateRoom("mr-owner", "midnight-room", true, "standard", 0, &models.RoomSettings{})
		if err != nil {
			t.Fatalf("create room: %v", err)
		}
		if err := db.Model(&models.Room{}).Where("id = ?", room.ID).
			Update("created_at", midnight).Error; err != nil {
			t.Fatalf("backdate room: %v", err)
		}
		var storedRoom models.Room
		if err := db.Where("id = ?", room.ID).First(&storedRoom).Error; err != nil {
			t.Fatalf("read back room: %v", err)
		}
		if !storedRoom.CreatedAt.UTC().Equal(midnight) {
			t.Fatalf("premise gone: seeded %s, stored %s — the row is no longer on the boundary",
				midnight, storedRoom.CreatedAt.UTC())
		}

		for _, tc := range []struct {
			dateTo string
			want   bool
		}{
			{dayBefore, false},
			{itsOwnDay, true},
		} {
			p := RoomFilterParams{Page: 1, Limit: 50, DateTo: tc.dateTo, Sort: "createdAt", Order: "desc"}
			rooms, _, err := repo.GetAllRoomsFiltered(&p)
			if err != nil {
				t.Fatalf("dateTo=%s: %v", tc.dateTo, err)
			}
			got := containsRoom(rooms, room.ID)
			if got != tc.want {
				t.Errorf("dateTo=%s found the midnight room = %v, want %v — a row on the boundary belongs to one day, not two",
					tc.dateTo, got, tc.want)
			}
		}
	})
}

// dayEnd is what makes the upper bound exclusive, and it is the single place
// all three endpoints now get it from.
func TestDayBounds(t *testing.T) {
	if _, ok := dayStart(""); ok {
		t.Error("an empty value is not a filter and must not produce a bound")
	}
	if _, ok := dayEnd(""); ok {
		t.Error("an empty value is not a filter and must not produce a bound")
	}
	for _, bad := range []string{"not-a-date", "2026-13-01", "2026-03-02T00:00:00Z", "03/02/2026"} {
		if _, ok := dayStart(bad); ok {
			t.Errorf("dayStart(%q) reported a usable bound", bad)
		}
	}

	from, ok := dayStart("2026-03-02")
	if !ok {
		t.Fatal("dayStart rejected a well-formed date")
	}
	if want := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC); !from.Equal(want) {
		t.Errorf("dayStart = %s, want %s", from, want)
	}

	to, ok := dayEnd("2026-03-02")
	if !ok {
		t.Fatal("dayEnd rejected a well-formed date")
	}
	if want := time.Date(2026, 3, 3, 0, 0, 0, 0, time.UTC); !to.Equal(want) {
		t.Errorf("dayEnd = %s, want %s — the bound is the next midnight, compared with <", to, want)
	}

	// A day that a fixed 24h would get wrong if the bound were ever resolved in
	// a zone with DST. AddDate says "next calendar day" whatever the zone.
	if to, _ := dayEnd("2026-03-07"); !to.Equal(time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("dayEnd across a DST-transition date = %s, want 2026-03-08", to)
	}
}

func containsUser(users []models.RecentUser, id string) bool {
	for i := range users {
		if users[i].ID == id {
			return true
		}
	}
	return false
}

func containsRoom(rooms []models.Room, id string) bool {
	for i := range rooms {
		if rooms[i].ID == id {
			return true
		}
	}
	return false
}

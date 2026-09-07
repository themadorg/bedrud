package repository

import (
	"testing"
	"time"

	"bedrud/internal/models"
)

// The admin rooms list joins users to resolve the owner, and `rooms` and
// `users` share id, name, is_active, created_at and updated_at. SQLite does not
// treat every one of those as ambiguous — an unqualified `name` resolves
// quietly against the driving table there — so the SQLite suite cannot tell a
// qualified column from an unqualified one for all of them.
//
// This file is the Postgres half, alongside room_events_postgres_test.go and
// for the same reason: it is where a dialect-specific regression in this
// endpoint lands. Every filter and sort here forces the join.
func TestPostgres_AdminRoomsList_JoinedQueriesResolve(t *testing.T) {
	repo, db := newPostgresRoomRepoTZ(t, "UTC")

	if err := db.Create(&models.User{
		ID: "pg-owner", Email: "pg-owner@ex.com", Name: "PgOwner",
		Provider: "local", IsActive: true,
	}).Error; err != nil {
		t.Fatalf("seed owner: %v", err)
	}
	room, err := repo.CreateRoom("pg-owner", "pg-admin-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	created := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	if err := db.Model(&models.Room{}).Where("id = ?", room.ID).
		Update("created_at", created).Error; err != nil {
		t.Fatalf("backdate room: %v", err)
	}

	cases := []struct {
		name  string
		param RoomFilterParams
		want  int
	}{
		{"owner filter", RoomFilterParams{Owner: "PgOwner"}, 1},
		{"owner filter and search", RoomFilterParams{Owner: "PgOwner", Search: "pg-admin-room"}, 1},
		// The search term matches the room but not the owner. An unqualified
		// `name` that resolved against users would return nothing here.
		{"search that only the room name satisfies", RoomFilterParams{Owner: "PgOwner", Search: "pg-admin"}, 1},
		// …and the mirror: a term matching only the owner must not match the room.
		{"search that only the owner name satisfies", RoomFilterParams{Owner: "PgOwner", Search: "PgOwner"}, 0},
		{"owner filter and status", RoomFilterParams{Owner: "PgOwner", Status: []string{"active"}}, 1},
		{"owner filter and visibility", RoomFilterParams{Owner: "PgOwner", Visibility: []string{"public"}}, 1},
		{"date range covering the room", RoomFilterParams{DateFrom: "2026-03-01", DateTo: "2026-03-02"}, 1},
		{"date range ending the day before", RoomFilterParams{DateTo: "2026-03-01"}, 0},
		{"last activity range", RoomFilterParams{LastActivityFrom: "2020-01-01"}, 1},
		{"sort by owner", RoomFilterParams{Sort: "createdBy", Order: "asc"}, 1},
		{"sort by last activity", RoomFilterParams{Sort: "lastActivityAt", Order: "desc"}, 1},
		{"sort by participant count", RoomFilterParams{Sort: "participantsCount", Order: "desc"}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.param
			p.Page, p.Limit = 1, 50
			if p.Sort == "" {
				p.Sort, p.Order = "createdAt", "desc"
			}
			rooms, total, err := repo.GetAllRoomsFiltered(&p)
			if err != nil {
				t.Fatalf("GetAllRoomsFiltered: %v", err)
			}
			if len(rooms) != tc.want {
				t.Errorf("returned %d room(s), want %d", len(rooms), tc.want)
			}
			// The statement is SELECT * over a join, and `users` repeats id,
			// name, is_active, created_at and updated_at. Assert the row that
			// comes back is the room and not the owner scanned into its place.
			for i := range rooms {
				if rooms[i].ID != room.ID || rooms[i].Name != "pg-admin-room" {
					t.Errorf("scanned room = {id:%s name:%s}, want {id:%s name:pg-admin-room} — a joined users column landed in the Room",
						rooms[i].ID, rooms[i].Name, room.ID)
				}
			}
			// The count runs as its own statement and can fail or disagree on
			// its own — it is the half that broke in #119.
			if int(total) != tc.want {
				t.Errorf("total = %d, want %d — the count and the page disagree", total, tc.want)
			}
		})
	}
}

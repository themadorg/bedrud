package handlers

import (
	"net/http"
	"testing"
	"time"
)

// Three admin list endpoints take a dateFrom/dateTo pair. Room events and the
// users list validate the value and answer 400 on a bad one; the rooms list
// read it straight off the query string, handed it to a repository that parsed
// RFC3339, and dropped the clause when the parse failed. A caller sending
// 2026-09-01 — the shape the other two document and the admin UI sends — got a
// complete unfiltered list with a 200 and nothing to say the filter had been
// ignored.
//
// These tests are about the handler's contract: what it accepts and what it
// refuses. Which day the bounds actually select is pinned separately, in
// repository/day_bounds_test.go, and the dialect-specific half in
// repository/admin_rooms_postgres_test.go. The fixture and the response readers
// live in admin_rooms_join_test.go.

// A value the endpoint cannot interpret must be refused, not dropped. Returning
// an unfiltered list with a 200 is the worst of both: the caller cannot tell
// from the response that it asked for something the server ignored.
func TestAdminListRooms_RejectsUnparseableDate(t *testing.T) {
	app := setupAdminDateFilterApp(t)

	for _, q := range []string{
		"?dateFrom=not-a-date",
		"?dateTo=not-a-date",
		"?lastActivityFrom=not-a-date",
		"?lastActivityTo=not-a-date",
		// An RFC3339 instant is no longer accepted either. It was never a
		// documented shape for this endpoint — the swagger block declares none
		// of these four params — and it carried a different meaning for dateTo
		// than the bare date every other admin filter uses.
		"?dateFrom=2026-09-01T00:00:00Z",
	} {
		t.Run(q, func(t *testing.T) {
			status, _ := roomsListStatusAndCount(t, app, q)
			if status != http.StatusBadRequest {
				t.Errorf("GET /admin/rooms%s = %d, want 400 — an uninterpretable filter must be refused, not silently dropped", q, status)
			}
		})
	}
}

// The shape every other admin date filter takes, and the one the admin UI
// sends. Before this was fixed the RFC3339 parse failed, the clause was
// dropped, and the room seeded today came back under a filter that excludes
// today.
func TestAdminListRooms_BareDateFiltersInsteadOfBeingIgnored(t *testing.T) {
	app := setupAdminDateFilterApp(t)

	// Bounds far enough either side of "now" that the seeded room's position is
	// not in doubt, computed without calling the code under test. Each is paired
	// with its mirror, so a repository that dropped every row would not pass.
	past := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")
	future := time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02")

	for _, tc := range []struct {
		query string
		want  int
		why   string
	}{
		{"?dateTo=" + past, 0, "the room was created long after this bound"},
		{"?dateTo=" + future, 1, "the room was created long before this bound"},
		{"?dateFrom=" + future, 0, "the room was created long before this bound"},
		{"?dateFrom=" + past, 1, "the room was created long after this bound"},
		{"?lastActivityTo=" + past, 0, "the room was last active long after this bound"},
		{"?lastActivityFrom=" + past, 1, "the room was last active long after this bound"},
		{"?dateFrom=" + past + "&dateTo=" + future, 1, "the room is inside this range"},
		{"?dateFrom=" + future + "&dateTo=" + future, 0, "the room is outside this range"},
	} {
		t.Run(tc.query, func(t *testing.T) {
			status, count := roomsListStatusAndCount(t, app, tc.query)
			if status != http.StatusOK {
				t.Fatalf("GET /admin/rooms%s = %d, want 200", tc.query, status)
			}
			if count != tc.want {
				t.Errorf("GET /admin/rooms%s returned %d room(s), want %d — %s", tc.query, count, tc.want, tc.why)
			}
		})
	}
}

// An empty value means "no filter" and must not become a 400, or clearing a
// filter in the admin UI would fail the request.
func TestAdminListRooms_EmptyDateIsNotAFilter(t *testing.T) {
	app := setupAdminDateFilterApp(t)

	status, count := roomsListStatusAndCount(t, app, "?dateFrom=&dateTo=&lastActivityFrom=&lastActivityTo=")
	if status != http.StatusOK {
		t.Fatalf("empty date params = %d, want 200", status)
	}
	if count != 1 {
		t.Errorf("empty date params returned %d room(s), want 1", count)
	}
}

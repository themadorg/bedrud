package repository

import (
	"testing"
	"time"
)

// parseDBTime turns the timestamp column back into a time.Time. Which layouts
// it needs is decided by two things at once: the driver, and the local zone of
// the process running the tests.
//
// SQLite hands back its own text format directly. Postgres hands back a
// time.Time, which database/sql's convertAssign renders with RFC3339Nano — and
// that renders UTC as a trailing "Z", a shape none of the original layouts
// accepted. The offset forms parsed fine, so the bug was invisible to anyone
// whose machine was not on UTC, and total on a UTC CI runner or container.
//
// These cases are literal strings for exactly that reason: they pin the shapes
// independently of where the test happens to run.
func TestParseDBTime_DialectLayouts(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  time.Time
	}{
		{
			name:  "postgres utc with fractional seconds",
			input: "2026-08-31T05:26:18.846418Z",
			want:  time.Date(2026, 8, 31, 5, 26, 18, 846418000, time.UTC),
		},
		{
			name:  "postgres utc whole seconds",
			input: "2026-08-31T05:26:18Z",
			want:  time.Date(2026, 8, 31, 5, 26, 18, 0, time.UTC),
		},
		{
			name:  "postgres non-utc offset",
			input: "2026-08-31T08:56:19.186763+03:30",
			want:  time.Date(2026, 8, 31, 8, 56, 19, 186763000, time.FixedZone("", 3*3600+1800)),
		},
		{
			name:  "sqlite with offset",
			input: "2026-08-31 05:26:18.846418+00:00",
			want:  time.Date(2026, 8, 31, 5, 26, 18, 846418000, time.UTC),
		},
		{
			name:  "sqlite without zone",
			input: "2026-08-31 05:26:18",
			want:  time.Date(2026, 8, 31, 5, 26, 18, 0, time.UTC),
		},
		{
			name:  "date only",
			input: "2026-08-31",
			want:  time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseDBTime(tc.input)
			if err != nil {
				t.Fatalf("parseDBTime(%q): %v", tc.input, err)
			}
			if !got.Equal(tc.want) {
				t.Fatalf("parseDBTime(%q) = %v, want %v", tc.input, got, tc.want)
			}
		})
	}
}

// An unparseable value has to come back as an error, not as a zero time that
// callers cannot tell from a real one. The three call sites log on error; they
// can only do that if the error survives.
func TestParseDBTime_RejectsGarbage(t *testing.T) {
	for _, input := range []string{"", "not a time", "31/08/2026", "2026-13-45T99:99:99Z"} {
		if got, err := parseDBTime(input); err == nil {
			t.Fatalf("parseDBTime(%q) = %v, want an error", input, got)
		}
	}
}

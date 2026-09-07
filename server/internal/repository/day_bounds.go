package repository

import "time"

// dayFilterLayout is the shape every admin date filter accepts: a bare calendar
// date, no offset, no time of day.
const dayFilterLayout = "2006-01-02"

// dayStart turns a YYYY-MM-DD filter value into the instant that day begins,
// and reports whether there was a value to convert at all. An empty string is
// not a filter, so it returns false without an error; a malformed one also
// returns false, and callers are expected to have rejected it before reaching
// here — the handlers answer 400 rather than letting a clause disappear.
//
// The bound is an instant, not a date literal, because the column holds an
// instant. Handing the driver a bare date leaves the dialect to interpret it:
// Postgres resolves it in the session timezone, so the same request would cover
// a different span of time depending on how the server happened to be
// configured.
func dayStart(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	d, err := time.Parse(dayFilterLayout, v)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// dayEnd turns a YYYY-MM-DD filter value into the instant the *following* day
// begins, so the caller compares with `<` and the named day is included whole.
//
// The exclusive form is what makes a row land in exactly one day. An inclusive
// `<= day + 24h` puts a row stored at exactly midnight in two adjacent filters
// at once — one instant wide, reachable only by a row on the boundary, and
// wrong in a way that only shows up when someone seeds that row deliberately.
//
// AddDate rather than Add(24 * time.Hour): the two agree in UTC, and AddDate
// says "the next calendar day" outright, which is the intent.
func dayEnd(v string) (time.Time, bool) {
	d, ok := dayStart(v)
	if !ok {
		return time.Time{}, false
	}
	return d.AddDate(0, 0, 1), true
}

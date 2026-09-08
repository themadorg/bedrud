package handlers

import "time"

// dayFilterLayout is the shape every admin date filter accepts: a bare calendar
// date, no offset, no time of day.
const dayFilterLayout = "2006-01-02"

// queryDayFilters reads a set of YYYY-MM-DD query parameters and reports the
// first one it cannot interpret, so the caller can refuse the request.
//
// Refusing is the point. The repositories drop a clause they cannot parse,
// which turns a filter the caller asked for into a full unfiltered result set
// with a 200 and nothing in the response to say so. Two of the three admin list
// endpoints already validated; the rooms list read these straight off the query
// string and handed them to an RFC3339 parse.
//
// It returns the name of the offending parameter rather than writing the
// response itself. An earlier version returned the result of c.Status().JSON(),
// which reports whether *serialisation* failed and is nil on success — so the
// caller's error check never fired and every bad value fell through to a panic
// on the empty slice. Returning a non-nil error instead would hand the request
// to Fiber's error handler and overwrite the body already written.
func queryDayFilters(c interface {
	Query(string, ...string) string
}, names ...string) (values []string, invalid string) {
	values = make([]string, len(names))
	for i, name := range names {
		v := c.Query(name)
		if v == "" {
			// Absent, or cleared in the admin UI. Not a filter, not an error.
			continue
		}
		if _, err := time.Parse(dayFilterLayout, v); err != nil {
			return nil, name
		}
		values[i] = v
	}
	return values, ""
}

// invalidDayFilter is the message every endpoint answers with, so the three
// list endpoints cannot drift into describing the same rejection differently.
func invalidDayFilter(name string) string {
	return "Invalid " + name + " format, expected YYYY-MM-DD"
}

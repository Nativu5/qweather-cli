package catalog

import "time"

const (
	TideDateWindowDays      = 10
	AstronomyDateWindowDays = 60
)

// SupportsStormYear reports whether year is accepted by the rolling Storm List contract.
func SupportsStormYear(now time.Time, year int) bool {
	if now.IsZero() {
		return false
	}
	current := now.UTC().Year()
	return year == current || year == current-1
}

// UTCDateWindow returns the inclusive UTC calendar-date window described by
// QWeather's date10Query and date60Query contracts.
func UTCDateWindow(now time.Time, days int) (time.Time, time.Time, bool) {
	if now.IsZero() || days <= 0 {
		return time.Time{}, time.Time{}, false
	}
	utc := now.UTC()
	first := time.Date(utc.Year(), utc.Month(), utc.Day(), 0, 0, 0, 0, time.UTC)
	return first, first.AddDate(0, 0, days-1), true
}

package catalog

import (
	"testing"
	"time"
)

func TestSupportsStormYearUsesUTCYear(t *testing.T) {
	now := time.Date(2027, 1, 1, 1, 0, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	tests := []struct {
		year int
		want bool
	}{
		{year: 2026, want: true},
		{year: 2025, want: true},
		{year: 2024, want: false},
		{year: 2027, want: false},
	}
	for _, test := range tests {
		if got := SupportsStormYear(now, test.year); got != test.want {
			t.Errorf("SupportsStormYear(%d) = %v, want %v", test.year, got, test.want)
		}
	}
}

func TestUTCDateWindowIncludesExactlyTheNamedNumberOfDays(t *testing.T) {
	now := time.Date(2027, 1, 1, 1, 0, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	tests := []struct {
		days      int
		wantFirst string
		wantLast  string
	}{
		{days: TideDateWindowDays, wantFirst: "2026-12-31", wantLast: "2027-01-09"},
		{days: AstronomyDateWindowDays, wantFirst: "2026-12-31", wantLast: "2027-02-28"},
	}
	for _, test := range tests {
		first, last, ok := UTCDateWindow(now, test.days)
		if !ok || first.Format("2006-01-02") != test.wantFirst || last.Format("2006-01-02") != test.wantLast {
			t.Errorf("UTCDateWindow(%d) = %s..%s, %v", test.days, first.Format("2006-01-02"), last.Format("2006-01-02"), ok)
		}
	}
}

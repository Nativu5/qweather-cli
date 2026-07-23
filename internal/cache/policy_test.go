package cache

import (
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

func TestExpirationUsesHardTTLAndEarlierBoundaries(t *testing.T) {
	now := time.Date(2026, 7, 23, 10, 50, 0, 0, time.UTC)
	tests := []struct {
		name     string
		policy   catalog.CachePolicy
		timezone string
		want     time.Time
	}{
		{
			name:   "hard TTL",
			policy: catalog.CachePolicy{TTL: 30 * time.Minute, Boundary: catalog.BoundaryNone},
			want:   now.Add(30 * time.Minute),
		},
		{
			name:   "UTC hour",
			policy: catalog.CachePolicy{TTL: 30 * time.Minute, Boundary: catalog.BoundaryUTCHour},
			want:   time.Date(2026, 7, 23, 11, 0, 0, 0, time.UTC),
		},
		{
			name:     "local day",
			policy:   catalog.CachePolicy{TTL: 8 * time.Hour, Boundary: catalog.BoundaryLocalDay},
			timezone: "Asia/Shanghai",
			want:     time.Date(2026, 7, 23, 16, 0, 0, 0, time.UTC),
		},
		{
			name:   "unknown local timezone uses hard TTL",
			policy: catalog.CachePolicy{TTL: time.Hour, Boundary: catalog.BoundaryLocalHour},
			want:   now.Add(time.Hour),
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := Expiration(now, test.policy, test.timezone)
			if err != nil {
				t.Fatal(err)
			}
			if !got.Equal(test.want) {
				t.Fatalf("Expiration() = %s, want %s", got, test.want)
			}
		})
	}
}

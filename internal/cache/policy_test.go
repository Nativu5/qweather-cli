package cache

import (
	"encoding/json"
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

func TestPolicyForResponseSelectsStormActiveAndInactiveTTL(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		outcome string
		body    string
		want    time.Duration
	}{
		{name: "list with active storm", id: "storm.list", outcome: "ok", body: `{"storm":[{"isActive":"0"},{"isActive":"1"}]}`, want: 20 * time.Minute},
		{name: "list with inactive storms", id: "storm.list", outcome: "ok", body: `{"storm":[{"isActive":"0"}]}`, want: time.Hour},
		{name: "empty storm list", id: "storm.list", outcome: "ok", body: `{"storm":[]}`, want: time.Hour},
		{name: "list missing status data", id: "storm.list", outcome: "ok", body: `{"futureField":true}`, want: 20 * time.Minute},
		{name: "active track", id: "storm.track", outcome: "ok", body: `{"isActive":"1"}`, want: 20 * time.Minute},
		{name: "inactive track", id: "storm.track", outcome: "ok", body: `{"isActive":"0"}`, want: time.Hour},
		{name: "forecast present", id: "storm.forecast", outcome: "ok", body: `{"forecast":[{}]}`, want: 20 * time.Minute},
		{name: "forecast empty", id: "storm.forecast", outcome: "ok", body: `{"forecast":[]}`, want: time.Hour},
		{name: "inactive forecast is null", id: "storm.forecast", outcome: "ok", body: `{"code":"200","forecast":null}`, want: time.Hour},
		{name: "storm no data", id: "storm.forecast", outcome: "no_data", body: `{}`, want: time.Hour},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capability := testCapability(t, test.id)
			got := PolicyForResponse(capability, test.outcome, json.RawMessage(test.body))
			if got.TTL != test.want || got.InactiveTTL != time.Hour {
				t.Fatalf("policy = %#v, want TTL %s", got, test.want)
			}
		})
	}

	ordinary := testCapability(t, "weather.city.current")
	if got := PolicyForResponse(ordinary, "ok", json.RawMessage(`{"now":{}}`)); got != ordinary.Cache {
		t.Fatalf("ordinary policy changed: got %#v want %#v", got, ordinary.Cache)
	}
}

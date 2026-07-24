package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
)

// Enabled reports whether effective configuration permits a capability's policy.
func Enabled(capability catalog.Capability, globalEnabled, sensitiveEnabled bool) bool {
	if !globalEnabled || capability.Domain == "geo" {
		return false
	}
	switch capability.Cache.Mode {
	case catalog.CacheEnabled:
		return true
	case catalog.CacheSensitive:
		return sensitiveEnabled
	default:
		return false
	}
}

// PolicyForResponse selects a response-dependent TTL without weakening the
// conservative default when the provider omits or changes its status fields.
func PolicyForResponse(capability catalog.Capability, outcome string, data json.RawMessage) catalog.CachePolicy {
	policy := capability.Cache
	if policy.InactiveTTL <= 0 {
		return policy
	}
	if outcome == "no_data" {
		policy.TTL = policy.InactiveTTL
		return policy
	}
	if outcome != "ok" || !inactiveStormResponse(capability.ID, data) {
		return policy
	}
	policy.TTL = policy.InactiveTTL
	return policy
}

func maximumTTL(policy catalog.CachePolicy) time.Duration {
	if policy.InactiveTTL > policy.TTL {
		return policy.InactiveTTL
	}
	return policy.TTL
}

func inactiveStormResponse(capabilityID string, data json.RawMessage) bool {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil {
		return false
	}
	switch capabilityID {
	case "storm.list":
		raw, ok := object["storm"]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return false
		}
		var storms []struct {
			IsActive string `json:"isActive"`
		}
		if json.Unmarshal(raw, &storms) != nil {
			return false
		}
		for _, storm := range storms {
			if storm.IsActive != "0" {
				return false
			}
		}
		return true
	case "storm.track":
		var isActive string
		return json.Unmarshal(object["isActive"], &isActive) == nil && isActive == "0"
	case "storm.forecast":
		raw, ok := object["forecast"]
		if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			return false
		}
		var forecast []json.RawMessage
		return json.Unmarshal(raw, &forecast) == nil && len(forecast) == 0
	default:
		return false
	}
}

// Expiration applies the hard TTL and any earlier forecast boundary.
func Expiration(now time.Time, policy catalog.CachePolicy, timezone string) (time.Time, error) {
	if policy.TTL <= 0 {
		return time.Time{}, errors.New("cache TTL must be positive")
	}
	now = now.UTC()
	hard := now.Add(policy.TTL)
	boundary, err := nextBoundary(now, policy.Boundary, timezone)
	if err != nil {
		return time.Time{}, err
	}
	if boundary.IsZero() || !boundary.Before(hard) {
		return hard, nil
	}
	return boundary, nil
}

func nextBoundary(now time.Time, boundary catalog.CacheBoundary, timezone string) (time.Time, error) {
	switch boundary {
	case catalog.BoundaryNone:
		return time.Time{}, nil
	case catalog.BoundaryUTCHour:
		return now.UTC().Truncate(time.Hour).Add(time.Hour), nil
	case catalog.BoundaryUTCDay:
		utc := now.UTC()
		return time.Date(utc.Year(), utc.Month(), utc.Day()+1, 0, 0, 0, 0, time.UTC), nil
	case catalog.BoundaryLocalHour, catalog.BoundaryLocalDay:
		if timezone == "" {
			return time.Time{}, nil
		}
		location, err := time.LoadLocation(timezone)
		if err != nil {
			return time.Time{}, fmt.Errorf("load target timezone: %w", err)
		}
		local := now.In(location)
		if boundary == catalog.BoundaryLocalHour {
			return time.Date(local.Year(), local.Month(), local.Day(), local.Hour()+1, 0, 0, 0, location).UTC(), nil
		}
		return time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location).UTC(), nil
	default:
		return time.Time{}, fmt.Errorf("unknown cache boundary %q", boundary)
	}
}

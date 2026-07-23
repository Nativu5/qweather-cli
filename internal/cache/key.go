package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

const keySchema = "qweather.cache-key/v1"

var ErrPolicyDisabled = errors.New("cache policy does not permit persistence")

// Key is an opaque cache address. Its canonical source material is never persisted.
type Key struct {
	capabilityID string
	profile      string
	ttl          time.Duration
	family       catalog.ResponseFamily
	digest       [sha256.Size]byte
}

func (k Key) CapabilityID() string {
	return k.capabilityID
}

func (k Key) String() string {
	return hex.EncodeToString(k.digest[:])
}

// Material contains only non-secret, response-affecting request semantics.
type Material struct {
	APIHost        string
	Profile        string
	EffectiveLang  string
	EffectiveUnit  string
	AllowSensitive bool
	Input          catalog.Input
	Resolved       place.Resolved
	Request        qweather.Request
}

type canonicalKey struct {
	Schema        string          `json:"schema"`
	Capability    string          `json:"capability"`
	Revision      uint            `json:"revision"`
	Origin        string          `json:"origin"`
	Profile       string          `json:"profile"`
	Target        canonicalTarget `json:"target"`
	OperationPath string          `json:"operationPath"`
	Query         []queryPair     `json:"query"`
	EffectiveLang string          `json:"effectiveLang,omitempty"`
	EffectiveUnit string          `json:"effectiveUnit,omitempty"`
}

type canonicalTarget struct {
	Kind string `json:"kind"`
	ID   string `json:"id,omitempty"`
	Lat  string `json:"lat,omitempty"`
	Lon  string `json:"lon,omitempty"`
}

type queryPair struct {
	Name   string   `json:"name"`
	Values []string `json:"values"`
}

// BuildKey constructs an opaque digest and rejects every policy-forbidden capability.
func BuildKey(capability catalog.Capability, material Material) (Key, error) {
	if capability.Domain == "geo" || capability.Cache.Mode == catalog.CacheDisabled {
		return Key{}, ErrPolicyDisabled
	}
	if capability.Cache.Mode == catalog.CacheSensitive && !material.AllowSensitive {
		return Key{}, ErrPolicyDisabled
	}
	if capability.RequestRevision == 0 {
		return Key{}, errors.New("request revision must be positive")
	}
	if capability.Cache.TTL <= 0 {
		return Key{}, errors.New("cache policy TTL must be positive")
	}
	if material.Request.CapabilityID != "" && material.Request.CapabilityID != capability.ID {
		return Key{}, errors.New("request capability does not match cache capability")
	}
	target, err := targetFor(capability.Target, material.Input, material.Resolved)
	if err != nil {
		return Key{}, err
	}
	canonical := canonicalKey{
		Schema:        keySchema,
		Capability:    capability.ID,
		Revision:      capability.RequestRevision,
		Origin:        "https://" + strings.ToLower(strings.TrimSuffix(strings.TrimSpace(material.APIHost), ".")),
		Profile:       material.Profile,
		Target:        target,
		OperationPath: material.Request.Path,
		Query:         canonicalQuery(material.Request.Query),
	}
	if hasFlag(capability, "lang") {
		canonical.EffectiveLang = material.EffectiveLang
	}
	if hasFlag(capability, "unit") {
		canonical.EffectiveUnit = material.EffectiveUnit
	}
	encoded, err := json.Marshal(canonical)
	if err != nil {
		return Key{}, fmt.Errorf("encode canonical cache key: %w", err)
	}
	return Key{
		capabilityID: capability.ID, profile: material.Profile,
		ttl: capability.Cache.TTL, family: capability.Upstream.ResponseFamily,
		digest: sha256.Sum256(encoded),
	}, nil
}

func targetFor(kind catalog.TargetKind, input catalog.Input, resolved place.Resolved) (canonicalTarget, error) {
	target := canonicalTarget{Kind: string(kind)}
	switch kind {
	case catalog.TargetNone:
		return target, nil
	case catalog.TargetPlace:
		if resolved.ID != "" {
			target.ID = resolved.ID
		} else {
			target.Lat, target.Lon = resolved.Lat, resolved.Lon
		}
		if target.ID == "" && (target.Lat == "" || target.Lon == "") {
			return canonicalTarget{}, errors.New("resolved place has no usable target")
		}
	case catalog.TargetLocationID:
		target.ID = resolved.ID
		if target.ID == "" {
			return canonicalTarget{}, errors.New("resolved place has no Location ID")
		}
	case catalog.TargetCoordinate:
		target.Lat, target.Lon = resolved.Lat, resolved.Lon
		if target.Lat == "" || target.Lon == "" {
			return canonicalTarget{}, errors.New("resolved place has no coordinates")
		}
	case catalog.TargetAirStation:
		target.ID = strings.TrimSpace(input.AirStationID)
	case catalog.TargetTideStation:
		target.ID = strings.TrimSpace(input.TideStationID)
	case catalog.TargetStorm:
		target.ID = strings.TrimSpace(input.StormID)
	case catalog.TargetGeoLookup:
		return canonicalTarget{}, ErrPolicyDisabled
	default:
		return canonicalTarget{}, fmt.Errorf("unsupported cache target kind %q", kind)
	}
	if target.ID == "" && target.Lat == "" && target.Lon == "" && kind != catalog.TargetNone {
		return canonicalTarget{}, errors.New("cache target is empty")
	}
	return target, nil
}

func canonicalQuery(values url.Values) []queryPair {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	result := make([]queryPair, 0, len(names))
	for _, name := range names {
		items := append([]string(nil), values[name]...)
		sort.Strings(items)
		result = append(result, queryPair{Name: name, Values: items})
	}
	return result
}

func hasFlag(capability catalog.Capability, name string) bool {
	for _, flag := range capability.Flags {
		if flag.Name == name {
			return true
		}
	}
	return false
}

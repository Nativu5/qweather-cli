package catalog

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"sort"
	"strings"
)

type Registry struct {
	records []Capability
	byID    map[string]Capability
}

func Default() (*Registry, error) {
	return New(records())
}

func New(records []Capability) (*Registry, error) {
	if err := Validate(records); err != nil {
		return nil, err
	}
	copyOfRecords := append([]Capability(nil), records...)
	sort.Slice(copyOfRecords, func(i, j int) bool { return copyOfRecords[i].ID < copyOfRecords[j].ID })
	byID := make(map[string]Capability, len(copyOfRecords))
	for _, record := range copyOfRecords {
		byID[record.ID] = record
	}
	return &Registry{records: copyOfRecords, byID: byID}, nil
}

func (r *Registry) All() []Capability {
	return append([]Capability(nil), r.records...)
}

func (r *Registry) Current() []Capability {
	return r.filterLifecycle(LifecycleCurrent)
}

func (r *Registry) Deprecated() []Capability {
	return r.filterLifecycle(LifecycleDeprecated)
}

func (r *Registry) filterLifecycle(lifecycle Lifecycle) []Capability {
	result := make([]Capability, 0, len(r.records))
	for _, record := range r.records {
		if record.Lifecycle == lifecycle {
			result = append(result, record)
		}
	}
	return result
}

func (r *Registry) Find(id string) (Capability, bool) {
	record, ok := r.byID[id]
	return record, ok
}

func (r *Registry) Hash() (string, error) {
	encoded, err := json.Marshal(r.records)
	if err != nil {
		return "", fmt.Errorf("encode registry: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}

func Validate(records []Capability) error {
	if len(records) == 0 {
		return errors.New("registry is empty")
	}
	ids := make(map[string]struct{}, len(records))
	paths := make(map[string]struct{}, len(records))
	for index, record := range records {
		prefix := fmt.Sprintf("record %d", index)
		if record.ID == "" {
			return fmt.Errorf("%s: capability ID is required", prefix)
		}
		if _, exists := ids[record.ID]; exists {
			return fmt.Errorf("duplicate capability ID %q", record.ID)
		}
		ids[record.ID] = struct{}{}
		if record.Summary == "" || record.Domain == "" {
			return fmt.Errorf("%s: summary and domain are required", record.ID)
		}
		parsedURL, err := url.Parse(record.DocsURL)
		if err != nil || parsedURL.Scheme != "https" || parsedURL.Host == "" {
			return fmt.Errorf("%s: official HTTPS documentation URL is required", record.ID)
		}
		if record.Lifecycle != LifecycleCurrent && record.Lifecycle != LifecycleDeprecated {
			return fmt.Errorf("%s: invalid lifecycle %q", record.ID, record.Lifecycle)
		}
		if record.Upstream.Method != "GET" || !strings.HasPrefix(record.Upstream.PathTemplate, "/") {
			return fmt.Errorf("%s: complete GET transport mapping is required", record.ID)
		}
		if record.Upstream.ResponseFamily == "" {
			return fmt.Errorf("%s: response family is required", record.ID)
		}
		if !validResponseFamily(record.Upstream.ResponseFamily) {
			return fmt.Errorf("%s: invalid response family %q", record.ID, record.Upstream.ResponseFamily)
		}
		if !validTarget(record.Target) {
			return fmt.Errorf("%s: invalid target kind %q", record.ID, record.Target)
		}
		if !validBillingGroup(record.BillingGroup) || !validProductGate(record.ProductGate) {
			return fmt.Errorf("%s: invalid billing group or product gate", record.ID)
		}
		if record.BillingGroup == BillingMarine && record.Lifecycle == LifecycleCurrent && record.ProductGate != GateMarine {
			return fmt.Errorf("%s: Marine capability requires the marine Product Gate", record.ID)
		}
		if record.BillingGroup == BillingSolar && record.Lifecycle == LifecycleCurrent && record.ProductGate != GateSolar {
			return fmt.Errorf("%s: Solar capability requires the solar Product Gate", record.ID)
		}
		if record.Cache.Mode == "" || record.Cache.Boundary == "" || record.Cache.Evidence == "" {
			return fmt.Errorf("%s: explicit cache policy is required", record.ID)
		}
		if !validCachePolicy(record.Cache) {
			return fmt.Errorf("%s: invalid cache policy", record.ID)
		}
		if record.Lifecycle == LifecycleCurrent {
			if strings.TrimSpace(record.CommandPath) == "" || record.RequestRevision == 0 {
				return fmt.Errorf("%s: current capability requires a command path and request revision", record.ID)
			}
			if _, exists := paths[record.CommandPath]; exists {
				return fmt.Errorf("duplicate command path %q", record.CommandPath)
			}
			paths[record.CommandPath] = struct{}{}
		} else if record.CommandPath != "" {
			return fmt.Errorf("%s: Tombstone cannot have an executable command path", record.ID)
		}
		flagNames := make(map[string]struct{}, len(record.Flags))
		for _, flag := range record.Flags {
			if flag.Name == "" || flag.Kind == "" {
				return fmt.Errorf("%s: flag name and kind are required", record.ID)
			}
			if _, exists := flagNames[flag.Name]; exists {
				return fmt.Errorf("%s: duplicate flag %q", record.ID, flag.Name)
			}
			flagNames[flag.Name] = struct{}{}
		}
	}
	return nil
}

func validResponseFamily(value ResponseFamily) bool {
	return value == ResponseLegacyV1 || value == ResponseModernV1 || value == ResponseConsoleV1
}

func validTarget(value TargetKind) bool {
	switch value {
	case TargetNone, TargetGeoLookup, TargetPlace, TargetLocationID, TargetCoordinate, TargetAirStation, TargetTideStation, TargetStorm:
		return true
	default:
		return false
	}
}

func validBillingGroup(value BillingGroup) bool {
	return value == BillingBasic || value == BillingMarine || value == BillingSolar
}

func validProductGate(value ProductGate) bool {
	return value == GateNone || value == GateMarine || value == GateSolar || value == GateSensitiveAccount
}

func validCachePolicy(value CachePolicy) bool {
	validBoundary := value.Boundary == BoundaryNone || value.Boundary == BoundaryLocalHour || value.Boundary == BoundaryLocalDay || value.Boundary == BoundaryUTCHour || value.Boundary == BoundaryUTCDay
	if !validBoundary {
		return false
	}
	switch value.Mode {
	case CacheDisabled:
		return value.TTL == 0 && value.InactiveTTL == 0 && value.Boundary == BoundaryNone
	case CacheEnabled, CacheSensitive:
		return value.TTL > 0 && value.InactiveTTL >= 0
	default:
		return false
	}
}

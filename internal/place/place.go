package place

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
)

type SpecKind string

const (
	SpecName       SpecKind = "name"
	SpecLocationID SpecKind = "location-id"
	SpecCoordinate SpecKind = "coordinate"
)

type Coordinate struct {
	Latitude  float64
	Longitude float64
	LatText   string
	LonText   string
}

// ProviderQuery returns the provider's longitude-first coordinate form.
func (c Coordinate) ProviderQuery() string {
	return c.LonText + "," + c.LatText
}

type Spec struct {
	Kind       SpecKind
	Name       string
	LocationID string
	Coordinate Coordinate
	Country    string
	Adm        string
}

type Candidate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Adm1    string `json:"adm1,omitempty"`
	Adm2    string `json:"adm2,omitempty"`
	Country string `json:"country,omitempty"`
	Lat     string `json:"-"`
	Lon     string `json:"-"`
	TZ      string `json:"-"`
}

type Resolved struct {
	ID      string
	Name    string
	Adm1    string
	Adm2    string
	Country string
	Lat     string
	Lon     string
	TZ      string
}

func (r Resolved) Output() *output.ResolvedPlace {
	return &output.ResolvedPlace{
		ID: r.ID, Name: r.Name, Adm1: r.Adm1, Adm2: r.Adm2, Country: r.Country,
		Lat: r.Lat, Lon: r.Lon, TZ: r.TZ,
	}
}

type LookupQuery struct {
	Spec     Spec
	Language string
}

type LookupFunc func(context.Context, LookupQuery) ([]Candidate, *output.Problem)

// Parse validates and normalizes exactly one caller-supplied Place Spec.
func Parse(name, locationID, coordinate, country, adm string) (Spec, error) {
	if countNonEmpty(name, locationID, coordinate) != 1 {
		return Spec{}, errors.New("exactly one place form is required")
	}
	if (country != "" || adm != "") && name == "" {
		return Spec{}, errors.New("country and administrative filters require a place name")
	}
	if name != "" {
		return Spec{Kind: SpecName, Name: strings.TrimSpace(name), Country: strings.TrimSpace(country), Adm: strings.TrimSpace(adm)}, nil
	}
	if locationID != "" {
		return Spec{Kind: SpecLocationID, LocationID: strings.TrimSpace(locationID)}, nil
	}
	parsed, err := ParseCoordinate(coordinate)
	if err != nil {
		return Spec{}, err
	}
	return Spec{Kind: SpecCoordinate, Coordinate: parsed}, nil
}

// ParseCoordinate parses the public latitude-first RFC 5870-inspired form.
func ParseCoordinate(value string) (Coordinate, error) {
	if !strings.HasPrefix(value, "geo:") {
		return Coordinate{}, errors.New("coordinate must use geo:<lat>,<lon> form")
	}
	parts := strings.Split(strings.TrimPrefix(value, "geo:"), ",")
	if len(parts) != 2 || strings.TrimSpace(parts[0]) != parts[0] || strings.TrimSpace(parts[1]) != parts[1] {
		return Coordinate{}, errors.New("coordinate must contain latitude then longitude without spaces")
	}
	if !validDecimal(parts[0], 2) || !validDecimal(parts[1], 2) {
		return Coordinate{}, errors.New("coordinate values must be decimal numbers with at most two fractional digits")
	}
	latitude, err := strconv.ParseFloat(parts[0], 64)
	if err != nil || latitude < -90 || latitude > 90 {
		return Coordinate{}, errors.New("latitude must be between -90 and 90")
	}
	longitude, err := strconv.ParseFloat(parts[1], 64)
	if err != nil || longitude < -180 || longitude > 180 {
		return Coordinate{}, errors.New("longitude must be between -180 and 180")
	}
	return Coordinate{
		Latitude: latitude, Longitude: longitude,
		LatText: canonicalNumber(latitude), LonText: canonicalNumber(longitude),
	}, nil
}

// Resolve converts a Place Spec into the target required by a capability.
func Resolve(ctx context.Context, spec Spec, target catalog.TargetKind, language string, lookup LookupFunc) (Resolved, []string, *output.Problem) {
	switch target {
	case catalog.TargetPlace:
		if spec.Kind == SpecLocationID {
			return Resolved{ID: spec.LocationID}, nil, nil
		}
		if spec.Kind == SpecCoordinate {
			return Resolved{Lat: spec.Coordinate.LatText, Lon: spec.Coordinate.LonText}, nil, nil
		}
	case catalog.TargetLocationID:
		if spec.Kind == SpecLocationID {
			return Resolved{ID: spec.LocationID}, nil, nil
		}
	case catalog.TargetCoordinate:
		if spec.Kind == SpecCoordinate {
			return Resolved{Lat: spec.Coordinate.LatText, Lon: spec.Coordinate.LonText}, nil, nil
		}
	default:
		problem := output.NewProblem(10, output.CodeInternalError, "capability target is not place-aware")
		return Resolved{}, nil, problem
	}
	if lookup == nil {
		problem := output.NewProblem(10, output.CodeInternalError, "place lookup is unavailable")
		return Resolved{}, nil, problem
	}
	candidates, problem := lookup(ctx, LookupQuery{Spec: spec, Language: language})
	if problem != nil {
		return Resolved{}, []string{"geo.city.lookup"}, problem
	}
	selected, problem := selectCandidate(spec, candidates)
	if problem != nil {
		return Resolved{}, []string{"geo.city.lookup"}, problem
	}
	if target == catalog.TargetCoordinate && (selected.Lat == "" || selected.Lon == "") {
		problem := output.NewProblem(9, output.CodeUpstreamProtocolError, "resolved place does not contain coordinates")
		return Resolved{}, []string{"geo.city.lookup"}, problem
	}
	if target == catalog.TargetCoordinate {
		coordinate, err := parseProviderCoordinate(selected.Lat, selected.Lon)
		if err != nil {
			problem := output.NewProblem(9, output.CodeUpstreamProtocolError, "resolved place contains invalid coordinates")
			problem.Cause = err
			return Resolved{}, []string{"geo.city.lookup"}, problem
		}
		selected.Lat = canonicalNumber(roundCoordinate(coordinate.Latitude))
		selected.Lon = canonicalNumber(roundCoordinate(coordinate.Longitude))
	}
	if target == catalog.TargetLocationID && selected.ID == "" {
		problem := output.NewProblem(9, output.CodeUpstreamProtocolError, "resolved place does not contain a Location ID")
		return Resolved{}, []string{"geo.city.lookup"}, problem
	}
	return Resolved{
		ID: selected.ID, Name: selected.Name, Adm1: selected.Adm1, Adm2: selected.Adm2,
		Country: selected.Country, Lat: selected.Lat, Lon: selected.Lon, TZ: selected.TZ,
	}, []string{"geo.city.lookup"}, nil
}

// DecodeCandidates decodes the candidate subset needed for invocation-local resolution.
func DecodeCandidates(data json.RawMessage) ([]Candidate, error) {
	var response struct {
		Locations []struct {
			ID      string `json:"id"`
			Name    string `json:"name"`
			Adm1    string `json:"adm1"`
			Adm2    string `json:"adm2"`
			Country string `json:"country"`
			Lat     string `json:"lat"`
			Lon     string `json:"lon"`
			TZ      string `json:"tz"`
		} `json:"location"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return nil, fmt.Errorf("decode GeoAPI candidates: %w", err)
	}
	candidates := make([]Candidate, 0, len(response.Locations))
	for _, location := range response.Locations {
		if location.ID == "" || location.Name == "" {
			return nil, errors.New("GeoAPI candidate is missing its ID or name")
		}
		if location.Lat != "" || location.Lon != "" {
			if _, err := parseProviderCoordinate(location.Lat, location.Lon); err != nil {
				return nil, err
			}
		}
		candidates = append(candidates, Candidate{
			ID: location.ID, Name: location.Name, Adm1: location.Adm1, Adm2: location.Adm2,
			Country: location.Country, Lat: location.Lat, Lon: location.Lon, TZ: location.TZ,
		})
	}
	return candidates, nil
}

func selectCandidate(spec Spec, candidates []Candidate) (Candidate, *output.Problem) {
	if len(candidates) == 0 {
		problem := output.NewProblem(5, output.CodePlaceNotFound, "place did not match any QWeather location")
		return Candidate{}, problem
	}
	if len(candidates) == 1 {
		return candidates[0], nil
	}
	if spec.Kind == SpecName {
		normalized := normalizeName(spec.Name)
		exact := make([]Candidate, 0, len(candidates))
		for _, candidate := range candidates {
			if normalizeName(candidate.Name) == normalized {
				exact = append(exact, candidate)
			}
		}
		if len(exact) == 1 {
			return exact[0], nil
		}
	}
	safe := make([]Candidate, 0, len(candidates))
	for _, candidate := range candidates {
		safe = append(safe, Candidate{
			ID: candidate.ID, Name: candidate.Name, Adm1: candidate.Adm1,
			Adm2: candidate.Adm2, Country: candidate.Country,
		})
	}
	problem := output.NewProblem(5, output.CodeAmbiguousPlace, "place matches multiple QWeather locations")
	problem.Details = map[string]any{"candidates": safe}
	return Candidate{}, problem
}

func normalizeName(value string) string {
	return strings.ToLower(strings.Join(strings.Fields(value), " "))
}

func validDecimal(value string, maxFraction int) bool {
	if value == "" || strings.ContainsAny(value, "eE+") {
		return false
	}
	unsigned := strings.TrimPrefix(value, "-")
	parts := strings.Split(unsigned, ".")
	if len(parts) > 2 || parts[0] == "" {
		return false
	}
	for _, part := range parts {
		for _, character := range part {
			if character < '0' || character > '9' {
				return false
			}
		}
	}
	return len(parts) == 1 || len(parts[1]) <= maxFraction
}

func parseProviderCoordinate(latitude, longitude string) (Coordinate, error) {
	lat, err := strconv.ParseFloat(latitude, 64)
	if err != nil || lat < -90 || lat > 90 {
		return Coordinate{}, errors.New("GeoAPI candidate contains an invalid latitude")
	}
	lon, err := strconv.ParseFloat(longitude, 64)
	if err != nil || lon < -180 || lon > 180 {
		return Coordinate{}, errors.New("GeoAPI candidate contains an invalid longitude")
	}
	return Coordinate{Latitude: lat, Longitude: lon, LatText: latitude, LonText: longitude}, nil
}

func canonicalNumber(value float64) string {
	if value == 0 {
		return "0"
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func roundCoordinate(value float64) float64 {
	return math.Round(value*100) / 100
}

func countNonEmpty(values ...string) int {
	count := 0
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			count++
		}
	}
	return count
}

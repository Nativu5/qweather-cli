package place

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
)

func TestParseRequiresExactlyOnePlaceForm(t *testing.T) {
	tests := []struct {
		name       string
		place      string
		locationID string
		coordinate string
		country    string
		wantError  bool
	}{
		{"name", "Beijing", "", "", "CN", false},
		{"ID", "", "101010100", "", "", false},
		{"coordinate", "", "", "geo:39.9,116.4", "", false},
		{"none", "", "", "", "", true},
		{"several", "Beijing", "101010100", "", "", true},
		{"filter without name", "", "101010100", "", "CN", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := Parse(test.place, test.locationID, test.coordinate, test.country, "")
			if (err != nil) != test.wantError {
				t.Fatalf("Parse() error = %v, wantError=%v", err, test.wantError)
			}
		})
	}
}

func TestParseCoordinateUsesLatitudeFirstAndTwoDecimalPrecision(t *testing.T) {
	coordinate, err := ParseCoordinate("geo:39.90,116.40")
	if err != nil {
		t.Fatal(err)
	}
	if coordinate.LatText != "39.9" || coordinate.LonText != "116.4" || coordinate.ProviderQuery() != "116.4,39.9" {
		t.Fatalf("coordinate = %#v", coordinate)
	}
	for _, value := range []string{
		"116.4,39.9", "geo:116.4,39.9", "geo:39.123,116.4", "geo:39.9,181", "geo:39.9, 116.4", "geo:1e1,2",
	} {
		if _, err := ParseCoordinate(value); err == nil {
			t.Errorf("ParseCoordinate(%q) unexpectedly succeeded", value)
		}
	}
}

func TestResolveUsesDirectCompatibleTargetsWithoutLookup(t *testing.T) {
	lookupCalls := 0
	lookup := func(context.Context, LookupQuery) ([]Candidate, *output.Problem) {
		lookupCalls++
		return nil, nil
	}
	coordinateSpec, err := Parse("", "", "geo:39.9,116.4", "", "")
	if err != nil {
		t.Fatal(err)
	}
	resolved, operations, problem := Resolve(context.Background(), coordinateSpec, catalog.TargetCoordinate, "en", lookup)
	if problem != nil || resolved.Lat != "39.9" || resolved.Lon != "116.4" || len(operations) != 0 || lookupCalls != 0 {
		t.Fatalf("resolved=%#v operations=%#v problem=%v calls=%d", resolved, operations, problem, lookupCalls)
	}
	idSpec, err := Parse("", "101010100", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	resolved, operations, problem = Resolve(context.Background(), idSpec, catalog.TargetPlace, "en", lookup)
	if problem != nil || resolved.ID != "101010100" || len(operations) != 0 || lookupCalls != 0 {
		t.Fatalf("resolved=%#v operations=%#v problem=%v calls=%d", resolved, operations, problem, lookupCalls)
	}
}

func TestResolveConvertsCoordinateToLocationIDThroughLookup(t *testing.T) {
	spec, err := Parse("", "", "geo:39.9,116.4", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lookupCalls := 0
	lookup := func(_ context.Context, query LookupQuery) ([]Candidate, *output.Problem) {
		lookupCalls++
		if query.Spec.Coordinate.ProviderQuery() != "116.4,39.9" {
			t.Fatalf("query = %#v", query)
		}
		return []Candidate{{ID: "101010100", Name: "Beijing", Lat: "39.9", Lon: "116.4"}}, nil
	}
	resolved, operations, problem := Resolve(context.Background(), spec, catalog.TargetLocationID, "en", lookup)
	if problem != nil || resolved.ID != "101010100" || lookupCalls != 1 || len(operations) != 1 {
		t.Fatalf("resolved=%#v operations=%#v problem=%v calls=%d", resolved, operations, problem, lookupCalls)
	}
}

func TestResolveSelectsOnlyUniqueExactName(t *testing.T) {
	spec, err := Parse("  Beijing  ", "", "", "CN", "Beijing")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(_ context.Context, query LookupQuery) ([]Candidate, *output.Problem) {
		if query.Spec.Country != "CN" || query.Spec.Adm != "Beijing" || query.Language != "en" {
			t.Fatalf("query = %#v", query)
		}
		return []Candidate{
			{ID: "other", Name: "Beijing City", Lat: "40", Lon: "117"},
			{ID: "101010100", Name: "beijing", Country: "China", Lat: "39.90499", Lon: "116.40529", TZ: "Asia/Shanghai"},
		}, nil
	}
	resolved, operations, problem := Resolve(context.Background(), spec, catalog.TargetCoordinate, "en", lookup)
	if problem != nil || resolved.ID != "101010100" || resolved.Lat != "39.9" || resolved.Lon != "116.41" || len(operations) != 1 || operations[0] != "geo.city.lookup" {
		t.Fatalf("resolved=%#v operations=%#v problem=%v", resolved, operations, problem)
	}
}

func TestResolveNeverSelectsByOrderOrRank(t *testing.T) {
	spec, err := Parse("Springfield", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context, LookupQuery) ([]Candidate, *output.Problem) {
		return []Candidate{
			{ID: "first", Name: "Springfield", Adm1: "A", Lat: "10", Lon: "20"},
			{ID: "second", Name: "Springfield", Adm1: "B", Lat: "30", Lon: "40"},
		}, nil
	}
	_, _, problem := Resolve(context.Background(), spec, catalog.TargetCoordinate, "en", lookup)
	if problem == nil || problem.ExitCode != 5 || problem.Code != "AMBIGUOUS_PLACE" {
		t.Fatalf("problem = %#v", problem)
	}
	encoded, err := json.Marshal(problem)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), `"lat"`) || strings.Contains(string(encoded), `"lon"`) {
		t.Fatalf("candidate details include coordinates: %s", encoded)
	}
}

func TestResolveReportsNotFound(t *testing.T) {
	spec, err := Parse("Missing", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	_, _, problem := Resolve(context.Background(), spec, catalog.TargetLocationID, "en", func(context.Context, LookupQuery) ([]Candidate, *output.Problem) {
		return nil, nil
	})
	if problem == nil || problem.ExitCode != 5 || problem.Code != "PLACE_NOT_FOUND" {
		t.Fatalf("problem = %#v", problem)
	}
}

func TestResolveRequiresCompleteTargetFields(t *testing.T) {
	spec, err := Parse("Incomplete", "", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	lookup := func(context.Context, LookupQuery) ([]Candidate, *output.Problem) {
		return []Candidate{{ID: "incomplete", Name: "Incomplete", Lat: "10"}}, nil
	}
	_, _, problem := Resolve(context.Background(), spec, catalog.TargetCoordinate, "en", lookup)
	if problem == nil || problem.ExitCode != 9 || problem.Code != "UPSTREAM_PROTOCOL_ERROR" {
		t.Fatalf("problem = %#v", problem)
	}
}

func TestDecodeCandidatesPreservesTargetFields(t *testing.T) {
	candidates, err := DecodeCandidates(json.RawMessage(`{"code":"200","location":[{"id":"101010100","name":"Beijing","adm1":"Beijing","adm2":"Beijing","country":"China","lat":"39.90499","lon":"116.40529","tz":"Asia/Shanghai"}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(candidates) != 1 || candidates[0].ID != "101010100" || candidates[0].TZ != "Asia/Shanghai" {
		t.Fatalf("candidates = %#v", candidates)
	}
}

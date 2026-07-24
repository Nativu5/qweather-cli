package app

import (
	"math"
	"net/url"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/place"
)

func TestCompileRequestCoversIssueSixCapabilitiesAndPolicies(t *testing.T) {
	tests := []struct {
		id       string
		input    catalog.Input
		resolved place.Resolved
		language string
		unit     string
		path     string
		query    url.Values
		mode     catalog.CacheMode
		ttl      time.Duration
		boundary catalog.CacheBoundary
	}{
		{
			id: "geo.city.lookup", input: catalog.Input{Query: "Beijing", Country: "CN", Adm: "Beijing", Limit: 20},
			language: "en", path: "/geo/v2/city/lookup",
			query: url.Values{"location": {"Beijing"}, "range": {"cn"}, "adm": {"Beijing"}, "number": {"20"}, "lang": {"en"}},
			mode:  catalog.CacheDisabled, boundary: catalog.BoundaryNone,
		},
		{
			id: "geo.city.top", input: catalog.Input{Country: "CN", Limit: 5}, language: "zh", path: "/geo/v2/city/top",
			query: url.Values{"range": {"cn"}, "number": {"5"}, "lang": {"zh"}},
			mode:  catalog.CacheDisabled, boundary: catalog.BoundaryNone,
		},
		{
			id: "geo.poi.lookup", input: catalog.Input{Query: "West Lake", POIType: "tide-station", Adm: "101210101", Limit: 10},
			language: "en", path: "/geo/v2/poi/lookup",
			query: url.Values{"location": {"West Lake"}, "type": {"TSTA"}, "city": {"101210101"}, "number": {"10"}, "lang": {"en"}},
			mode:  catalog.CacheDisabled, boundary: catalog.BoundaryNone,
		},
		{
			id: "geo.poi.nearby", input: catalog.Input{POIType: "scenic", Limit: 20}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			language: "zh", path: "/geo/v2/poi/range",
			query: url.Values{"location": {"116.4,39.9"}, "type": {"scenic"}, "number": {"20"}, "lang": {"zh"}},
			mode:  catalog.CacheDisabled, boundary: catalog.BoundaryNone,
		},
		{
			id: "weather.city.current", resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/weather/now",
			query: url.Values{"location": {"101010100"}, "lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 10 * time.Minute, boundary: catalog.BoundaryNone,
		},
		{
			id: "weather.city.forecast.daily", input: catalog.Input{Days: 7}, resolved: place.Resolved{ID: "101010100"}, language: "zh", path: "/v7/weather/7d",
			query: url.Values{"location": {"101010100"}, "lang": {"zh"}},
			mode:  catalog.CacheEnabled, ttl: time.Hour, boundary: catalog.BoundaryLocalDay,
		},
		{
			id: "weather.city.forecast.hourly", input: catalog.Input{Hours: 168}, resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/weather/168h",
			query: url.Values{"location": {"101010100"}, "lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 30 * time.Minute, boundary: catalog.BoundaryLocalHour,
		},
		{
			id: "weather.grid.current", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", unit: "imperial", path: "/v7/grid-weather/now",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"en"}, "unit": {"i"}},
			mode:  catalog.CacheEnabled, ttl: 10 * time.Minute, boundary: catalog.BoundaryNone,
		},
		{
			id: "weather.grid.forecast.daily", input: catalog.Input{Days: 3}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", unit: "metric", path: "/v7/grid-weather/3d",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"zh"}, "unit": {"m"}},
			mode:  catalog.CacheEnabled, ttl: time.Hour, boundary: catalog.BoundaryUTCDay,
		},
		{
			id: "weather.grid.forecast.hourly", input: catalog.Input{Hours: 72}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", unit: "metric", path: "/v7/grid-weather/72h",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"en"}, "unit": {"m"}},
			mode:  catalog.CacheEnabled, ttl: 30 * time.Minute, boundary: catalog.BoundaryUTCHour,
		},
		{
			id: "weather.precipitation.minutely", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", path: "/v7/minutely/5m",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"zh"}},
			mode:  catalog.CacheEnabled, ttl: 5 * time.Minute, boundary: catalog.BoundaryNone,
		},
		{
			id: "weather.indices.forecast", input: catalog.Input{Days: 3, Indices: []int{3, 1}}, resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/indices/3d",
			query: url.Values{"location": {"101010100"}, "type": {"1,3"}, "lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 6 * time.Hour, boundary: catalog.BoundaryLocalDay,
		},
		{
			id: "weather.history", input: catalog.Input{Date: "2026-07-22"}, resolved: place.Resolved{ID: "101010100"}, language: "en", unit: "metric", path: "/v7/historical/weather",
			query: url.Values{"location": {"101010100"}, "date": {"20260722"}, "lang": {"en"}, "unit": {"m"}},
			mode:  catalog.CacheEnabled, ttl: 24 * time.Hour, boundary: catalog.BoundaryNone,
		},
		{
			id: "alert.current", input: catalog.Input{LocalTime: true}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/weatheralert/v1/current/39.9/116.4",
			query: url.Values{"localTime": {"true"}, "lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 5 * time.Minute, boundary: catalog.BoundaryNone,
		},
		{
			id: "air.current", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/airquality/v1/current/39.9/116.4",
			query: url.Values{"lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 30 * time.Minute, boundary: catalog.BoundaryNone,
		},
		{
			id: "air.forecast.daily", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", path: "/airquality/v1/daily/39.9/116.4",
			query: url.Values{"lang": {"zh"}},
			mode:  catalog.CacheEnabled, ttl: 8 * time.Hour, boundary: catalog.BoundaryLocalDay,
		},
		{
			id: "air.forecast.hourly", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/airquality/v1/hourly/39.9/116.4",
			query: url.Values{"lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 30 * time.Minute, boundary: catalog.BoundaryLocalHour,
		},
		{
			id: "air.station.current", input: catalog.Input{AirStationID: "P58911"}, language: "en", path: "/airquality/v1/stations/P58911",
			query: url.Values{"lang": {"en"}},
			mode:  catalog.CacheEnabled, ttl: 30 * time.Minute, boundary: catalog.BoundaryNone,
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{
				Input: test.input, Language: test.language, Unit: test.unit, Resolved: test.resolved,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
			if capability.Cache.Mode != test.mode || capability.Cache.TTL != test.ttl || capability.Cache.Boundary != test.boundary {
				t.Fatalf("cache policy = %#v", capability.Cache)
			}
		})
	}
}

func TestCompileRequestNormalizesCountryFilter(t *testing.T) {
	tests := []struct {
		name      string
		id        string
		input     catalog.Input
		wantRange string
	}{
		{name: "lookup uppercase", id: "geo.city.lookup", input: catalog.Input{Query: "Beijing", Country: "CN"}, wantRange: "cn"},
		{name: "lookup lowercase", id: "geo.city.lookup", input: catalog.Input{Query: "Beijing", Country: "cn"}, wantRange: "cn"},
		{name: "top uppercase", id: "geo.city.top", input: catalog.Input{Country: "CN"}, wantRange: "cn"},
		{name: "top lowercase", id: "geo.city.top", input: catalog.Input{Country: "cn"}, wantRange: "cn"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request, problem := CompileRequest(capability(t, test.id), RequestParameters{Input: test.input, Language: "auto"})
			if problem != nil {
				t.Fatal(problem)
			}
			if got := request.Query.Get("range"); got != test.wantRange {
				t.Fatalf("range = %q, want %q", got, test.wantRange)
			}
		})
	}
}

func TestCompileRequestTargetAndValidationEdges(t *testing.T) {
	geo := capability(t, "geo.city.lookup")
	for _, test := range []struct {
		name     string
		input    catalog.Input
		location string
	}{
		{name: "Location ID", input: catalog.Input{PlaceID: "101010100"}, location: "101010100"},
		{name: "coordinate order", input: catalog.Input{Coordinate: "geo:39.90,116.40"}, location: "116.4,39.9"},
	} {
		t.Run(test.name, func(t *testing.T) {
			request, problem := CompileRequest(geo, RequestParameters{Input: test.input, Language: "auto"})
			if problem != nil || request.Query.Get("location") != test.location || request.Query.Has("lang") {
				t.Fatalf("request=%#v problem=%v", request, problem)
			}
		})
	}

	indices := capability(t, "weather.indices.forecast")
	request, problem := CompileRequest(indices, RequestParameters{
		Input: catalog.Input{Days: 1, AllIndices: true}, Language: "auto", Resolved: place.Resolved{ID: "101010100"},
	})
	if problem != nil || request.Query.Get("type") != "0" {
		t.Fatalf("request=%#v problem=%v", request, problem)
	}

	invalid := []struct {
		name       string
		id         string
		parameters RequestParameters
	}{
		{name: "unsupported days", id: "weather.city.forecast.daily", parameters: RequestParameters{Input: catalog.Input{Days: 5}, Resolved: place.Resolved{ID: "101010100"}, Language: "en"}},
		{name: "unsupported unit", id: "weather.grid.current", parameters: RequestParameters{Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, Language: "en", Unit: "kelvin"}},
		{name: "limited language", id: "weather.precipitation.minutely", parameters: RequestParameters{Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, Language: "fr"}},
		{name: "invalid date", id: "weather.history", parameters: RequestParameters{Input: catalog.Input{Date: "2026-02-30"}, Resolved: place.Resolved{ID: "101010100"}, Language: "en", Unit: "metric"}},
		{name: "unsafe station", id: "air.station.current", parameters: RequestParameters{Input: catalog.Input{AirStationID: "../secret"}, Language: "en"}},
		{name: "unstable POI type", id: "geo.poi.lookup", parameters: RequestParameters{Input: catalog.Input{Query: "station", POIType: "CSTA"}, Language: "en"}},
		{name: "filters without text", id: "geo.city.lookup", parameters: RequestParameters{Input: catalog.Input{PlaceID: "101010100", Country: "CN"}, Language: "en"}},
		{name: "missing coordinate", id: "air.current", parameters: RequestParameters{Language: "en"}},
	}
	for _, test := range invalid {
		t.Run(test.name, func(t *testing.T) {
			_, problem := CompileRequest(capability(t, test.id), test.parameters)
			if problem == nil || problem.ExitCode != 2 || problem.Code != "INVALID_INVOCATION" || problem.Capability != test.id {
				t.Fatalf("problem = %#v", problem)
			}
		})
	}
}

func TestCompileRequestCoversIssueSevenStormCapabilities(t *testing.T) {
	tests := []struct {
		id       string
		input    catalog.Input
		path     string
		query    url.Values
		ttl      time.Duration
		inactive time.Duration
	}{
		{
			id: "storm.list", input: catalog.Input{Year: 2026}, path: "/v7/tropical/storm-list",
			query: url.Values{"basin": {"NP"}, "year": {"2026"}}, ttl: 20 * time.Minute, inactive: time.Hour,
		},
		{
			id: "storm.track", input: catalog.Input{StormID: "NP_2024"}, path: "/v7/tropical/storm-track",
			query: url.Values{"stormid": {"NP_2024"}}, ttl: 20 * time.Minute, inactive: time.Hour,
		},
		{
			id: "storm.forecast", input: catalog.Input{StormID: "NP_2024"}, path: "/v7/tropical/storm-forecast",
			query: url.Values{"stormid": {"NP_2024"}}, ttl: 20 * time.Minute, inactive: time.Hour,
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{Input: test.input})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
			if capability.BillingGroup != catalog.BillingMarine || capability.ProductGate != catalog.GateMarine || capability.Cache.Mode != catalog.CacheEnabled || capability.Cache.TTL != test.ttl || capability.Cache.InactiveTTL != test.inactive || capability.Cache.Boundary != catalog.BoundaryNone {
				t.Fatalf("policy = billing:%q gate:%q cache:%#v", capability.BillingGroup, capability.ProductGate, capability.Cache)
			}
		})
	}
}

func TestCompileRequestCoversIssueSevenMarineAndAstronomyCapabilities(t *testing.T) {
	tests := []struct {
		id       string
		input    catalog.Input
		resolved place.Resolved
		language string
		path     string
		query    url.Values
		billing  catalog.BillingGroup
		gate     catalog.ProductGate
		ttl      time.Duration
	}{
		{
			id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2026-07-25"},
			path: "/v7/ocean/tide", query: url.Values{"location": {"P66981"}, "date": {"20260725"}},
			billing: catalog.BillingMarine, gate: catalog.GateMarine, ttl: 8 * time.Hour,
		},
		{
			id: "astronomy.sun.events", input: catalog.Input{Date: "2026-07-25"}, resolved: place.Resolved{ID: "101010100"},
			path: "/v7/astronomy/sun", query: url.Values{"location": {"101010100"}, "date": {"20260725"}},
			billing: catalog.BillingBasic, gate: catalog.GateNone, ttl: 24 * time.Hour,
		},
		{
			id: "astronomy.moon.events", input: catalog.Input{Date: "2026-07-25"}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh",
			path: "/v7/astronomy/moon", query: url.Values{"location": {"116.4,39.9"}, "date": {"20260725"}, "lang": {"zh"}},
			billing: catalog.BillingBasic, gate: catalog.GateNone, ttl: 24 * time.Hour,
		},
		{
			id: "astronomy.solar.position", input: catalog.Input{At: "2026-07-25T12:30:00+08:00", AltitudeMeters: 43.5}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			path:    "/v7/astronomy/solar-elevation-angle",
			query:   url.Values{"location": {"116.4,39.9"}, "date": {"20260725"}, "time": {"1230"}, "tz": {"0800"}, "alt": {"43.5"}},
			billing: catalog.BillingBasic, gate: catalog.GateNone, ttl: 24 * time.Hour,
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{
				Input: test.input, Resolved: test.resolved, Language: test.language,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
			if capability.BillingGroup != test.billing || capability.ProductGate != test.gate || capability.Cache.Mode != catalog.CacheEnabled || capability.Cache.TTL != test.ttl || capability.Cache.Boundary != catalog.BoundaryNone {
				t.Fatalf("policy = billing:%q gate:%q cache:%#v", capability.BillingGroup, capability.ProductGate, capability.Cache)
			}
		})
	}
}

func TestCompileRequestCoversIssueSevenSolarAndAccountCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		input    catalog.Input
		changed  map[string]bool
		resolved place.Resolved
		path     string
		query    url.Values
		billing  catalog.BillingGroup
		gate     catalog.ProductGate
		mode     catalog.CacheMode
		ttl      time.Duration
		family   catalog.ResponseFamily
	}{
		{
			name: "solar options", id: "solar.radiation.forecast",
			input: catalog.Input{
				Hours: 12, IntervalMinutes: 15, Includes: []string{"weather", "poa"},
				TiltDegrees: 30, AzimuthDegrees: 180, LocalTime: true,
			},
			changed:  map[string]bool{"hours": true, "interval-min": true, "tilt-deg": true, "azimuth-deg": true},
			resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			path:     "/solarradiation/v1/forecast/39.9/116.4",
			query: url.Values{
				"hours": {"12"}, "interval": {"15"}, "extra": {"poa,weather"},
				"tilt": {"30"}, "azimuth": {"180"}, "localTime": {"true"},
			},
			billing: catalog.BillingSolar, gate: catalog.GateSolar, mode: catalog.CacheEnabled,
			ttl: 6 * time.Hour, family: catalog.ResponseModernV1,
		},
		{
			name: "solar provider defaults", id: "solar.radiation.forecast",
			resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			path:     "/solarradiation/v1/forecast/39.9/116.4",
			query:    url.Values{"hours": {"24"}, "interval": {"60"}},
			billing:  catalog.BillingSolar, gate: catalog.GateSolar, mode: catalog.CacheEnabled,
			ttl: 6 * time.Hour, family: catalog.ResponseModernV1,
		},
		{
			name: "finance summary", id: "account.finance.summary", path: "/finance/v1/summary", query: url.Values{},
			billing: catalog.BillingBasic, gate: catalog.GateSensitiveAccount, mode: catalog.CacheSensitive,
			ttl: 10 * time.Minute, family: catalog.ResponseConsoleV1,
		},
		{
			name: "usage by project", id: "account.requests.stats", input: catalog.Input{ProjectID: "project_123"},
			path: "/metrics/v1/stats", query: url.Values{"project": {"project_123"}},
			billing: catalog.BillingBasic, gate: catalog.GateSensitiveAccount, mode: catalog.CacheSensitive,
			ttl: 10 * time.Minute, family: catalog.ResponseConsoleV1,
		},
		{
			name: "usage by credential", id: "account.requests.stats", input: catalog.Input{CredentialID: "cred_abc"},
			path: "/metrics/v1/stats", query: url.Values{"cred": {"cred_abc"}},
			billing: catalog.BillingBasic, gate: catalog.GateSensitiveAccount, mode: catalog.CacheSensitive,
			ttl: 10 * time.Minute, family: catalog.ResponseConsoleV1,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{
				Input: test.input, Changed: test.changed, Resolved: test.resolved,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
			if capability.BillingGroup != test.billing || capability.ProductGate != test.gate || capability.Cache.Mode != test.mode || capability.Cache.TTL != test.ttl || capability.Cache.Boundary != catalog.BoundaryNone || capability.Upstream.ResponseFamily != test.family {
				t.Fatalf("policy = billing:%q gate:%q cache:%#v family:%q", capability.BillingGroup, capability.ProductGate, capability.Cache, capability.Upstream.ResponseFamily)
			}
		})
	}
}

func TestCompileRequestRejectsIssueSevenInvalidParameters(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		parameters RequestParameters
	}{
		{name: "storm year", id: "storm.list", parameters: RequestParameters{Input: catalog.Input{Year: 2017}}},
		{name: "missing storm ID", id: "storm.track"},
		{name: "missing tide station", id: "marine.tide", parameters: RequestParameters{Input: catalog.Input{Date: "2026-07-25"}}},
		{name: "invalid tide date", id: "marine.tide", parameters: RequestParameters{Input: catalog.Input{TideStationID: "P66981", Date: "2026-02-30"}}},
		{
			name: "missing solar coordinate", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{Hours: 24, IntervalMinutes: 60}},
		},
		{
			name: "zero explicit solar hours", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{Hours: 0}, Changed: map[string]bool{"hours": true}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "invalid solar interval", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{IntervalMinutes: 45}, Changed: map[string]bool{"interval-min": true}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "unknown solar extra", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{Includes: []string{"unknown"}}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "poa without angles", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{Includes: []string{"poa"}}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "invalid solar tilt", id: "solar.radiation.forecast",
			parameters: RequestParameters{Input: catalog.Input{TiltDegrees: 91}, Changed: map[string]bool{"tilt-deg": true}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "invalid sun date", id: "astronomy.sun.events",
			parameters: RequestParameters{Input: catalog.Input{Date: "2026-02-30"}, Resolved: place.Resolved{ID: "101010100"}},
		},
		{name: "missing moon target", id: "astronomy.moon.events", parameters: RequestParameters{Input: catalog.Input{Date: "2026-07-25"}}},
		{
			name: "invalid position timestamp", id: "astronomy.solar.position",
			parameters: RequestParameters{Input: catalog.Input{At: "not-a-time", AltitudeMeters: 43}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "non-finite altitude", id: "astronomy.solar.position",
			parameters: RequestParameters{Input: catalog.Input{At: "2026-07-25T12:30:00+08:00", AltitudeMeters: math.NaN()}, Resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}},
		},
		{
			name: "account filters", id: "account.requests.stats",
			parameters: RequestParameters{Input: catalog.Input{ProjectID: "project_123", CredentialID: "cred_abc"}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, problem := CompileRequest(capability(t, test.id), test.parameters)
			if problem == nil || problem.ExitCode != 2 || problem.Code != "INVALID_INVOCATION" || problem.Capability != test.id {
				t.Fatalf("problem = %#v", problem)
			}
		})
	}
}

func TestIssueSixCapabilitiesExposeExactFlagSets(t *testing.T) {
	placeFlags := []string{"adm", "coordinate", "country", "lang", "place", "place-id"}
	placeUnitFlags := append(append([]string(nil), placeFlags...), "unit")
	expected := map[string][]string{
		"geo.city.lookup":                {"adm", "coordinate", "country", "lang", "limit", "place-id", "query"},
		"geo.city.top":                   {"country", "lang", "limit"},
		"geo.poi.lookup":                 {"adm", "lang", "limit", "poi-type", "query"},
		"geo.poi.nearby":                 {"coordinate", "lang", "limit", "poi-type"},
		"weather.city.current":           placeFlags,
		"weather.city.forecast.daily":    append(append([]string(nil), placeFlags...), "days"),
		"weather.city.forecast.hourly":   append(append([]string(nil), placeFlags...), "hours"),
		"weather.grid.current":           placeUnitFlags,
		"weather.grid.forecast.daily":    append(append([]string(nil), placeUnitFlags...), "days"),
		"weather.grid.forecast.hourly":   append(append([]string(nil), placeUnitFlags...), "hours"),
		"weather.precipitation.minutely": placeFlags,
		"weather.indices.forecast":       append(append([]string(nil), placeFlags...), "all-indices", "days", "index"),
		"weather.history":                append(append([]string(nil), placeUnitFlags...), "date"),
		"alert.current":                  append(append([]string(nil), placeFlags...), "local-time"),
		"air.current":                    placeFlags,
		"air.forecast.daily":             placeFlags,
		"air.forecast.hourly":            placeFlags,
		"air.station.current":            {"air-station-id", "lang"},
	}
	for id, want := range expected {
		capability := capability(t, id)
		got := make([]string, 0, len(capability.Flags))
		for _, flag := range capability.Flags {
			got = append(got, flag.Name)
		}
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s flags=%v want=%v", id, got, want)
		}
	}
}

func TestIssueSevenCapabilitiesExposeExactFlagSets(t *testing.T) {
	placeFlags := []string{"adm", "coordinate", "country", "place", "place-id"}
	placeLanguageFlags := append(append([]string(nil), placeFlags...), "lang")
	expected := map[string][]string{
		"storm.list":               {"allow-product", "year"},
		"storm.track":              {"allow-product", "storm-id"},
		"storm.forecast":           {"allow-product", "storm-id"},
		"marine.tide":              {"allow-product", "date", "tide-station-id"},
		"solar.radiation.forecast": append(append([]string(nil), placeFlags...), "allow-product", "azimuth-deg", "hours", "include", "interval-min", "local-time", "tilt-deg"),
		"astronomy.sun.events":     append(append([]string(nil), placeFlags...), "date"),
		"astronomy.moon.events":    append(append([]string(nil), placeLanguageFlags...), "date"),
		"astronomy.solar.position": append(append([]string(nil), placeFlags...), "altitude-m", "at"),
		"account.finance.summary":  {"allow-sensitive-output"},
		"account.requests.stats":   {"allow-sensitive-output", "credential-id", "project-id"},
	}
	for id, want := range expected {
		capability := capability(t, id)
		got := make([]string, 0, len(capability.Flags))
		for _, flag := range capability.Flags {
			got = append(got, flag.Name)
		}
		sort.Strings(got)
		sort.Strings(want)
		if !slices.Equal(got, want) {
			t.Errorf("%s flags=%v want=%v", id, got, want)
		}
	}

	solar := capability(t, "solar.radiation.forecast")
	for _, flag := range solar.Flags {
		if (flag.Name == "tilt-deg" || flag.Name == "azimuth-deg") && flag.Kind != catalog.FlagInt {
			t.Errorf("solar flag %s kind=%q, want %q", flag.Name, flag.Kind, catalog.FlagInt)
		}
		if flag.Name == "hours" && flag.Default != "24" {
			t.Errorf("solar hours default=%q, want 24", flag.Default)
		}
		if flag.Name == "interval-min" && flag.Default != "60" {
			t.Errorf("solar interval default=%q, want 60", flag.Default)
		}
	}
}

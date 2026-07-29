package app

import (
	"math"
	"net/url"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/place"
)

func TestCompileRequestMapsCoreCapabilities(t *testing.T) {
	tests := []struct {
		id       string
		input    catalog.Input
		resolved place.Resolved
		language string
		unit     string
		path     string
		query    url.Values
	}{
		{
			id: "geo.city.lookup", input: catalog.Input{Query: "Beijing", Country: "CN", Adm: "Beijing", Limit: 20},
			language: "en", path: "/geo/v2/city/lookup",
			query: url.Values{"location": {"Beijing"}, "range": {"cn"}, "adm": {"Beijing"}, "number": {"20"}, "lang": {"en"}},
		},
		{
			id: "geo.city.top", input: catalog.Input{Country: "CN", Limit: 5}, language: "zh", path: "/geo/v2/city/top",
			query: url.Values{"range": {"cn"}, "number": {"5"}, "lang": {"zh"}},
		},
		{
			id: "geo.poi.lookup", input: catalog.Input{Query: "West Lake", POIType: "tide-station", Adm: "101210101", Limit: 10},
			language: "en", path: "/geo/v2/poi/lookup",
			query: url.Values{"location": {"West Lake"}, "type": {"TSTA"}, "city": {"101210101"}, "number": {"10"}, "lang": {"en"}},
		},
		{
			id: "geo.poi.nearby", input: catalog.Input{POIType: "scenic", Limit: 20}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			language: "zh", path: "/geo/v2/poi/range",
			query: url.Values{"location": {"116.4,39.9"}, "type": {"scenic"}, "number": {"20"}, "lang": {"zh"}},
		},
		{
			id: "weather.city.current", resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/weather/now",
			query: url.Values{"location": {"101010100"}, "lang": {"en"}},
		},
		{
			id: "weather.city.forecast.daily", input: catalog.Input{Days: 7}, resolved: place.Resolved{ID: "101010100"}, language: "zh", path: "/v7/weather/7d",
			query: url.Values{"location": {"101010100"}, "lang": {"zh"}},
		},
		{
			id: "weather.city.forecast.hourly", input: catalog.Input{Hours: 168}, resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/weather/168h",
			query: url.Values{"location": {"101010100"}, "lang": {"en"}},
		},
		{
			id: "weather.grid.current", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", unit: "imperial", path: "/v7/grid-weather/now",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"en"}, "unit": {"i"}},
		},
		{
			id: "weather.grid.forecast.daily", input: catalog.Input{Days: 3}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", unit: "metric", path: "/v7/grid-weather/3d",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"zh"}, "unit": {"m"}},
		},
		{
			id: "weather.grid.forecast.hourly", input: catalog.Input{Hours: 72}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", unit: "metric", path: "/v7/grid-weather/72h",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"en"}, "unit": {"m"}},
		},
		{
			id: "weather.precipitation.minutely", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", path: "/v7/minutely/5m",
			query: url.Values{"location": {"116.4,39.9"}, "lang": {"zh"}},
		},
		{
			id: "weather.indices.forecast", input: catalog.Input{Days: 3, Indices: []int{3, 1}}, resolved: place.Resolved{ID: "101010100"}, language: "en", path: "/v7/indices/3d",
			query: url.Values{"location": {"101010100"}, "type": {"1,3"}, "lang": {"en"}},
		},
		{
			id: "weather.history", input: catalog.Input{Date: "2026-07-22"}, resolved: place.Resolved{ID: "101010100"}, language: "en", unit: "metric", path: "/v7/historical/weather",
			query: url.Values{"location": {"101010100"}, "date": {"20260722"}, "lang": {"en"}, "unit": {"m"}},
		},
		{
			id: "alert.current", input: catalog.Input{LocalTime: true}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/weatheralert/v1/current/39.9/116.4",
			query: url.Values{"localTime": {"true"}, "lang": {"en"}},
		},
		{
			id: "air.current", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/airquality/v1/current/39.9/116.4",
			query: url.Values{"lang": {"en"}},
		},
		{
			id: "air.forecast.daily", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh", path: "/airquality/v1/daily/39.9/116.4",
			query: url.Values{"lang": {"zh"}},
		},
		{
			id: "air.forecast.hourly", resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "en", path: "/airquality/v1/hourly/39.9/116.4",
			query: url.Values{"lang": {"en"}},
		},
		{
			id: "air.station.current", input: catalog.Input{AirStationID: "P58911"}, language: "en", path: "/airquality/v1/stations/P58911",
			query: url.Values{"lang": {"en"}},
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

func TestCompileRequestMapsStormCapabilities(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		id    string
		input catalog.Input
		path  string
		query url.Values
	}{
		{
			id: "storm.list", input: catalog.Input{Year: 2026}, path: "/v7/tropical/storm-list",
			query: url.Values{"basin": {"NP"}, "year": {"2026"}},
		},
		{
			id: "storm.track", input: catalog.Input{StormID: "NP_2024"}, path: "/v7/tropical/storm-track",
			query: url.Values{"stormid": {"NP_2024"}},
		},
		{
			id: "storm.forecast", input: catalog.Input{StormID: "NP_2024"}, path: "/v7/tropical/storm-forecast",
			query: url.Values{"stormid": {"NP_2024"}},
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{Input: test.input, Now: now})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
		})
	}
}

func TestCompileRequestMapsMarineAndAstronomyCapabilities(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		id       string
		input    catalog.Input
		resolved place.Resolved
		language string
		path     string
		query    url.Values
	}{
		{
			id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2026-07-25"},
			path: "/v7/ocean/tide", query: url.Values{"location": {"P66981"}, "date": {"20260725"}},
		},
		{
			id: "astronomy.sun.events", input: catalog.Input{Date: "2026-07-25"}, resolved: place.Resolved{ID: "101010100"},
			path: "/v7/astronomy/sun", query: url.Values{"location": {"101010100"}, "date": {"20260725"}},
		},
		{
			id: "astronomy.moon.events", input: catalog.Input{Date: "2026-07-25"}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"}, language: "zh",
			path: "/v7/astronomy/moon", query: url.Values{"location": {"116.4,39.9"}, "date": {"20260725"}, "lang": {"zh"}},
		},
		{
			id: "astronomy.solar.position", input: catalog.Input{At: "2026-07-25T12:30:00+08:00", AltitudeMeters: 43.5}, resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			path:  "/v7/astronomy/solar-elevation-angle",
			query: url.Values{"location": {"116.4,39.9"}, "date": {"20260725"}, "time": {"1230"}, "tz": {"0800"}, "alt": {"43.5"}},
		},
	}

	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			capability := capability(t, test.id)
			request, problem := CompileRequest(capability, RequestParameters{
				Input: test.input, Resolved: test.resolved, Language: test.language, Now: now,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if request.CapabilityID != test.id || request.Path != test.path || request.Query.Encode() != test.query.Encode() {
				t.Fatalf("request=%#v want path=%q query=%q", request, test.path, test.query.Encode())
			}
		})
	}
}

func TestCompileRequestMapsSolarAndAccountCapabilities(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		input    catalog.Input
		changed  map[string]bool
		resolved place.Resolved
		path     string
		query    url.Values
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
		},
		{
			name: "solar provider defaults", id: "solar.radiation.forecast",
			resolved: place.Resolved{Lat: "39.9", Lon: "116.4"},
			path:     "/solarradiation/v1/forecast/39.9/116.4",
			query:    url.Values{"hours": {"24"}, "interval": {"60"}},
		},
		{
			name: "finance summary", id: "account.finance.summary", path: "/finance/v1/summary", query: url.Values{},
		},
		{
			name: "usage by project", id: "account.requests.stats", input: catalog.Input{ProjectID: "project_123"},
			path: "/metrics/v1/stats", query: url.Values{"project": {"project_123"}},
		},
		{
			name: "usage by credential", id: "account.requests.stats", input: catalog.Input{CredentialID: "cred_abc"},
			path: "/metrics/v1/stats", query: url.Values{"cred": {"cred_abc"}},
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
		})
	}
}

func TestCompileRequestRejectsSpecializedInvalidParameters(t *testing.T) {
	now := time.Date(2026, 7, 24, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		id         string
		parameters RequestParameters
	}{
		{name: "storm year", id: "storm.list", parameters: RequestParameters{Input: catalog.Input{Year: 2024}, Now: now}},
		{name: "missing storm ID", id: "storm.track"},
		{name: "missing tide station", id: "marine.tide", parameters: RequestParameters{Input: catalog.Input{Date: "2026-07-25"}}},
		{name: "invalid tide date", id: "marine.tide", parameters: RequestParameters{Input: catalog.Input{TideStationID: "P66981", Date: "2026-02-30"}, Now: now}},
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
			parameters: RequestParameters{Input: catalog.Input{Date: "2026-02-30"}, Resolved: place.Resolved{ID: "101010100"}, Now: now},
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

func TestCompileRequestEnforcesCapabilityDateWindows(t *testing.T) {
	now := time.Date(2026, 12, 29, 1, 30, 0, 0, time.FixedZone("UTC+14", 14*60*60))
	tests := []struct {
		name     string
		id       string
		input    catalog.Input
		resolved place.Resolved
		wantOK   bool
	}{
		{name: "storm current UTC year", id: "storm.list", input: catalog.Input{Year: 2026}, wantOK: true},
		{name: "storm previous UTC year", id: "storm.list", input: catalog.Input{Year: 2025}, wantOK: true},
		{name: "storm year too old", id: "storm.list", input: catalog.Input{Year: 2024}},
		{name: "storm future year", id: "storm.list", input: catalog.Input{Year: 2027}},
		{name: "tide today", id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2026-12-28"}, wantOK: true},
		{name: "tide ninth day ahead", id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2027-01-06"}, wantOK: true},
		{name: "tide yesterday", id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2026-12-27"}},
		{name: "tide tenth day ahead", id: "marine.tide", input: catalog.Input{TideStationID: "P66981", Date: "2027-01-07"}},
		{name: "sun today", id: "astronomy.sun.events", input: catalog.Input{Date: "2026-12-28"}, resolved: place.Resolved{ID: "101010100"}, wantOK: true},
		{name: "sun fifty-ninth day ahead", id: "astronomy.sun.events", input: catalog.Input{Date: "2027-02-25"}, resolved: place.Resolved{ID: "101010100"}, wantOK: true},
		{name: "sun yesterday", id: "astronomy.sun.events", input: catalog.Input{Date: "2026-12-27"}, resolved: place.Resolved{ID: "101010100"}},
		{name: "sun sixtieth day ahead", id: "astronomy.sun.events", input: catalog.Input{Date: "2027-02-26"}, resolved: place.Resolved{ID: "101010100"}},
		{name: "moon today", id: "astronomy.moon.events", input: catalog.Input{Date: "2026-12-28"}, resolved: place.Resolved{ID: "101010100"}, wantOK: true},
		{name: "moon fifty-ninth day ahead", id: "astronomy.moon.events", input: catalog.Input{Date: "2027-02-25"}, resolved: place.Resolved{ID: "101010100"}, wantOK: true},
		{name: "moon yesterday", id: "astronomy.moon.events", input: catalog.Input{Date: "2026-12-27"}, resolved: place.Resolved{ID: "101010100"}},
		{name: "moon sixtieth day ahead", id: "astronomy.moon.events", input: catalog.Input{Date: "2027-02-26"}, resolved: place.Resolved{ID: "101010100"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, problem := CompileRequest(capability(t, test.id), RequestParameters{
				Input: test.input, Resolved: test.resolved, Now: now,
			})
			if test.wantOK && problem != nil {
				t.Fatalf("CompileRequest() problem = %#v", problem)
			}
			if !test.wantOK && (problem == nil || problem.Code != "INVALID_INVOCATION") {
				t.Fatalf("CompileRequest() problem = %#v, want INVALID_INVOCATION", problem)
			}
		})
	}
}

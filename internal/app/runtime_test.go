package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
	cachepkg "github.com/Nativu5/qweather-cli/internal/cache"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/cli"
	"github.com/Nativu5/qweather-cli/internal/config"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

type scriptedDoer struct {
	response  qweather.Response
	err       error
	responses []qweather.Response
	errors    []error
	requests  []qweather.Request
}

func (d *scriptedDoer) Do(_ context.Context, request qweather.Request) (qweather.Response, error) {
	d.requests = append(d.requests, request)
	index := len(d.requests) - 1
	if index < len(d.responses) {
		var err error
		if index < len(d.errors) {
			err = d.errors[index]
		}
		return d.responses[index], err
	}
	return d.response, d.err
}

func testEffective(t *testing.T) config.Effective {
	t.Helper()
	credentials, err := auth.NewAPIKey("secret")
	if err != nil {
		t.Fatal(err)
	}
	return config.Effective{
		APIHost: "example.qweatherapi.com", Profile: "default", Language: "en", Unit: "metric",
		AuthMethod: auth.MethodAPIKey, Credentials: credentials,
	}
}

func capability(t *testing.T, id string) catalog.Capability {
	t.Helper()
	registry, err := catalog.Default()
	if err != nil {
		t.Fatal(err)
	}
	result, ok := registry.Find(id)
	if !ok {
		t.Fatalf("missing capability %s", id)
	}
	return result
}

func TestRuntimeBuildsStableResultFromScriptedProvider(t *testing.T) {
	now := time.Date(2026, 7, 24, 3, 30, 0, 0, time.UTC)
	doer := &scriptedDoer{response: qweather.Response{
		StatusCode: 200,
		Body:       []byte(`{"code":"200","now":{"temp":"20","unknown":true},"refer":{"sources":["QWeather"]}}`),
	}}
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return testEffective(t), config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		func(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
			if !parameters.Now.Equal(now) {
				t.Fatalf("request clock = %s, want %s", parameters.Now, now)
			}
			return qweather.Request{CapabilityID: capability.ID, Path: capability.Upstream.PathTemplate, Query: url.Values{"location": {parameters.Resolved.ID}}}, nil
		},
	)
	runtime.now = func() time.Time { return now }
	result, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "weather.city.current"),
		Common:     cli.CommonOptions{Timeout: time.Second},
		Input:      catalog.Input{PlaceID: "101010100"},
		Changed:    map[string]bool{},
	})
	if problem != nil {
		t.Fatal(problem)
	}
	if result.Schema != output.ResultSchema || result.Outcome != "ok" || result.Capability != "weather.city.current" || len(result.Attribution) != 1 {
		t.Fatalf("result = %#v", result)
	}
	if len(doer.requests) != 1 || doer.requests[0].Query.Get("location") != "101010100" {
		t.Fatalf("requests = %#v", doer.requests)
	}
}

func TestRuntimeMapsConfigurationAndNetworkErrors(t *testing.T) {
	configurationFailure := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return config.Effective{}, config.Diagnostics{}, errors.New("bad configuration")
		}, nil, nil,
	)
	_, problem := configurationFailure.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "geo.city.top"),
		Common:     cli.CommonOptions{Timeout: time.Second},
	})
	if problem == nil || problem.ExitCode != 3 || problem.Code != "CONFIG_INVALID" {
		t.Fatalf("configuration problem = %#v", problem)
	}

	doer := &scriptedDoer{err: &qweather.ClientError{Kind: qweather.ErrorNetwork, Err: context.DeadlineExceeded}}
	networkFailure := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return testEffective(t), config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		func(capability catalog.Capability, _ RequestParameters) (qweather.Request, *output.Problem) {
			return qweather.Request{CapabilityID: capability.ID, Path: "/test"}, nil
		},
	)
	_, problem = networkFailure.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "geo.city.top"),
		Common:     cli.CommonOptions{Timeout: time.Second},
	})
	if problem == nil || problem.ExitCode != 8 || problem.Code != "TIMEOUT" || !problem.Retryable {
		t.Fatalf("network problem = %#v", problem)
	}
}

func TestCheckConfigReportsNotConfiguredProblem(t *testing.T) {
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return config.Effective{}, config.Diagnostics{}, fmt.Errorf("%w: no sources", config.ErrNotConfigured)
		}, nil, nil,
	)

	_, problem := runtime.CheckConfig(context.Background(), cli.CommonOptions{})
	if problem == nil || problem.ExitCode != 3 || problem.Code != "CONFIG_INVALID" || problem.Message != "QWeather is not configured" {
		t.Fatalf("configuration problem = %#v", problem)
	}
}

func TestRuntimePerformsInvocationLocalGeoResolution(t *testing.T) {
	for _, country := range []string{"CN", "cn"} {
		t.Run(country, func(t *testing.T) {
			doer := &scriptedDoer{responses: []qweather.Response{
				{StatusCode: 200, Body: []byte(`{"code":"200","location":[{"id":"101010100","name":"Beijing","adm1":"Beijing","adm2":"Beijing","country":"China","lat":"39.90499","lon":"116.40529","tz":"Asia/Shanghai"}]}`)},
				{StatusCode: 200, Body: []byte(`{"code":"200","now":{"temp":"20"}}`)},
			}}
			compiled := 0
			runtime := New(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					return testEffective(t), config.Diagnostics{}, nil
				},
				func(config.Effective) (qweather.Doer, error) { return doer, nil },
				func(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
					compiled++
					if parameters.Resolved.ID != "101010100" || parameters.Resolved.TZ != "Asia/Shanghai" || parameters.Language != "en" {
						t.Fatalf("parameters = %#v", parameters)
					}
					return qweather.Request{CapabilityID: capability.ID, Path: "/v7/weather/now", Query: url.Values{"location": {parameters.Resolved.ID}}}, nil
				},
			)
			result, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, "weather.city.current"),
				Input: catalog.Input{
					Place: "Beijing", Country: country, Adm: "Beijing",
				},
				Common:  cli.CommonOptions{Timeout: time.Second},
				Changed: map[string]bool{},
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if compiled != 1 || len(doer.requests) != 2 {
				t.Fatalf("compiled=%d requests=%#v", compiled, doer.requests)
			}
			lookup := doer.requests[0]
			if lookup.Path != "/geo/v2/city/lookup" || lookup.Query.Get("location") != "Beijing" || lookup.Query.Get("range") != "cn" || lookup.Query.Get("adm") != "Beijing" || lookup.Query.Get("number") != "20" || lookup.Query.Get("lang") != "en" {
				t.Fatalf("lookup = %#v", lookup)
			}
			if result.ResolvedPlace == nil || result.ResolvedPlace.ID != "101010100" || len(result.Operations) != 2 || result.Operations[0] != "geo.city.lookup" {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestRuntimeAvoidsGeoForCompatibleCoordinate(t *testing.T) {
	doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(`{"code":"200","now":{}}`)}}
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return testEffective(t), config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		func(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
			if parameters.Resolved.Lat != "39.9" || parameters.Resolved.Lon != "116.4" {
				t.Fatalf("resolved = %#v", parameters.Resolved)
			}
			return qweather.Request{CapabilityID: capability.ID, Path: "/v7/grid-weather/now"}, nil
		},
	)
	result, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "weather.grid.current"),
		Input:      catalog.Input{Coordinate: "geo:39.90,116.40"},
		Common:     cli.CommonOptions{Timeout: time.Second},
		Changed:    map[string]bool{},
	})
	if problem != nil {
		t.Fatal(problem)
	}
	if len(doer.requests) != 1 || len(result.Operations) != 1 || result.ResolvedPlace == nil || result.ResolvedPlace.Lat != "39.9" {
		t.Fatalf("result=%#v requests=%#v", result, doer.requests)
	}
}

func TestRuntimeMapsAmbiguousAndMissingPlaces(t *testing.T) {
	tests := []struct {
		name string
		body string
		code string
	}{
		{
			name: "ambiguous",
			body: `{"code":"200","location":[{"id":"one","name":"Springfield","adm1":"A","lat":"10","lon":"20"},{"id":"two","name":"Springfield","adm1":"B","lat":"30","lon":"40"}]}`,
			code: "AMBIGUOUS_PLACE",
		},
		{name: "missing", body: `{"code":"204"}`, code: "PLACE_NOT_FOUND"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(test.body)}}
			compiled := 0
			runtime := New(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					return testEffective(t), config.Diagnostics{}, nil
				},
				func(config.Effective) (qweather.Doer, error) { return doer, nil },
				func(catalog.Capability, RequestParameters) (qweather.Request, *output.Problem) {
					compiled++
					return qweather.Request{}, nil
				},
			)
			_, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, "air.current"),
				Input:      catalog.Input{Place: "Springfield"},
				Common:     cli.CommonOptions{Timeout: time.Second},
				Changed:    map[string]bool{},
			})
			if problem == nil || problem.ExitCode != 5 || problem.Code != test.code || problem.Capability != "air.current" || compiled != 0 || len(doer.requests) != 1 {
				t.Fatalf("problem=%#v compiled=%d requests=%#v", problem, compiled, doer.requests)
			}
		})
	}
}

func TestProductGatePrecedesConfigurationAndNetwork(t *testing.T) {
	ids := []string{
		"storm.list", "storm.track", "storm.forecast", "marine.tide",
		"solar.radiation.forecast", "account.finance.summary", "account.requests.stats",
	}
	for _, id := range ids {
		t.Run(id, func(t *testing.T) {
			loads := 0
			clients := 0
			runtime := New(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					loads++
					return config.Effective{}, config.Diagnostics{}, errors.New("must not run")
				},
				func(config.Effective) (qweather.Doer, error) {
					clients++
					return &scriptedDoer{}, nil
				}, nil,
			)
			_, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, id),
				Input:      catalog.Input{Place: "Beijing"},
				Common:     cli.CommonOptions{Timeout: time.Second},
			})
			if problem == nil || problem.ExitCode != 4 || problem.Code != "PRODUCT_GATE_REQUIRED" || problem.Message != "this capability requires --yes before network I/O" || loads != 0 || clients != 0 {
				t.Fatalf("problem=%#v loads=%d clients=%d", problem, loads, clients)
			}
		})
	}
}

func TestProductGateUsesInvocationAcknowledgement(t *testing.T) {
	for _, id := range []string{"storm.list", "solar.radiation.forecast", "account.finance.summary"} {
		t.Run(id, func(t *testing.T) {
			invocation := cli.Invocation{
				Capability:       capability(t, id),
				GateAcknowledged: true,
			}
			if problem := checkProductGate(invocation); problem != nil {
				t.Fatalf("acknowledged gate returned %#v", problem)
			}
		})
	}
}

func TestRuntimeExecutesIssueSevenResponseFamiliesWithAcknowledgement(t *testing.T) {
	tests := []struct {
		name        string
		id          string
		input       catalog.Input
		body        string
		path        string
		family      string
		attribution int
	}{
		{
			name: "marine code-refer", id: "storm.track",
			input: catalog.Input{StormID: "NP_2024"},
			body:  `{"code":"200","isActive":"1","futureField":{"kept":true},"refer":{"sources":["QWeather"]}}`,
			path:  "/v7/tropical/storm-track", family: "code-refer-v1", attribution: 1,
		},
		{
			name: "solar metadata", id: "solar.radiation.forecast",
			input: catalog.Input{Coordinate: "geo:39.9,116.4"},
			body:  `{"metadata":{"attributions":[{"name":"QWeather"}]},"forecasts":[],"futureField":{"kept":true}}`,
			path:  "/solarradiation/v1/forecast/39.9/116.4", family: "metadata-v1", attribution: 1,
		},
		{
			name: "account console", id: "account.finance.summary",
			input: catalog.Input{},
			body:  `{"metadata":{"attributions":[{"name":"QWeather"}]},"balance":10,"futureField":{"kept":true}}`,
			path:  "/finance/v1/summary", family: "console-v1", attribution: 1,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effective := testEffective(t)
			effective.Cache.Enabled = false
			doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(test.body)}}
			runtime := New(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					return effective, config.Diagnostics{}, nil
				},
				func(config.Effective) (qweather.Doer, error) { return doer, nil },
				nil,
			)
			result, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, test.id), Input: test.input, GateAcknowledged: true,
				Common: cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if len(doer.requests) != 1 || doer.requests[0].Path != test.path {
				t.Fatalf("requests = %#v", doer.requests)
			}
			if result.Upstream.ResponseFamily != test.family || len(result.Attribution) != test.attribution || string(result.ProviderBody) != test.body || !strings.Contains(string(result.Data), `"futureField"`) {
				t.Fatalf("result = %#v", result)
			}
		})
	}
}

func TestConfigCheckDoesNotCreateProviderClient(t *testing.T) {
	factories := 0
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return testEffective(t), config.Diagnostics{AuthSource: "test"}, nil
		},
		func(config.Effective) (qweather.Doer, error) {
			factories++
			return &scriptedDoer{}, nil
		}, nil,
	)
	result, problem := runtime.CheckConfig(context.Background(), cli.CommonOptions{})
	if problem != nil || factories != 0 {
		t.Fatalf("result=%#v problem=%#v factories=%d", result, problem, factories)
	}
	check := result.(config.CheckResult)
	if !check.Valid || check.Diagnostics.AuthSource != "test" {
		t.Fatalf("check = %#v", check)
	}
}

func TestRuntimeCacheHitRefreshAndNoCache(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	directory := filepath.Join(t.TempDir(), "cache")
	effective := testEffective(t)
	effective.Cache = config.CacheSettings{Enabled: true, Directory: directory}
	doer := &scriptedDoer{responses: []qweather.Response{
		{StatusCode: 200, Body: []byte(`{"code":"200","now":{"temp":"20"}}`)},
		{StatusCode: 200, Body: []byte(`{"code":"200","now":{"temp":"21"}}`)},
		{StatusCode: 200, Body: []byte(`{"code":"200","now":{"temp":"22"}}`)},
	}}
	runtime := NewWithCache(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return effective, config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		func(effective config.Effective) (CacheStore, error) {
			return cachepkg.NewStore(effective.Cache.Directory, effective.Profile, func() time.Time { return now })
		},
		func(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
			return qweather.Request{CapabilityID: capability.ID, Path: "/v7/weather/now", Query: url.Values{"location": {parameters.Resolved.ID}, "lang": {parameters.Language}}}, nil
		},
	)
	runtime.now = func() time.Time { return now }
	invocation := cli.Invocation{
		Capability: capability(t, "weather.city.current"), Input: catalog.Input{PlaceID: "101010100"},
		Common: cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
	}
	first, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || first.Cache.Status != "miss" || first.Cache.StoredAt == "" || len(doer.requests) != 1 {
		t.Fatalf("first=%#v problem=%v requests=%d", first, problem, len(doer.requests))
	}
	now = now.Add(time.Minute)
	second, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || second.Cache.Status != "hit" || second.Cache.UpstreamRequested || second.Cache.AgeSeconds != 60 || len(doer.requests) != 1 || !strings.Contains(string(second.Data), `"20"`) {
		t.Fatalf("second=%#v problem=%v requests=%d", second, problem, len(doer.requests))
	}
	invocation.Common.Refresh = true
	refreshed, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || refreshed.Cache.Status != "miss" || !refreshed.Cache.UpstreamRequested || len(doer.requests) != 2 || !strings.Contains(string(refreshed.Data), `"21"`) {
		t.Fatalf("refreshed=%#v problem=%v requests=%d", refreshed, problem, len(doer.requests))
	}
	invocation.Common.Refresh = false
	invocation.Common.NoCache = true
	bypassed, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || bypassed.Cache.Status != "disabled" || len(doer.requests) != 3 || !strings.Contains(string(bypassed.Data), `"22"`) {
		t.Fatalf("bypassed=%#v problem=%v requests=%d", bypassed, problem, len(doer.requests))
	}
	invocation.Common.NoCache = false
	afterBypass, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || afterBypass.Cache.Status != "hit" || len(doer.requests) != 3 || !strings.Contains(string(afterBypass.Data), `"21"`) {
		t.Fatalf("afterBypass=%#v problem=%v requests=%d", afterBypass, problem, len(doer.requests))
	}
}

func TestRuntimeUsesStormResponseTTL(t *testing.T) {
	now := time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name      string
		body      string
		ttl       time.Duration
		second    string
		wantCalls int
	}{
		{name: "active", body: `{"code":"200","isActive":"1"}`, ttl: 20 * time.Minute, second: "miss", wantCalls: 2},
		{name: "inactive", body: `{"code":"200","isActive":"0"}`, ttl: time.Hour, second: "hit", wantCalls: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			effective := testEffective(t)
			effective.Cache = config.CacheSettings{Enabled: true, Directory: filepath.Join(t.TempDir(), "cache")}
			doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(test.body)}}
			runtime := NewWithCache(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					return effective, config.Diagnostics{}, nil
				},
				func(config.Effective) (qweather.Doer, error) { return doer, nil },
				func(effective config.Effective) (CacheStore, error) {
					return cachepkg.NewStore(effective.Cache.Directory, effective.Profile, func() time.Time { return now })
				},
				nil,
			)
			runtime.now = func() time.Time { return now }
			result, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, "storm.track"),
				Input:      catalog.Input{StormID: "NP_2024"},
				Common:     cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
				GateAcknowledged: true,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			want := now.Add(test.ttl).Format(time.RFC3339)
			if result.Cache.Status != "miss" || result.Cache.ExpiresAt != want || len(doer.requests) != 1 {
				t.Fatalf("cache=%#v requests=%d want expiry=%s", result.Cache, len(doer.requests), want)
			}
			now = now.Add(30 * time.Minute)
			second, problem := runtime.Run(context.Background(), cli.Invocation{
				Capability: capability(t, "storm.track"),
				Input:      catalog.Input{StormID: "NP_2024"},
				Common:     cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
				GateAcknowledged: true,
			})
			if problem != nil {
				t.Fatal(problem)
			}
			if second.Cache.Status != test.second || len(doer.requests) != test.wantCalls {
				t.Fatalf("second cache=%#v requests=%d", second.Cache, len(doer.requests))
			}
			now = time.Date(2026, 7, 24, 0, 0, 0, 0, time.UTC)
		})
	}
}

func TestRuntimeNeverCachesGeoCapabilities(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "cache")
	effective := testEffective(t)
	effective.Cache = config.CacheSettings{Enabled: true, Sensitive: true, Directory: directory}
	doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(`{"code":"200","topCityList":[]}`)}}
	runtime := NewWithCache(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return effective, config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil }, nil,
		func(capability catalog.Capability, _ RequestParameters) (qweather.Request, *output.Problem) {
			return qweather.Request{CapabilityID: capability.ID, Path: capability.Upstream.PathTemplate}, nil
		},
	)
	invocation := cli.Invocation{Capability: capability(t, "geo.city.top"), Common: cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{}}
	for range 2 {
		result, problem := runtime.Run(context.Background(), invocation)
		if problem != nil || result.Cache.Status != "disabled" {
			t.Fatalf("result=%#v problem=%v", result, problem)
		}
	}
	if len(doer.requests) != 2 {
		t.Fatalf("Geo data request count = %d", len(doer.requests))
	}
	if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Geo query created cache storage: %v", err)
	}
}

func TestRuntimeRequiresSensitiveCacheOptInForAccount(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	for _, test := range []struct {
		name      string
		sensitive bool
		wantCalls int
		wantLast  string
	}{
		{name: "default disabled", sensitive: false, wantCalls: 2, wantLast: "disabled"},
		{name: "explicit opt-in", sensitive: true, wantCalls: 1, wantLast: "hit"},
	} {
		t.Run(test.name, func(t *testing.T) {
			effective := testEffective(t)
			effective.Cache = config.CacheSettings{Enabled: true, Sensitive: test.sensitive, Directory: filepath.Join(t.TempDir(), "cache")}
			doer := &scriptedDoer{response: qweather.Response{StatusCode: 200, Body: []byte(`{"balance":10}`)}}
			runtime := NewWithCache(
				func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
					return effective, config.Diagnostics{}, nil
				},
				func(config.Effective) (qweather.Doer, error) { return doer, nil },
				func(effective config.Effective) (CacheStore, error) {
					return cachepkg.NewStore(effective.Cache.Directory, effective.Profile, func() time.Time { return now })
				},
				func(capability catalog.Capability, _ RequestParameters) (qweather.Request, *output.Problem) {
					return qweather.Request{CapabilityID: capability.ID, Path: capability.Upstream.PathTemplate}, nil
				},
			)
			runtime.now = func() time.Time { return now }
			invocation := cli.Invocation{
				Capability:       capability(t, "account.finance.summary"),
				GateAcknowledged: true,
				Common:           cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
			}
			var last *output.Result
			for range 2 {
				var problem *output.Problem
				last, problem = runtime.Run(context.Background(), invocation)
				if problem != nil {
					t.Fatal(problem)
				}
			}
			if len(doer.requests) != test.wantCalls || last.Cache.Status != test.wantLast {
				t.Fatalf("requests=%d last=%#v", len(doer.requests), last)
			}
		})
	}
}

func TestRuntimeNeverCachesProviderErrors(t *testing.T) {
	now := time.Date(2026, 7, 23, 0, 0, 0, 0, time.UTC)
	effective := testEffective(t)
	effective.Cache = config.CacheSettings{Enabled: true, Directory: filepath.Join(t.TempDir(), "cache")}
	doer := &scriptedDoer{
		responses: []qweather.Response{
			{},
			{StatusCode: 200, Body: []byte(`{"code":"200","now":{"temp":"20"}}`)},
		},
		errors: []error{&qweather.ClientError{Kind: qweather.ErrorNetwork, Err: errors.New("temporary failure")}},
	}
	runtime := NewWithCache(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return effective, config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		func(effective config.Effective) (CacheStore, error) {
			return cachepkg.NewStore(effective.Cache.Directory, effective.Profile, func() time.Time { return now })
		},
		func(capability catalog.Capability, parameters RequestParameters) (qweather.Request, *output.Problem) {
			return qweather.Request{CapabilityID: capability.ID, Path: "/v7/weather/now", Query: url.Values{"location": {parameters.Resolved.ID}}}, nil
		},
	)
	runtime.now = func() time.Time { return now }
	invocation := cli.Invocation{
		Capability: capability(t, "weather.city.current"), Input: catalog.Input{PlaceID: "101010100"},
		Common: cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
	}
	if _, problem := runtime.Run(context.Background(), invocation); problem == nil || problem.ExitCode != 8 {
		t.Fatalf("first problem = %#v", problem)
	}
	second, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || second.Cache.Status != "miss" || len(doer.requests) != 2 {
		t.Fatalf("second=%#v problem=%v requests=%d", second, problem, len(doer.requests))
	}
	third, problem := runtime.Run(context.Background(), invocation)
	if problem != nil || third.Cache.Status != "hit" || len(doer.requests) != 2 {
		t.Fatalf("third=%#v problem=%v requests=%d", third, problem, len(doer.requests))
	}
}

func TestDefaultCompilerExecutesMetadataResponseFamily(t *testing.T) {
	effective := testEffective(t)
	effective.Cache.Enabled = false
	doer := &scriptedDoer{response: qweather.Response{
		StatusCode: 200,
		Body:       []byte(`{"metadata":{"attributions":[{"name":"QWeather"}]},"indexes":[],"futureField":{"kept":true}}`),
	}}
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return effective, config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) { return doer, nil },
		nil,
	)
	result, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "air.current"), Input: catalog.Input{Coordinate: "geo:39.90,116.40"},
		Common: cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
	})
	if problem != nil {
		t.Fatal(problem)
	}
	if len(doer.requests) != 1 || doer.requests[0].Path != "/airquality/v1/current/39.9/116.4" || doer.requests[0].Query.Get("lang") != "en" {
		t.Fatalf("requests = %#v", doer.requests)
	}
	if result.Upstream.ResponseFamily != "metadata-v1" || len(result.Attribution) != 1 || !strings.Contains(string(result.Data), `"futureField"`) {
		t.Fatalf("result = %#v", result)
	}
}

func TestProviderResponseUnitMatchesCapabilityContract(t *testing.T) {
	tests := []struct {
		capabilityID string
		effective    string
		want         string
	}{
		{capabilityID: "weather.grid.current", effective: "imperial", want: "imperial"},
		{capabilityID: "weather.history", effective: "metric", want: "metric"},
		{capabilityID: "weather.city.current", effective: "imperial", want: "metric"},
		{capabilityID: "weather.precipitation.minutely", effective: "imperial", want: "metric"},
		{capabilityID: "storm.track", effective: "imperial", want: ""},
	}
	for _, test := range tests {
		t.Run(test.capabilityID, func(t *testing.T) {
			if got := providerResponseUnit(capability(t, test.capabilityID), test.effective); got != test.want {
				t.Fatalf("providerResponseUnit() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCapabilityLanguageValidationPrecedesClientAndGeo(t *testing.T) {
	effective := testEffective(t)
	effective.Language = "fr"
	clients := 0
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			return effective, config.Diagnostics{}, nil
		},
		func(config.Effective) (qweather.Doer, error) {
			clients++
			return &scriptedDoer{}, nil
		},
		nil,
	)
	_, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "weather.indices.forecast"),
		Input:      catalog.Input{Place: "Beijing", Days: 1, AllIndices: true},
		Common:     cli.CommonOptions{Timeout: time.Second}, Changed: map[string]bool{},
	})
	if problem == nil || problem.ExitCode != 2 || clients != 0 {
		t.Fatalf("problem=%#v clients=%d", problem, clients)
	}
}

package app

import (
	"context"
	"errors"
	"net/url"
	"testing"
	"time"

	"github.com/Nativu5/qweather-cli/internal/auth"
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
			return qweather.Request{CapabilityID: capability.ID, Path: capability.Upstream.PathTemplate, Query: url.Values{"location": {parameters.Resolved.ID}}}, nil
		},
	)
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

func TestRuntimePerformsInvocationLocalGeoResolution(t *testing.T) {
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
			Place: "Beijing", Country: "CN", Adm: "Beijing",
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
	if lookup.Path != "/geo/v2/city/lookup" || lookup.Query.Get("location") != "Beijing" || lookup.Query.Get("range") != "CN" || lookup.Query.Get("adm") != "Beijing" || lookup.Query.Get("number") != "20" || lookup.Query.Get("lang") != "en" {
		t.Fatalf("lookup = %#v", lookup)
	}
	if result.ResolvedPlace == nil || result.ResolvedPlace.ID != "101010100" || len(result.Operations) != 2 || result.Operations[0] != "geo.city.lookup" {
		t.Fatalf("result = %#v", result)
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
	loads := 0
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			loads++
			return config.Effective{}, config.Diagnostics{}, errors.New("must not run")
		}, nil, nil,
	)
	_, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "solar.radiation.forecast"),
		Input:      catalog.Input{Place: "Beijing"},
		Common:     cli.CommonOptions{Timeout: time.Second},
	})
	if problem == nil || problem.ExitCode != 4 || problem.Code != "PRODUCT_GATE_REQUIRED" || loads != 0 {
		t.Fatalf("problem=%#v loads=%d", problem, loads)
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

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
	response qweather.Response
	err      error
	requests []qweather.Request
}

func (d *scriptedDoer) Do(_ context.Context, request qweather.Request) (qweather.Response, error) {
	d.requests = append(d.requests, request)
	return d.response, d.err
}

func testEffective(t *testing.T) config.Effective {
	t.Helper()
	credentials, err := auth.NewAPIKey("secret")
	if err != nil {
		t.Fatal(err)
	}
	return config.Effective{APIHost: "example.qweatherapi.com", AuthMethod: auth.MethodAPIKey, Credentials: credentials}
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
		func(capability catalog.Capability, _ catalog.Input, _ map[string]bool) (qweather.Request, *output.Problem) {
			return qweather.Request{CapabilityID: capability.ID, Path: capability.Upstream.PathTemplate, Query: url.Values{"location": {"101010100"}}}, nil
		},
	)
	result, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "weather.city.current"),
		Common:     cli.CommonOptions{Timeout: time.Second},
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
		func(capability catalog.Capability, _ catalog.Input, _ map[string]bool) (qweather.Request, *output.Problem) {
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

func TestProductGatePrecedesConfigurationAndNetwork(t *testing.T) {
	loads := 0
	runtime := New(
		func(context.Context, config.Options) (config.Effective, config.Diagnostics, error) {
			loads++
			return config.Effective{}, config.Diagnostics{}, errors.New("must not run")
		}, nil, nil,
	)
	_, problem := runtime.Run(context.Background(), cli.Invocation{
		Capability: capability(t, "storm.list"),
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

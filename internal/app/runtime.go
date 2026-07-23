package app

import (
	"context"
	"fmt"
	"net/url"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/cli"
	"github.com/Nativu5/qweather-cli/internal/config"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

type ConfigLoader func(context.Context, config.Options) (config.Effective, config.Diagnostics, error)
type ClientFactory func(config.Effective) (qweather.Doer, error)

type RequestParameters struct {
	Input    catalog.Input
	Changed  map[string]bool
	Language string
	Unit     string
	Resolved place.Resolved
}

type RequestCompiler func(catalog.Capability, RequestParameters) (qweather.Request, *output.Problem)

type Runtime struct {
	loadConfig ConfigLoader
	newClient  ClientFactory
	compile    RequestCompiler
}

func New(loadConfig ConfigLoader, newClient ClientFactory, compile RequestCompiler) *Runtime {
	if loadConfig == nil {
		loadConfig = config.Load
	}
	if newClient == nil {
		newClient = func(effective config.Effective) (qweather.Doer, error) {
			return qweather.NewClient(effective.APIHost, effective.Credentials, qweather.ClientOptions{})
		}
	}
	if compile == nil {
		compile = unavailableCompiler
	}
	return &Runtime{loadConfig: loadConfig, newClient: newClient, compile: compile}
}

func NewDefault() *Runtime {
	return New(nil, nil, nil)
}

func (r *Runtime) Run(ctx context.Context, invocation cli.Invocation) (*output.Result, *output.Problem) {
	if problem := checkProductGate(invocation); problem != nil {
		return nil, problem
	}
	options := config.Options{ConfigPath: invocation.Common.ConfigPath, Profile: invocation.Common.Profile}
	if invocation.Changed["lang"] {
		value := invocation.Input.Language
		options.LanguageOverride = &value
	}
	if invocation.Changed["unit"] {
		value := invocation.Input.Unit
		options.UnitOverride = &value
	}
	effective, _, err := r.loadConfig(ctx, options)
	if err != nil {
		return nil, configProblem(invocation.Capability.ID, err)
	}
	client, err := r.newClient(effective)
	if err != nil {
		problem := output.NewProblem(3, "CONFIG_INVALID", "provider client configuration is invalid")
		problem.Capability = invocation.Capability.ID
		problem.Cause = err
		return nil, problem
	}
	requestContext, cancel := context.WithTimeout(ctx, invocation.Common.Timeout)
	defer cancel()
	resolved, operations, problem := r.resolvePlace(requestContext, client, invocation, effective.Language)
	if problem != nil {
		problem.Capability = invocation.Capability.ID
		return nil, problem
	}
	request, problem := r.compile(invocation.Capability, RequestParameters{
		Input: invocation.Input, Changed: invocation.Changed,
		Language: effective.Language, Unit: effective.Unit, Resolved: resolved,
	})
	if problem != nil {
		return nil, problem
	}
	response, err := client.Do(requestContext, request)
	if err != nil {
		return nil, qweather.ProblemForError(err, invocation.Capability.ID)
	}
	classified, problem := qweather.Classify(invocation.Capability.Upstream.ResponseFamily, response, invocation.Capability.ID)
	if problem != nil {
		return nil, problem
	}
	operations = append(operations, invocation.Capability.ID)
	result := &output.Result{
		Schema:       output.ResultSchema,
		Outcome:      classified.Outcome,
		Capability:   invocation.Capability.ID,
		Operations:   operations,
		Policy:       output.Policy{BillingGroup: string(invocation.Capability.BillingGroup)},
		Cache:        output.Cache{Status: "disabled", UpstreamRequested: true},
		Upstream:     output.Upstream{HTTPStatus: response.StatusCode, ResponseFamily: string(invocation.Capability.Upstream.ResponseFamily)},
		Attribution:  classified.Attribution,
		Data:         classified.Data,
		ProviderBody: append([]byte(nil), response.Body...),
	}
	if isPlaceTarget(invocation.Capability.Target) {
		result.ResolvedPlace = resolved.Output()
	}
	return result, nil
}

func (r *Runtime) resolvePlace(ctx context.Context, client qweather.Doer, invocation cli.Invocation, language string) (place.Resolved, []string, *output.Problem) {
	if !isPlaceTarget(invocation.Capability.Target) {
		return place.Resolved{}, nil, nil
	}
	spec, err := place.Parse(invocation.Input.Place, invocation.Input.PlaceID, invocation.Input.Coordinate, invocation.Input.Country, invocation.Input.Adm)
	if err != nil {
		problem := output.NewProblem(2, "INVALID_INVOCATION", err.Error())
		problem.Capability = invocation.Capability.ID
		return place.Resolved{}, nil, problem
	}
	lookup := func(ctx context.Context, query place.LookupQuery) ([]place.Candidate, *output.Problem) {
		values := url.Values{"number": {"20"}}
		switch query.Spec.Kind {
		case place.SpecName:
			values.Set("location", query.Spec.Name)
			if query.Spec.Country != "" {
				values.Set("range", query.Spec.Country)
			}
			if query.Spec.Adm != "" {
				values.Set("adm", query.Spec.Adm)
			}
		case place.SpecLocationID:
			values.Set("location", query.Spec.LocationID)
		case place.SpecCoordinate:
			values.Set("location", query.Spec.Coordinate.ProviderQuery())
		default:
			return nil, output.NewProblem(10, "INTERNAL_ERROR", "place lookup received an unknown Place Spec")
		}
		if query.Language != "" && query.Language != "auto" {
			values.Set("lang", query.Language)
		}
		response, err := client.Do(ctx, qweather.Request{
			CapabilityID: "geo.city.lookup",
			Path:         "/geo/v2/city/lookup",
			Query:        values,
		})
		if err != nil {
			return nil, qweather.ProblemForError(err, invocation.Capability.ID)
		}
		classified, problem := qweather.Classify(catalog.ResponseLegacyV1, response, invocation.Capability.ID)
		if problem != nil {
			return nil, problem
		}
		if classified.Outcome == "no_data" {
			return nil, nil
		}
		candidates, decodeErr := place.DecodeCandidates(classified.Data)
		if decodeErr != nil {
			problem := output.NewProblem(9, "UPSTREAM_PROTOCOL_ERROR", "GeoAPI returned invalid place candidates")
			problem.Capability = invocation.Capability.ID
			problem.Cause = decodeErr
			return nil, problem
		}
		return candidates, nil
	}
	return place.Resolve(ctx, spec, invocation.Capability.Target, language, lookup)
}

func isPlaceTarget(target catalog.TargetKind) bool {
	return target == catalog.TargetPlace || target == catalog.TargetLocationID || target == catalog.TargetCoordinate
}

func (r *Runtime) CheckConfig(ctx context.Context, common cli.CommonOptions) (any, *output.Problem) {
	effective, diagnostics, err := r.loadConfig(ctx, config.Options{ConfigPath: common.ConfigPath, Profile: common.Profile})
	if err != nil {
		return nil, configProblem("", err)
	}
	return config.CheckResult{Valid: true, Effective: effective, Diagnostics: diagnostics}, nil
}

func (*Runtime) CacheStatus(context.Context, cli.CacheControlOptions) (any, *output.Problem) {
	return nil, output.NewProblem(10, "CONTROL_NOT_IMPLEMENTED", "persistent cache is not implemented")
}

func (*Runtime) CacheClear(context.Context, cli.CacheControlOptions) (any, *output.Problem) {
	return nil, output.NewProblem(10, "CONTROL_NOT_IMPLEMENTED", "persistent cache is not implemented")
}

func unavailableCompiler(capability catalog.Capability, _ RequestParameters) (qweather.Request, *output.Problem) {
	problem := output.NewProblem(10, "CAPABILITY_NOT_IMPLEMENTED", "capability request mapping is not implemented")
	problem.Capability = capability.ID
	return qweather.Request{}, problem
}

func checkProductGate(invocation cli.Invocation) *output.Problem {
	acknowledged := true
	switch invocation.Capability.ProductGate {
	case catalog.GateMarine:
		acknowledged = invocation.Input.AllowProduct == "marine"
	case catalog.GateSolar:
		acknowledged = invocation.Input.AllowProduct == "solar"
	case catalog.GateSensitiveAccount:
		acknowledged = invocation.Input.AllowSensitive == "account"
	}
	if acknowledged {
		return nil
	}
	problem := output.NewProblem(4, "PRODUCT_GATE_REQUIRED", "required product or sensitive-output acknowledgement is missing")
	problem.Capability = invocation.Capability.ID
	return problem
}

func configProblem(capabilityID string, err error) *output.Problem {
	problem := output.NewProblem(3, "CONFIG_INVALID", "QWeather configuration is invalid")
	problem.Capability = capabilityID
	problem.Details = map[string]any{"reason": fmt.Sprintf("%v", err)}
	problem.Cause = err
	return problem
}

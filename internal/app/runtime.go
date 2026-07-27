package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	cachepkg "github.com/Nativu5/qweather-cli/internal/cache"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/cli"
	"github.com/Nativu5/qweather-cli/internal/config"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/Nativu5/qweather-cli/internal/place"
	"github.com/Nativu5/qweather-cli/internal/qweather"
)

type ConfigLoader func(context.Context, config.Options) (config.Effective, config.Diagnostics, error)
type ClientFactory func(config.Effective) (qweather.Doer, error)
type CacheFactory func(config.Effective) (CacheStore, error)

type CacheStore interface {
	Get(context.Context, cachepkg.Key) (cachepkg.Record, bool, error)
	Put(context.Context, cachepkg.Key, cachepkg.Record) error
	Delete(context.Context, cachepkg.Key) error
	Status(context.Context, bool, bool) (cachepkg.Status, error)
	Clear(context.Context, string, bool) (cachepkg.ClearResult, error)
}

type RequestParameters struct {
	Input    catalog.Input
	Changed  map[string]bool
	Language string
	Unit     string
	Resolved place.Resolved
	Now      time.Time
}

type RequestCompiler func(catalog.Capability, RequestParameters) (qweather.Request, *output.Problem)

type Runtime struct {
	loadConfig ConfigLoader
	newClient  ClientFactory
	newCache   CacheFactory
	compile    RequestCompiler
	now        func() time.Time
}

func New(loadConfig ConfigLoader, newClient ClientFactory, compile RequestCompiler) *Runtime {
	return NewWithCache(loadConfig, newClient, nil, compile)
}

func NewWithCache(loadConfig ConfigLoader, newClient ClientFactory, newCache CacheFactory, compile RequestCompiler) *Runtime {
	if loadConfig == nil {
		loadConfig = config.Load
	}
	if newClient == nil {
		newClient = func(effective config.Effective) (qweather.Doer, error) {
			return qweather.NewClient(effective.APIHost, effective.Credentials, qweather.ClientOptions{})
		}
	}
	if newCache == nil {
		newCache = func(effective config.Effective) (CacheStore, error) {
			return cachepkg.NewStore(effective.Cache.Directory, effective.Profile, nil)
		}
	}
	if compile == nil {
		compile = CompileRequest
	}
	return &Runtime{loadConfig: loadConfig, newClient: newClient, newCache: newCache, compile: compile, now: time.Now}
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
	if problem := validateEffectiveCapability(invocation.Capability, effective); problem != nil {
		return nil, problem
	}
	client, err := r.newClient(effective)
	if err != nil {
		problem := output.NewProblem(3, output.CodeConfigInvalid, "provider client configuration is invalid")
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
		Now: r.now().UTC(),
	})
	if problem != nil {
		return nil, problem
	}
	presentationUnit := providerResponseUnit(invocation.Capability, effective.Unit)
	operations = append(operations, invocation.Capability.ID)
	cacheMetadata := output.Cache{Status: "disabled", UpstreamRequested: true}
	cacheEnabled := cachepkg.Enabled(invocation.Capability, effective.Cache.Enabled, effective.Cache.Sensitive) && !invocation.Common.NoCache
	var store CacheStore
	var cacheKey cachepkg.Key
	if cacheEnabled {
		store, err = r.newCache(effective)
		if err != nil {
			return nil, cacheProblem(invocation.Capability.ID, err)
		}
		cacheKey, err = cachepkg.BuildKey(invocation.Capability, cachepkg.Material{
			APIHost: effective.APIHost, Profile: effective.Profile,
			EffectiveLang: effective.Language, EffectiveUnit: effective.Unit,
			AllowSensitive: effective.Cache.Sensitive,
			Input:          invocation.Input, Resolved: resolved, Request: request,
		})
		if err != nil {
			return nil, cacheProblem(invocation.Capability.ID, err)
		}
		cacheMetadata.Status = "miss"
		if !invocation.Common.Refresh {
			record, hit, cacheErr := store.Get(requestContext, cacheKey)
			if cacheErr != nil {
				return nil, cacheProblem(invocation.Capability.ID, cacheErr)
			}
			if hit {
				response := qweather.Response{StatusCode: record.HTTPStatus, Body: append([]byte(nil), record.ProviderBody...)}
				classified, cachedProblem := qweather.Classify(record.ResponseFamily, response, invocation.Capability.ID)
				if cachedProblem == nil && classified.Outcome == record.Outcome && record.ResponseFamily == invocation.Capability.Upstream.ResponseFamily {
					age := r.now().UTC().Sub(record.StoredAt).Seconds()
					if age < 0 {
						age = 0
					}
					cacheMetadata = output.Cache{
						Status: "hit", StoredAt: record.StoredAt.UTC().Format(time.RFC3339),
						ExpiresAt: record.ExpiresAt.UTC().Format(time.RFC3339), AgeSeconds: int64(age),
						UpstreamRequested: false,
					}
					return buildResult(invocation.Capability, resolved, operations, response, classified, cacheMetadata, presentationUnit), nil
				}
				_ = store.Delete(requestContext, cacheKey)
			}
		}
	}
	response, err := client.Do(requestContext, request)
	if err != nil {
		return nil, qweather.ProblemForError(err, invocation.Capability.ID)
	}
	classified, problem := qweather.Classify(invocation.Capability.Upstream.ResponseFamily, response, invocation.Capability.ID)
	if problem != nil {
		return nil, problem
	}
	if cacheEnabled {
		storedAt := r.now().UTC()
		responsePolicy := cachepkg.PolicyForResponse(invocation.Capability, classified.Outcome, classified.Data)
		expiresAt, expirationErr := cachepkg.Expiration(storedAt, responsePolicy, resolved.TZ)
		if expirationErr != nil {
			return nil, cacheProblem(invocation.Capability.ID, expirationErr)
		}
		record, recordErr := cachepkg.NewRecord(invocation.Capability, classified.Outcome, response, storedAt, expiresAt)
		if recordErr != nil {
			return nil, cacheProblem(invocation.Capability.ID, recordErr)
		}
		if putErr := store.Put(requestContext, cacheKey, record); putErr == nil {
			cacheMetadata.StoredAt = storedAt.Format(time.RFC3339)
			cacheMetadata.ExpiresAt = expiresAt.Format(time.RFC3339)
		}
	}
	return buildResult(invocation.Capability, resolved, operations, response, classified, cacheMetadata, presentationUnit), nil
}

func providerResponseUnit(capability catalog.Capability, effectiveUnit string) string {
	for _, flag := range capability.Flags {
		if flag.Name == "unit" {
			return effectiveUnit
		}
	}
	switch capability.ID {
	case "weather.city.current", "weather.city.forecast.daily", "weather.city.forecast.hourly", "weather.precipitation.minutely":
		return "metric"
	default:
		return ""
	}
}

func validateEffectiveCapability(capability catalog.Capability, effective config.Effective) *output.Problem {
	if capability.ID == "weather.indices.forecast" || capability.ID == "weather.precipitation.minutely" {
		return validateLimitedLanguage(effective.Language, capability.ID)
	}
	return nil
}

func buildResult(capability catalog.Capability, resolved place.Resolved, operations []string, response qweather.Response, classified qweather.Classified, cacheMetadata output.Cache, unit string) *output.Result {
	result := &output.Result{
		Schema:       output.ResultSchema,
		Outcome:      classified.Outcome,
		Capability:   capability.ID,
		Operations:   operations,
		Policy:       output.Policy{BillingGroup: string(capability.BillingGroup)},
		Cache:        cacheMetadata,
		Upstream:     output.Upstream{HTTPStatus: response.StatusCode, ResponseFamily: string(capability.Upstream.ResponseFamily)},
		Attribution:  classified.Attribution,
		Data:         classified.Data,
		ProviderBody: append([]byte(nil), response.Body...),
		Unit:         unit,
	}
	if isPlaceTarget(capability.Target) {
		result.ResolvedPlace = resolved.Output()
	}
	return result
}

func (r *Runtime) resolvePlace(ctx context.Context, client qweather.Doer, invocation cli.Invocation, language string) (place.Resolved, []string, *output.Problem) {
	if !isPlaceTarget(invocation.Capability.Target) {
		return place.Resolved{}, nil, nil
	}
	spec, err := place.Parse(invocation.Input.Place, invocation.Input.PlaceID, invocation.Input.Coordinate, invocation.Input.Country, invocation.Input.Adm)
	if err != nil {
		problem := output.NewProblem(2, output.CodeInvalidInvocation, err.Error())
		problem.Capability = invocation.Capability.ID
		return place.Resolved{}, nil, problem
	}
	lookup := func(ctx context.Context, query place.LookupQuery) ([]place.Candidate, *output.Problem) {
		values := url.Values{"number": {"20"}}
		switch query.Spec.Kind {
		case place.SpecName:
			values.Set("location", query.Spec.Name)
			if country := providerCountry(query.Spec.Country); country != "" {
				values.Set("range", country)
			}
			if query.Spec.Adm != "" {
				values.Set("adm", query.Spec.Adm)
			}
		case place.SpecLocationID:
			values.Set("location", query.Spec.LocationID)
		case place.SpecCoordinate:
			values.Set("location", query.Spec.Coordinate.ProviderQuery())
		default:
			return nil, output.NewProblem(10, output.CodeInternalError, "place lookup received an unknown Place Spec")
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
		classified, problem := qweather.Classify(catalog.ResponseCodeReferV1, response, invocation.Capability.ID)
		if problem != nil {
			return nil, problem
		}
		if classified.Outcome == "no_data" {
			return nil, nil
		}
		candidates, decodeErr := place.DecodeCandidates(classified.Data)
		if decodeErr != nil {
			problem := output.NewProblem(9, output.CodeUpstreamProtocolError, "GeoAPI returned invalid place candidates")
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

func (r *Runtime) CacheStatus(ctx context.Context, options cli.CacheControlOptions) (any, *output.Problem) {
	effective, _, err := r.loadConfig(ctx, config.Options{ConfigPath: options.Common.ConfigPath, Profile: options.Common.Profile})
	if err != nil {
		return nil, configProblem("", err)
	}
	store, err := r.newCache(effective)
	if err != nil {
		return nil, cacheProblem("", err)
	}
	status, err := store.Status(ctx, effective.Cache.Enabled, effective.Cache.Sensitive)
	if err != nil {
		return nil, cacheProblem("", err)
	}
	return status, nil
}

func (r *Runtime) CacheClear(ctx context.Context, options cli.CacheControlOptions) (any, *output.Problem) {
	effective, _, err := r.loadConfig(ctx, config.Options{ConfigPath: options.Common.ConfigPath, Profile: options.Common.Profile})
	if err != nil {
		return nil, configProblem("", err)
	}
	store, err := r.newCache(effective)
	if err != nil {
		return nil, cacheProblem("", err)
	}
	result, err := store.Clear(ctx, options.CapabilityID, options.AllProfiles)
	if err != nil {
		return nil, cacheProblem("", err)
	}
	return result, nil
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
	problem := output.NewProblem(4, output.CodeProductGateRequired, "required product or sensitive-output acknowledgement is missing")
	problem.Capability = invocation.Capability.ID
	return problem
}

func configProblem(capabilityID string, err error) *output.Problem {
	message := "QWeather configuration is invalid"
	if errors.Is(err, config.ErrNotConfigured) {
		message = "QWeather is not configured"
	}
	problem := output.NewProblem(3, output.CodeConfigInvalid, message)
	problem.Capability = capabilityID
	problem.Details = map[string]any{"reason": fmt.Sprintf("%v", err)}
	problem.Cause = err
	return problem
}

func cacheProblem(capabilityID string, err error) *output.Problem {
	problem := output.NewProblem(10, output.CodeCacheIOError, "persistent cache operation failed")
	problem.Capability = capabilityID
	problem.Cause = err
	return problem
}

package cli

import (
	"context"
	"time"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
)

type CommonOptions struct {
	ConfigPath string
	Profile    string
	Timeout    time.Duration
	Output     string
	Refresh    bool
	NoCache    bool
	Debug      bool
}

type Invocation struct {
	Capability catalog.Capability
	Input      catalog.Input
	Common     CommonOptions
	Changed    map[string]bool
}

type CacheControlOptions struct {
	Common       CommonOptions
	CapabilityID string
	AllProfiles  bool
}

type Runtime interface {
	Run(context.Context, Invocation) (*output.Result, *output.Problem)
	CheckConfig(context.Context, CommonOptions) (any, *output.Problem)
	CacheStatus(context.Context, CacheControlOptions) (any, *output.Problem)
	CacheClear(context.Context, CacheControlOptions) (any, *output.Problem)
}

type UnavailableRuntime struct{}

func (UnavailableRuntime) Run(_ context.Context, invocation Invocation) (*output.Result, *output.Problem) {
	problem := output.NewProblem(10, "CAPABILITY_NOT_IMPLEMENTED", "capability implementation is not available")
	problem.Capability = invocation.Capability.ID
	return nil, problem
}

func (UnavailableRuntime) CheckConfig(context.Context, CommonOptions) (any, *output.Problem) {
	return nil, output.NewProblem(10, "CONTROL_NOT_IMPLEMENTED", "configuration checking is not available")
}

func (UnavailableRuntime) CacheStatus(context.Context, CacheControlOptions) (any, *output.Problem) {
	return nil, output.NewProblem(10, "CONTROL_NOT_IMPLEMENTED", "cache status is not available")
}

func (UnavailableRuntime) CacheClear(context.Context, CacheControlOptions) (any, *output.Problem) {
	return nil, output.NewProblem(10, "CONTROL_NOT_IMPLEMENTED", "cache clearing is not available")
}

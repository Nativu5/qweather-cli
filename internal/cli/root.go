package cli

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func NewRoot(registry *catalog.Registry, runtime Runtime, info buildinfo.Info) (*cobra.Command, error) {
	if registry == nil {
		return nil, fmt.Errorf("registry is required")
	}
	if runtime == nil {
		runtime = UnavailableRuntime{}
	}
	capabilityIDs := make([]string, 0, len(registry.Current()))
	for _, capability := range registry.Current() {
		capabilityIDs = append(capabilityIDs, capability.ID)
	}
	renderer, err := output.NewRenderer(capabilityIDs)
	if err != nil {
		return nil, fmt.Errorf("construct output renderer: %w", err)
	}
	common := &CommonOptions{}
	root := &cobra.Command{
		Use:           "qweather",
		Short:         "Query QWeather through a stable, agent-friendly CLI",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.CompletionOptions.DisableDefaultCmd = true
	root.PersistentFlags().StringVar(&common.ConfigPath, "config", "", "TOML configuration file")
	root.PersistentFlags().StringVar(&common.Profile, "profile", "", "configuration profile")
	root.PersistentFlags().DurationVar(&common.Timeout, "timeout", 10*time.Second, "invocation deadline")
	root.PersistentFlags().StringVar(&common.Output, "output", "text", "output mode: text, json, or body")
	root.PersistentFlags().BoolVar(&common.Refresh, "refresh", false, "skip cache reads and replace a successful entry")
	root.PersistentFlags().BoolVar(&common.NoCache, "no-cache", false, "skip cache reads and writes")
	root.PersistentFlags().BoolVar(&common.Debug, "debug", false, "write secret-free diagnostics to stderr")

	if err := addNetworkCommands(root, registry, runtime, common, renderer); err != nil {
		return nil, err
	}
	root.AddCommand(newCapabilityCommand(registry, common))
	root.AddCommand(newConfigCommand(runtime, common))
	root.AddCommand(newCacheCommand(runtime, common, registry))
	root.AddCommand(newVersionCommand(info, common))
	return root, nil
}

func Execute(ctx context.Context, root *cobra.Command, args []string, stdout, stderr io.Writer) int {
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)
	root.SetContext(ctx)
	if err := root.Execute(); err != nil {
		problem, ok := err.(*output.Problem)
		if !ok {
			if renderErr := output.RenderCobraError(stderr, err); renderErr != nil {
				return 10
			}
			return 2
		}
		mode := selectedMode(root)
		if renderErr := output.RenderProblem(stderr, problem, mode); renderErr != nil {
			_, _ = fmt.Fprintf(stderr, "failed to write command error: %v\n", renderErr)
			return 10
		}
		if problem.ExitCode == 0 {
			return 10
		}
		return problem.ExitCode
	}
	return 0
}

func selectedMode(root *cobra.Command) output.Mode {
	value, err := root.PersistentFlags().GetString("output")
	if err != nil {
		return output.ModeText
	}
	mode := output.Mode(value)
	if !mode.Valid() {
		return output.ModeText
	}
	return mode
}

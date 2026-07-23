package cli

import (
	"context"
	"fmt"
	"io"
	"strings"
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
	root.PersistentFlags().StringVar(&common.Output, "output", "json", "output format: json or body")
	root.PersistentFlags().BoolVar(&common.Pretty, "pretty", false, "pretty-print JSON output")
	root.PersistentFlags().BoolVar(&common.Refresh, "refresh", false, "skip cache reads and replace a successful entry")
	root.PersistentFlags().BoolVar(&common.NoCache, "no-cache", false, "skip cache reads and writes")
	root.PersistentFlags().BoolVar(&common.Debug, "debug", false, "write secret-free diagnostics to stderr")

	if err := addNetworkCommands(root, registry, runtime, common); err != nil {
		return nil, err
	}
	root.AddCommand(newCapabilityCommand(registry))
	root.AddCommand(newConfigCommand(runtime, common))
	root.AddCommand(newCacheCommand(runtime, common, registry))
	root.AddCommand(newVersionCommand(info))
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
			problem = output.NewProblem(2, "INVALID_INVOCATION", cleanCobraError(err))
		}
		pretty, _ := root.Flags().GetBool("pretty")
		if renderErr := output.RenderProblem(stderr, problem, pretty); renderErr != nil {
			_, _ = fmt.Fprintf(stderr, "{\"schema\":%q,\"code\":%q,\"message\":%q,\"retryable\":false}\n", output.ProblemSchema, "INTERNAL_ERROR", renderErr.Error())
			return 10
		}
		if problem.ExitCode == 0 {
			return 10
		}
		return problem.ExitCode
	}
	return 0
}

func cleanCobraError(err error) string {
	message := strings.TrimSpace(err.Error())
	if message == "" {
		return "invalid command invocation"
	}
	return message
}

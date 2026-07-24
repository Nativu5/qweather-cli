package cli

import (
	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCommand(runtime Runtime, common *CommonOptions) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Inspect QWeather CLI configuration"}
	command.AddCommand(&cobra.Command{
		Use:   "check",
		Short: "Validate effective configuration without a provider request",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			result, problem := runtime.CheckConfig(command.Context(), *common)
			if problem != nil {
				return problem
			}
			return renderLocalResult(command, result, *common)
		},
	})
	return command
}

func newCacheCommand(runtime Runtime, common *CommonOptions, registry *catalog.Registry) *cobra.Command {
	command := &cobra.Command{Use: "cache", Short: "Inspect or clear the persistent cache"}
	command.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show cache statistics without revealing targets",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			result, problem := runtime.CacheStatus(command.Context(), CacheControlOptions{Common: *common})
			if problem != nil {
				return problem
			}
			return renderLocalResult(command, result, *common)
		},
	})
	var capabilityID string
	var allProfiles bool
	clear := &cobra.Command{
		Use:   "clear",
		Short: "Clear cache entries",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			if capabilityID != "" {
				capability, ok := registry.Find(capabilityID)
				if !ok || capability.Lifecycle != catalog.LifecycleCurrent {
					return invalid("", "--capability must name a Current Capability")
				}
			}
			result, problem := runtime.CacheClear(command.Context(), CacheControlOptions{
				Common:       *common,
				CapabilityID: capabilityID,
				AllProfiles:  allProfiles,
			})
			if problem != nil {
				return problem
			}
			return renderLocalResult(command, result, *common)
		},
	}
	clear.Flags().StringVar(&capabilityID, "capability", "", "clear only one capability")
	clear.Flags().BoolVar(&allProfiles, "all-profiles", false, "clear every profile explicitly")
	command.AddCommand(clear)
	return command
}

func validateLocalOutput(common CommonOptions) *output.Problem {
	if common.Output == string(output.ModeBody) {
		return invalid("", "--output body is not available for local control commands")
	}
	if common.Output != string(output.ModeText) && common.Output != string(output.ModeJSON) {
		return invalid("", "--output must be text or json for local control commands")
	}
	return nil
}

func renderLocalResult(command *cobra.Command, result any, common CommonOptions) error {
	var err error
	if common.Output == string(output.ModeJSON) {
		err = output.WriteJSON(command.OutOrStdout(), result)
	} else {
		err = output.WriteValueText(command.OutOrStdout(), result)
	}
	if err == nil {
		return nil
	}
	return outputFailure(err)
}

func outputFailure(err error) error {
	if err == nil {
		return nil
	}
	problem := output.NewProblem(10, "OUTPUT_ERROR", "failed to write command output")
	problem.Cause = err
	return problem
}

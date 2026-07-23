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
			result, problem := runtime.CheckConfig(command.Context(), *common)
			if problem != nil {
				return problem
			}
			return output.WriteJSON(command.OutOrStdout(), result, common.Pretty)
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
			result, problem := runtime.CacheStatus(command.Context(), CacheControlOptions{Common: *common})
			if problem != nil {
				return problem
			}
			return output.WriteJSON(command.OutOrStdout(), result, common.Pretty)
		},
	})
	var capabilityID string
	var allProfiles bool
	clear := &cobra.Command{
		Use:   "clear",
		Short: "Clear cache entries",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
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
			return output.WriteJSON(command.OutOrStdout(), result, common.Pretty)
		},
	}
	clear.Flags().StringVar(&capabilityID, "capability", "", "clear only one capability")
	clear.Flags().BoolVar(&allProfiles, "all-profiles", false, "clear every profile explicitly")
	command.AddCommand(clear)
	return command
}

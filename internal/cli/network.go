package cli

import (
	"fmt"
	"strings"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func addNetworkCommands(root *cobra.Command, registry *catalog.Registry, runtime Runtime, common *CommonOptions) error {
	nodes := map[string]*cobra.Command{"": root}
	for _, capability := range registry.Current() {
		parts := strings.Fields(capability.CommandPath)
		if len(parts) < 2 {
			return fmt.Errorf("invalid command path %q", capability.CommandPath)
		}
		parentKey := ""
		for index, part := range parts {
			key := strings.Join(parts[:index+1], " ")
			if existing, ok := nodes[key]; ok {
				parentKey = key
				_ = existing
				continue
			}
			parent := nodes[parentKey]
			if index == len(parts)-1 {
				leaf, err := newNetworkLeaf(capability, runtime, common)
				if err != nil {
					return err
				}
				parent.AddCommand(leaf)
				nodes[key] = leaf
			} else {
				branch := &cobra.Command{Use: part, Short: branchSummary(key)}
				parent.AddCommand(branch)
				nodes[key] = branch
			}
			parentKey = key
		}
	}
	return nil
}

func branchSummary(path string) string {
	return "QWeather " + path + " commands"
}

func newNetworkLeaf(capability catalog.Capability, runtime Runtime, common *CommonOptions) (*cobra.Command, error) {
	input := &catalog.Input{}
	parts := strings.Fields(capability.CommandPath)
	command := &cobra.Command{
		Use:   parts[len(parts)-1],
		Short: capability.Summary,
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			changed := changedFlags(command)
			if problem := validateInvocation(capability, *input, *common, changed); problem != nil {
				return problem
			}
			if common.Debug {
				_ = output.WriteJSON(command.ErrOrStderr(), map[string]any{
					"schema":     "qweather.debug/v1",
					"event":      "query.start",
					"capability": capability.ID,
				}, false)
			}
			result, problem := runtime.Run(command.Context(), Invocation{
				Capability: capability,
				Input:      *input,
				Common:     *common,
				Changed:    changed,
			})
			if problem != nil {
				return problem
			}
			if err := output.RenderResult(command.OutOrStdout(), result, common.Output == "body", common.Pretty); err != nil {
				problem := output.NewProblem(10, "OUTPUT_ERROR", "failed to write command output")
				problem.Cause = err
				return problem
			}
			return nil
		},
	}
	if err := bindCapabilityFlags(command, input, capability.Flags); err != nil {
		return nil, fmt.Errorf("%s: %w", capability.ID, err)
	}
	return command, nil
}

func changedFlags(command *cobra.Command) map[string]bool {
	changed := make(map[string]bool)
	command.Flags().Visit(func(flag *pflag.Flag) {
		changed[flag.Name] = true
	})
	return changed
}

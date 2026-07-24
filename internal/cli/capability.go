package cli

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/Nativu5/qweather-cli/internal/catalog"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func newCapabilityCommand(registry *catalog.Registry, common *CommonOptions) *cobra.Command {
	command := &cobra.Command{Use: "capability", Short: "Inspect the offline capability catalog"}
	command.AddCommand(newCapabilityListCommand(registry, common))
	command.AddCommand(newCapabilityShowCommand(registry, common))
	return command
}

func newCapabilityListCommand(registry *catalog.Registry, common *CommonOptions) *cobra.Command {
	var domain, billing, lifecycle string
	command := &cobra.Command{
		Use:   "list",
		Short: "List capabilities without network access",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			if lifecycle != "current" && lifecycle != "deprecated" && lifecycle != "all" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--lifecycle must be current, deprecated, or all")
			}
			if billing != "" && billing != "basic" && billing != "marine" && billing != "solar" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--billing-group must be basic, marine, or solar")
			}
			records := registry.All()
			filtered := make([]catalog.Capability, 0, len(records))
			for _, record := range records {
				if lifecycle != "all" && string(record.Lifecycle) != lifecycle {
					continue
				}
				if domain != "" && record.Domain != domain {
					continue
				}
				if billing != "" && string(record.BillingGroup) != billing {
					continue
				}
				filtered = append(filtered, record)
			}
			sort.Slice(filtered, func(i, j int) bool { return filtered[i].ID < filtered[j].ID })
			if common.Output == string(output.ModeJSON) {
				return renderLocalResult(command, filtered, *common)
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 4, 2, ' ', 0)
			_, _ = fmt.Fprintln(writer, "ID\tCOMMAND\tBILLING\tLIFECYCLE")
			for _, record := range filtered {
				commandPath := record.CommandPath
				if commandPath == "" {
					commandPath = "-"
				}
				_, _ = fmt.Fprintf(writer, "%s\t%s\t%s\t%s\n", record.ID, commandPath, record.BillingGroup, record.Lifecycle)
			}
			return outputFailure(writer.Flush())
		},
	}
	command.Flags().StringVar(&domain, "domain", "", "filter by capability domain")
	command.Flags().StringVar(&billing, "billing-group", "", "filter by billing group")
	command.Flags().StringVar(&lifecycle, "lifecycle", "current", "filter by lifecycle")
	return command
}

func newCapabilityShowCommand(registry *catalog.Registry, common *CommonOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "show <capability-id>",
		Short: "Show one capability without network access",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			record, ok := registry.Find(args[0])
			if !ok {
				return output.NewProblem(2, "UNKNOWN_CAPABILITY", "unknown capability ID")
			}
			if common.Output == string(output.ModeJSON) {
				return renderLocalResult(command, record, *common)
			}
			writer := tabwriter.NewWriter(command.OutOrStdout(), 0, 4, 2, ' ', 0)
			fields := [][2]string{
				{"ID", record.ID},
				{"Summary", record.Summary},
				{"Lifecycle", string(record.Lifecycle)},
				{"Command", valueOrDash(record.CommandPath)},
				{"Billing group", string(record.BillingGroup)},
				{"Product gate", string(record.ProductGate)},
				{"Target", string(record.Target)},
				{"Upstream", strings.TrimSpace(record.Upstream.Method + " " + record.Upstream.PathTemplate)},
				{"Documentation", record.DocsURL},
			}
			if record.Replacement != "" {
				fields = append(fields, [2]string{"Replacement", record.Replacement})
			}
			for _, field := range fields {
				_, _ = fmt.Fprintf(writer, "%s:\t%s\n", field[0], field[1])
			}
			return outputFailure(writer.Flush())
		},
	}
	return command
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

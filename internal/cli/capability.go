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

func newCapabilityCommand(registry *catalog.Registry) *cobra.Command {
	command := &cobra.Command{Use: "capability", Short: "Inspect the offline capability catalog"}
	command.AddCommand(newCapabilityListCommand(registry))
	command.AddCommand(newCapabilityShowCommand(registry))
	return command
}

func newCapabilityListCommand(registry *catalog.Registry) *cobra.Command {
	var domain, billing, lifecycle, format string
	command := &cobra.Command{
		Use:   "list",
		Short: "List capabilities without network access",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if lifecycle != "current" && lifecycle != "deprecated" && lifecycle != "all" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--lifecycle must be current, deprecated, or all")
			}
			if billing != "" && billing != "basic" && billing != "marine" && billing != "solar" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--billing-group must be basic, marine, or solar")
			}
			if format != "table" && format != "json" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--format must be table or json")
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
			if format == "json" {
				return output.WriteJSON(command.OutOrStdout(), filtered, false)
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
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&domain, "domain", "", "filter by capability domain")
	command.Flags().StringVar(&billing, "billing-group", "", "filter by billing group")
	command.Flags().StringVar(&lifecycle, "lifecycle", "current", "filter by lifecycle")
	command.Flags().StringVar(&format, "format", "table", "output format: table or json")
	return command
}

func newCapabilityShowCommand(registry *catalog.Registry) *cobra.Command {
	var format string
	command := &cobra.Command{
		Use:   "show <capability-id>",
		Short: "Show one capability without network access",
		Args:  cobra.ExactArgs(1),
		RunE: func(command *cobra.Command, args []string) error {
			if format != "text" && format != "json" {
				return output.NewProblem(2, "INVALID_INVOCATION", "--format must be text or json")
			}
			record, ok := registry.Find(args[0])
			if !ok {
				return output.NewProblem(2, "UNKNOWN_CAPABILITY", "unknown capability ID")
			}
			if format == "json" {
				return output.WriteJSON(command.OutOrStdout(), record, false)
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
			return writer.Flush()
		},
	}
	command.Flags().StringVar(&format, "format", "text", "output format: text or json")
	return command
}

func valueOrDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

package cli

import (
	"fmt"

	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func newVersionCommand(info buildinfo.Info) *cobra.Command {
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "version",
		Short: "Show build and registry version information",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if jsonOutput {
				return output.WriteJSON(command.OutOrStdout(), info, false)
			}
			_, err := fmt.Fprintf(command.OutOrStdout(), "qweather %s\n", info.Version)
			return err
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit machine-readable version information")
	return command
}

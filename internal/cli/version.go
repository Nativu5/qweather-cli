package cli

import (
	"fmt"

	"github.com/Nativu5/qweather-cli/internal/buildinfo"
	"github.com/Nativu5/qweather-cli/internal/output"
	"github.com/spf13/cobra"
)

func newVersionCommand(info buildinfo.Info, common *CommonOptions) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Show build and registry version information",
		Args:  cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			if problem := validateLocalOutput(*common); problem != nil {
				return problem
			}
			if common.Output == string(output.ModeJSON) {
				return renderLocalResult(command, info, *common)
			}
			_, err := fmt.Fprintf(command.OutOrStdout(), "qweather %s\n", info.Version)
			return outputFailure(err)
		},
	}
}

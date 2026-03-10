package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/chapsuk/wait-jobs/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version information",
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprintln(cmd.OutOrStdout(), buildinfo.String())
		},
	}
}

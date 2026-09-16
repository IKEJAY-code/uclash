package cli

import (
	"fmt"

	"gitee.com/IKEJAY-code/uclash/internal/version"
	"github.com/spf13/cobra"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "uclash %s (commit %s, built %s)\n", version.Version, version.Commit, version.Date)
			a, err := newApp()
			if err != nil {
				return err
			}
			fmt.Fprintf(out, "config: %s\n", a.ConfigPath)
			fmt.Fprintf(out, "data:   %s\n", a.DataDir)
			return nil
		},
	}
}

package cli

import (
	"gitee.com/IKEJAY-code/uclash/internal/app"
	"github.com/spf13/cobra"
)

var (
	flagDataDir    string
	flagConfigFile string
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "uclash",
		Short:         "Per-user, sudo-free mihomo (Clash.Meta) proxy manager",
		Long: `uclash runs mihomo entirely inside your home directory.

No sudo, no systemd, no shared state: every user gets their own core
process, ports, secret and dashboard. Stopping your instance never
touches another user's.`,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	pf.StringVar(&flagDataDir, "data-dir", "", "data directory (default $UCLASH_HOME or ~/.local/share/uclash)")
	pf.StringVar(&flagConfigFile, "config", "", "config file (default $UCLASH_CONFIG or ~/.config/uclash/config.yaml)")

	root.AddCommand(
		newInitCmd(),
		newStartCmd(),
		newStopCmd(),
		newRestartCmd(),
		newStatusCmd(),
		newSubCmd(),
		newProfileCmd(),
		newNodeCmd(),
		newEnvCmd(),
		newUICmd(),
		newModeCmd(),
		newPortCmd(),
		newLogCmd(),
		newCoreCmd(),
		newShellCmd(),
		newDoctorCmd(),
		newVersionCmd(),
		newUninstallCmd(),
	)
	return root
}

func newApp() (*app.App, error) { return app.New(flagConfigFile, flagDataDir) }

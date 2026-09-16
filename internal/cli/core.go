package cli

import (
	"fmt"
	"os/exec"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/core"
	"gitee.com/IKEJAY-code/uclash/internal/fsx"
	"github.com/spf13/cobra"
)

func newCoreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "core",
		Short: "Inspect or update the mihomo core binary",
	}
	cmd.AddCommand(newCoreInfoCmd(), newCoreUpdateCmd())
	return cmd
}

func newCoreInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show core binary and version information",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "path:    %s\n", a.CorePath())
			fmt.Fprintf(out, "present: %v\n", fsx.Exists(a.CorePath()))
			fmt.Fprintf(out, "pinned:  %s\n", orDash(a.Cfg.Core.Version))
			if v := coreBinaryVersion(a.CorePath()); v != "" {
				fmt.Fprintf(out, "binary:  %s\n", v)
			}
			if client, ok := a.Client(); ok {
				if v, err := client.Version(cmd.Context()); err == nil {
					fmt.Fprintf(out, "running: %s\n", v)
				}
			}
			return nil
		},
	}
}

func newCoreUpdateCmd() *cobra.Command {
	var (
		version string
		mirror  string
	)
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Download a newer mihomo core (restarts the core if it was running)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if mirror != "" {
				a.Cfg.Mirror = mirror
			}
			_, running := a.Running()
			if running {
				if err := a.Manager().Stop(cmd.Context()); err != nil {
					return err
				}
			}
			tag, err := core.FetchCore(cmd.Context(), version, a.DownloadOptions(true), a.CorePath())
			if err != nil {
				if running {
					fmt.Fprintln(out, "update failed; restarting the previous core")
					_ = startApp(cmd, a, true)
				}
				return err
			}
			a.Cfg.Core.Version = tag
			if err := a.Save(); err != nil {
				return err
			}
			fmt.Fprintf(out, "core: mihomo %s installed\n", tag)
			if running {
				if err := startApp(cmd, a, true); err != nil {
					return err
				}
				fmt.Fprintln(out, "core: restarted")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&version, "version", "", "pin a specific release tag (default: latest stable)")
	cmd.Flags().StringVar(&mirror, "mirror", "", "GitHub mirror prefix (persisted)")
	return cmd
}

func coreBinaryVersion(path string) string {
	if !fsx.Exists(path) {
		return ""
	}
	out, err := exec.Command(path, "-v").CombinedOutput()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

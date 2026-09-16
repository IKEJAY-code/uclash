package cli

import (
	"fmt"
	"os"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/shellenv"
	"github.com/spf13/cobra"
)

func newShellCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "shell",
		Short: "Manage shell integration (proxyon/proxyoff helpers)",
	}
	cmd.AddCommand(
		&cobra.Command{
			Use:   "status",
			Short: "Show shell integration status",
			RunE: func(cmd *cobra.Command, args []string) error {
				a, err := newApp()
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				rc := a.Cfg.Shell.RCFile
				if rc == "" {
					rc = shellenv.DetectRC()
				}
				fmt.Fprintf(out, "rc file:   %s\n", orDash(rc))
				if rc != "" {
					fmt.Fprintf(out, "installed: %v\n", shellenv.Installed(rc))
				}
				return nil
			},
		},
		&cobra.Command{
			Use:   "install",
			Short: "Add proxyon/proxyoff helpers to your shell rc file",
			RunE: func(cmd *cobra.Command, args []string) error {
				a, err := newApp()
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				rc := shellenv.DetectRC()
				if rc == "" {
					return fmt.Errorf("could not detect an rc file; add this to your shell config manually:\n\n%s", shellenv.Block())
				}
				changed, err := shellenv.Install(rc)
				if err != nil {
					return err
				}
				a.Cfg.Shell.Integration = true
				a.Cfg.Shell.RCFile = rc
				if err := a.Save(); err != nil {
					return err
				}
				if changed {
					fmt.Fprintf(out, "installed into %s\n", rc)
				} else {
					fmt.Fprintf(out, "already present in %s\n", rc)
				}
				fmt.Fprintf(out, "open a new shell or run: source %s\n", rc)
				return nil
			},
		},
		&cobra.Command{
			Use:   "uninstall",
			Short: "Remove the managed block from your shell rc file",
			RunE: func(cmd *cobra.Command, args []string) error {
				a, err := newApp()
				if err != nil {
					return err
				}
				out := cmd.OutOrStdout()
				rc := a.Cfg.Shell.RCFile
				if rc == "" {
					rc = shellenv.DetectRC()
				}
				if rc != "" {
					changed, err := shellenv.Uninstall(rc)
					if err != nil {
						return err
					}
					if changed {
						fmt.Fprintf(out, "removed from %s\n", rc)
					} else {
						fmt.Fprintf(out, "nothing to remove in %s\n", rc)
					}
				}
				a.Cfg.Shell.Integration = false
				return a.Save()
			},
		},
	)
	return cmd
}

func newUninstallCmd() *cobra.Command {
	var (
		purge bool
		yes   bool
	)
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Stop the core and remove shell integration (--purge deletes all uclash data)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if err := a.Manager().Stop(cmd.Context()); err != nil {
				fmt.Fprintf(out, "core: %v\n", err)
			} else {
				fmt.Fprintln(out, "core: stopped")
			}
			rc := a.Cfg.Shell.RCFile
			if rc == "" {
				rc = shellenv.DetectRC()
			}
			if rc != "" {
				if changed, err := shellenv.Uninstall(rc); err == nil && changed {
					fmt.Fprintf(out, "shell: helpers removed from %s\n", rc)
				}
			}
			if !purge {
				fmt.Fprintln(out, "kept your data; delete manually if you want:")
				fmt.Fprintf(out, "  rm -rf %s\n", a.DataDir)
				fmt.Fprintf(out, "  rm -rf %s\n", a.ConfigPath)
				return nil
			}
			if !yes {
				return fmt.Errorf("--purge deletes %s and %s; pass --yes to confirm", a.DataDir, a.ConfigPath)
			}
			for _, dir := range []string{a.DataDir} {
				if err := os.RemoveAll(dir); err != nil {
					return err
				}
				fmt.Fprintf(out, "removed %s\n", dir)
			}
			if err := os.RemoveAll(configDirOf(a.ConfigPath)); err != nil {
				return err
			}
			fmt.Fprintf(out, "removed %s\n", configDirOf(a.ConfigPath))
			if exe, err := os.Executable(); err == nil {
				fmt.Fprintf(out, "the uclash binary itself can be removed with: rm -f %s\n", exe)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&purge, "purge", false, "delete data and config directories")
	cmd.Flags().BoolVar(&yes, "yes", false, "confirm --purge without prompting")
	return cmd
}

func configDirOf(p string) string {
	if i := strings.LastIndexAny(p, `/\`); i > 0 {
		return p[:i]
	}
	return p
}

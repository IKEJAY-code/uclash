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
		Short: "Optional shell integration (makes `uclash proxy on` a single command)",
		Long: `uclash never edits shell rc files unless you ask it to.

Without any setup, enabling the proxy in the current shell is:

  eval "$(uclash proxy on)"          # bash / zsh
  uclash proxy on --fish | source    # fish

Installing the wrapper adds one uclash() function to your rc file that
evaluates ` + "`uclash proxy on|off`" + ` for you, so you can simply run:

  uclash proxy on
`,
	}
	cmd.AddCommand(newShellInstallCmd(), newShellStatusCmd(), newShellUninstallCmd())
	return cmd
}

func newShellInstallCmd() *cobra.Command {
	var (
		shellFlag string
		rcFlag    string
	)
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Add the uclash() wrapper to your shell rc file",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			shell := shellFlag
			if shell == "" {
				shell = shellenv.DetectShell()
			}
			if shell == "" {
				shell = "bash"
				fmt.Fprintln(cmd.ErrOrStderr(), "uclash: could not detect your shell; defaulting to bash (use --shell bash|zsh|fish)")
			}
			rc := rcFlag
			source := "you specified it"
			if rc == "" {
				rc, source = shellenv.ResolveRC(shell)
			}
			changed, err := shellenv.Install(rc, shell)
			if err != nil {
				return err
			}
			a.Cfg.Shell.Integration = true
			a.Cfg.Shell.RCFile = rc
			if err := a.Save(); err != nil {
				return err
			}
			fmt.Fprintf(out, "shell:  %s\n", shell)
			fmt.Fprintf(out, "rc:     %s (%s)\n", rc, source)
			if changed {
				fmt.Fprintf(out, "installed the uclash wrapper into %s\n", rc)
			} else {
				fmt.Fprintf(out, "wrapper already present in %s\n", rc)
			}
			if shell == "bash" && rcFlag == "" {
				if risky := shellenv.BashLoginRisk(); risky != "" {
					fmt.Fprintf(cmd.ErrOrStderr(), "note: %s does not reference ~/.bashrc, so login shells may miss the wrapper.\n", risky)
					fmt.Fprintf(cmd.ErrOrStderr(), "      add `source ~/.bashrc` there, or run: uclash shell install --rc %s\n", risky)
				}
			}
			fmt.Fprintf(out, "open a new shell or run: source %s\n", rc)
			fmt.Fprintln(out, "then use:  uclash proxy on / uclash proxy off")
			return nil
		},
	}
	cmd.Flags().StringVar(&shellFlag, "shell", "", "shell to target: bash, zsh or fish (default: auto-detect)")
	cmd.Flags().StringVar(&rcFlag, "rc", "", "rc file to edit (default: bash=~/.bashrc, zsh=${ZDOTDIR:-~}/.zshrc, fish=~/.config/fish/config.fish)")
	return cmd
}

func newShellStatusCmd() *cobra.Command {
	var rcFlag string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show shell integration status",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			detected := shellenv.DetectShell()
			fmt.Fprintf(out, "detected shell: %s\n", orDash(detected))
			rc := rcFlag
			source := "you specified it"
			if rc == "" {
				rc = a.Cfg.Shell.RCFile
				source = "recorded at install time"
			}
			if rc == "" {
				shell := detected
				if shell == "" {
					shell = "bash"
				}
				rc, source = shellenv.ResolveRC(shell)
			}
			fmt.Fprintf(out, "rc file:        %s (%s)\n", orDash(rc), source)
			if rc != "" {
				fmt.Fprintf(out, "wrapper:        %v\n", shellenv.Installed(rc))
			}
			fmt.Fprintln(out)
			fmt.Fprintln(out, `without the wrapper:  eval "$(uclash proxy on)"`)
			fmt.Fprintln(out, "with the wrapper:     uclash proxy on")
			return nil
		},
	}
	cmd.Flags().StringVar(&rcFlag, "rc", "", "rc file to inspect")
	return cmd
}

func newShellUninstallCmd() *cobra.Command {
	var rcFlag string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the managed block from your shell rc file",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			rc := rcFlag
			if rc == "" {
				rc = a.Cfg.Shell.RCFile
			}
			if rc == "" {
				rc = shellenv.DetectRC()
			}
			if rc != "" {
				if _, err := os.Stat(rc); err == nil {
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
			} else {
				fmt.Fprintln(out, "no rc file to clean up")
			}
			a.Cfg.Shell.Integration = false
			return a.Save()
		},
	}
	cmd.Flags().StringVar(&rcFlag, "rc", "", "rc file to clean up")
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
					fmt.Fprintf(out, "shell: wrapper removed from %s\n", rc)
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

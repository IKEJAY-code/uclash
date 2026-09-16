package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/app"
	"gitee.com/IKEJAY-code/uclash/internal/shellenv"
	"gitee.com/IKEJAY-code/uclash/internal/uiurl"
	"github.com/spf13/cobra"
)

// newProxyCmd is the single, shell-agnostic entry point for the terminal
// proxy. The statements must be evaluated by the calling shell, which is why
// `uclash shell install` offers an optional wrapper that does it for you.
func newProxyCmd() *cobra.Command {
	var fish bool
	cmd := &cobra.Command{
		Use:   "proxy [on|off|status]",
		Short: "Enable/disable the proxy for the current shell (any shell)",
		Long: `Enable or disable the proxy for the current terminal.

  eval "$(uclash proxy on)"          # bash / zsh: start the core (if needed)
  eval "$(uclash proxy off)"         #   and set/unset http_proxy/... here
  uclash proxy on --fish | source    # fish
  uclash proxy status                # is this terminal already using it?

A child process cannot change its parent shell's environment, so the shell
must evaluate the output once. To make it a single command, install the
optional wrapper (it defines a uclash() function that does the eval for you):

  uclash shell install               # then: uclash proxy on

Only this terminal is affected; other terminals and other users are not.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			action := "status"
			if len(args) == 1 {
				action = strings.ToLower(args[0])
			}
			out := cmd.OutOrStdout()
			errOut := cmd.ErrOrStderr()

			switch action {
			case "on":
				// Start the core quietly so `eval "$(uclash proxy on)"` is a
				// complete "turn it on"; everything noisy goes to stderr.
				if _, running := a.Running(); !running {
					if err := startApp(cmd, a, true); err != nil {
						return err
					}
				}
				printProxyHint(errOut, a, true, fish)
				if fish {
					fmt.Fprint(out, shellenv.FishOn(a.Cfg.Ports.Mixed))
				} else {
					fmt.Fprint(out, shellenv.ExportOn(a.Cfg.Ports.Mixed))
				}
			case "off":
				printProxyHint(errOut, a, false, fish)
				if fish {
					fmt.Fprint(out, shellenv.FishOff(a.Cfg.Ports.Mixed))
				} else {
					fmt.Fprint(out, shellenv.ExportOff(a.Cfg.Ports.Mixed))
				}
			case "status":
				printProxyStatus(out, a)
			default:
				return fmt.Errorf("expected on|off|status, got %q", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&fish, "fish", false, "emit fish-compatible statements")
	return cmd
}

// printProxyHint explains how to apply the output, but only when it is meant
// for a human: if stdout is a pipe (the shell is running `eval "$(...)"` or
// `--fish | source`) the statements must stay byte-clean.
func printProxyHint(w io.Writer, a *app.App, on bool, fish bool) {
	if !uiurl.IsTerminal(os.Stdout) {
		return
	}
	action := "off"
	if on {
		action = "on"
	}
	if fish {
		fmt.Fprintf(w, "uclash: to apply this in the current shell:\n  uclash proxy %s --fish | source\n", action)
		return
	}
	fmt.Fprintf(w, "uclash: to apply this in the current shell:\n  eval \"$(uclash proxy %s)\"\n", action)
	if rc := a.Cfg.Shell.RCFile; rc != "" && shellenv.Installed(rc) {
		fmt.Fprintf(w, "        (the uclash wrapper is installed in %s, so `uclash proxy %s` also works directly)\n", rc, action)
	} else {
		fmt.Fprintln(w, "  or install the shell wrapper once:  uclash shell install")
	}
}

func printProxyStatus(out io.Writer, a *app.App) {
	port := a.Cfg.Ports.Mixed
	if pi, running := a.Running(); running {
		fmt.Fprintf(out, "core:   running (pid %d, mixed 127.0.0.1:%d)\n", pi.PID, port)
	} else {
		fmt.Fprintf(out, "core:   stopped (configured mixed port: %d)\n", port)
	}
	want := shellenv.ProxyURL(port)
	current := os.Getenv("http_proxy")
	if current == "" {
		current = os.Getenv("HTTP_PROXY")
	}
	switch {
	case current == "":
		fmt.Fprintln(out, "shell:  proxy OFF in this terminal")
	case current == want:
		fmt.Fprintf(out, "shell:  proxy ON in this terminal (http_proxy=%s)\n", current)
	default:
		fmt.Fprintf(out, "shell:  http_proxy=%s (not this uclash instance; ours would be %s)\n", current, want)
	}
	fmt.Fprintln(out)
	fmt.Fprintln(out, `enable:  eval "$(uclash proxy on)"        # bash / zsh`)
	fmt.Fprintln(out, `disable: eval "$(uclash proxy off)"`)
	fmt.Fprintln(out, "fish:    uclash proxy on --fish | source")
}

// newEnvCmd is the low-level, script-friendly printer kept for compatibility.
func newEnvCmd() *cobra.Command {
	var (
		sh   bool
		fish bool
	)
	cmd := &cobra.Command{
		Use:   "env on|off",
		Short: "Print shell statements for the terminal proxy (script-friendly)",
		Long: `Print the raw shell statements (same as ` + "`uclash proxy`" + `, without hints).

  eval "$(uclash env on)"    # export http_proxy/https_proxy/... for this shell
  eval "$(uclash env off)"   # remove them again`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch strings.ToLower(args[0]) {
			case "on":
				if _, running := a.Running(); !running {
					fmt.Fprintln(cmd.ErrOrStderr(), "uclash: core is not running; env points at a dead port (start with `uclash start`)")
				}
				switch {
				case fish:
					fmt.Fprint(out, shellenv.FishOn(a.Cfg.Ports.Mixed))
				case sh:
					fmt.Fprint(out, shify(shellenv.ExportOn(a.Cfg.Ports.Mixed), true))
				default:
					fmt.Fprint(out, shellenv.ExportOn(a.Cfg.Ports.Mixed))
				}
			case "off":
				switch {
				case fish:
					fmt.Fprint(out, shellenv.FishOff(a.Cfg.Ports.Mixed))
				case sh:
					fmt.Fprint(out, shify(shellenv.ExportOff(a.Cfg.Ports.Mixed), false))
				default:
					fmt.Fprint(out, shellenv.ExportOff(a.Cfg.Ports.Mixed))
				}
			default:
				return fmt.Errorf("expected 'on' or 'off', got %q", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&sh, "sh", false, "emit POSIX sh compatible statements (no `export` keyword on assignment)")
	cmd.Flags().BoolVar(&fish, "fish", false, "emit fish-compatible statements")
	return cmd
}

// shify rewrites "export K=V" into "K=V\nexport K" which works in dash/sh.
func shify(text string, on bool) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(text, "\n"), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			kv := strings.TrimPrefix(line, "export ")
			name := kv
			if i := strings.IndexByte(kv, '='); i > 0 {
				name = kv[:i]
			}
			fmt.Fprintf(&b, "%s\nexport %s\n", kv, name)
			continue
		}
		b.WriteString(line + "\n")
	}
	return b.String()
}

func newUICmd() *cobra.Command {
	var (
		open  bool
		plain bool
	)
	cmd := &cobra.Command{
		Use:   "ui",
		Short: "Print (or open) the dashboard URL with the control secret prefilled",
		Long: `Print the dashboard URL.

VS Code Remote terminals auto-forward localhost URLs, so Ctrl+Click on the
printed link opens the dashboard in your local browser.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			target := uiurl.Setup(a.Cfg.Ports.Controller, a.Cfg.Secret)
			if plain {
				target = uiurl.Serve(a.Cfg.Ports.Controller)
			}
			if uiurl.IsTerminal(os.Stdout) {
				fmt.Fprintln(out, uiurl.Hyperlink(target, target))
			} else {
				fmt.Fprintln(out, target)
			}
			if plain {
				fmt.Fprintf(out, "# secret: %s\n", a.Cfg.Secret)
			}
			if open {
				if err := openBrowser(target); err != nil {
					return err
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&open, "open", false, "also try to open it in a browser")
	cmd.Flags().BoolVar(&plain, "plain", false, "print the URL without the prefilled secret")
	return cmd
}

func openBrowser(target string) error {
	var candidates [][]string
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates, []string{"open", target})
	case "windows":
		candidates = append(candidates, []string{"cmd", "/c", "start", "", target})
	default:
		if b := os.Getenv("BROWSER"); b != "" {
			candidates = append(candidates, []string{b, target})
		}
		candidates = append(candidates,
			[]string{"xdg-open", target},
			[]string{"sensible-browser", target},
		)
	}
	var lastErr error
	for _, c := range candidates {
		if _, err := exec.LookPath(c[0]); err != nil {
			lastErr = err
			continue
		}
		goCmd := exec.Command(c[0], c[1:]...)
		if err := goCmd.Start(); err != nil {
			lastErr = err
			continue
		}
		go func() { _ = goCmd.Wait() }()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no browser opener found")
	}
	return fmt.Errorf("open browser: %w", lastErr)
}

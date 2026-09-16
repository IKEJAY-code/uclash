package cli

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/shellenv"
	"gitee.com/IKEJAY-code/uclash/internal/uiurl"
	"github.com/spf13/cobra"
)

func newEnvCmd() *cobra.Command {
	var sh bool
	cmd := &cobra.Command{
		Use:   "env on|off",
		Short: "Print shell statements to enable/disable the terminal proxy",
		Long: `Print shell statements for the current terminal.

  eval "$(uclash env on)"    # export http_proxy/https_proxy/... for this shell
  eval "$(uclash env off)"   # remove them again

These only affect the shell you run them in; other users (and your other
terminals) are untouched.`,
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
				text := shellenv.ExportOn(a.Cfg.Ports.Mixed)
				if sh {
					text = shify(text, true)
				}
				fmt.Fprint(out, text)
			case "off":
				text := shellenv.ExportOff(a.Cfg.Ports.Mixed)
				if sh {
					text = shify(text, false)
				}
				fmt.Fprint(out, text)
			default:
				return fmt.Errorf("expected 'on' or 'off', got %q", args[0])
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&sh, "sh", false, "emit POSIX sh compatible statements (no `export` keyword on assignment)")
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
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		if err := cmd.Start(); err != nil {
			lastErr = err
			continue
		}
		go func() { _ = cmd.Wait() }()
		return nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no browser opener found")
	}
	return fmt.Errorf("open browser: %w", lastErr)
}

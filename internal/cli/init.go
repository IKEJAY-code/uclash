package cli

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/app"
	"gitee.com/IKEJAY-code/uclash/internal/core"
	"gitee.com/IKEJAY-code/uclash/internal/fsx"
	"gitee.com/IKEJAY-code/uclash/internal/shellenv"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"gitee.com/IKEJAY-code/uclash/internal/uiurl"
	"github.com/spf13/cobra"
)

func newInitCmd() *cobra.Command {
	var (
		subURL           string
		name             string
		mirror           string
		corePath         string
		uiSource         string
		force            bool
		noShell          bool
		shellIntegration bool
	)
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Set up uclash: fetch the mihomo core + dashboard, pick ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			out := cmd.OutOrStdout()
			a, err := newApp()
			if err != nil {
				return err
			}
			if err := a.EnsureDirs(); err != nil {
				return err
			}
			if mirror != "" {
				a.Cfg.Mirror = mirror
			}
			if a.Cfg.Secret == "" {
				a.Cfg.Secret = state.NewSecret()
			}
			if _, err := a.EnsurePorts(); err != nil {
				return err
			}

			switch {
			case corePath != "":
				if err := fsx.CopyFile(corePath, a.CorePath()); err != nil {
					return err
				}
				fmt.Fprintf(out, "core:   installed from %s\n", corePath)
			case force || !fsx.Exists(a.CorePath()):
				fmt.Fprintln(out, "core:   downloading mihomo ...")
				tag, err := core.FetchCore(ctx, a.Cfg.Core.Version, a.DownloadOptions(true), a.CorePath())
				if err != nil {
					return err
				}
				a.Cfg.Core.Version = tag
				fmt.Fprintf(out, "core:   mihomo %s installed\n", tag)
			default:
				fmt.Fprintf(out, "core:   already present at %s\n", a.CorePath())
			}

			switch {
			case uiSource == "skip":
				fmt.Fprintln(out, "ui:     skipped")
			case uiSource != "":
				if !fsx.IsDir(uiSource) {
					return fmt.Errorf("--ui expects a directory (or 'skip'), got %q", uiSource)
				}
				tmp := a.UIDir() + ".tmp"
				_ = os.RemoveAll(tmp)
				if err := fsx.CopyDir(uiSource, tmp); err != nil {
					return err
				}
				if err := fsx.ReplaceDir(tmp, a.UIDir()); err != nil {
					return err
				}
				fmt.Fprintf(out, "ui:     installed from %s\n", uiSource)
			case force || !fsx.Exists(filepath.Join(a.UIDir(), "index.html")):
				fmt.Fprintln(out, "ui:     downloading metacubexd dashboard ...")
				if err := core.FetchUI(ctx, a.DownloadOptions(true), a.UIDir()); err != nil {
					return err
				}
				fmt.Fprintln(out, "ui:     metacubexd installed")
			default:
				fmt.Fprintf(out, "ui:     already present at %s\n", a.UIDir())
			}

			if subURL != "" {
				if err := addSubscription(ctx, a, out, subURL, name); err != nil {
					return err
				}
			}
			if err := a.GenerateConfig(); err != nil && !isNoProfile(err) {
				return err
			}

			if shellIntegration && !noShell {
				shellName := shellenv.DetectShell()
				if shellName == "" {
					shellName = "bash"
				}
				rc, source := shellenv.ResolveRC(shellName)
				changed, err := shellenv.Install(rc, shellName)
				if err != nil {
					fmt.Fprintf(out, "shell:  could not update %s: %v\n", rc, err)
				} else {
					a.Cfg.Shell.Integration = true
					a.Cfg.Shell.RCFile = rc
					fmt.Fprintf(out, "shell:  %s wrapper -> %s (%s)\n", shellName, rc, source)
					if changed {
						fmt.Fprintln(out, "        installed; open a new shell (or source it) to use `uclash proxy on`")
					} else {
						fmt.Fprintln(out, "        already present")
					}
				}
			} else if !noShell {
				fmt.Fprintln(out, "shell:  rc files left untouched (as of v0.2).")
				fmt.Fprintln(out, "        - any shell, no setup:  eval \"$(uclash proxy on)\"")
				fmt.Fprintln(out, "        - one command instead:  uclash shell install   (then: uclash proxy on)")
			}
			if err := a.Save(); err != nil {
				return err
			}

			fmt.Fprintln(out)
			fmt.Fprintf(out, "config: %s\n", a.ConfigPath)
			fmt.Fprintf(out, "data:   %s\n", a.DataDir)
			fmt.Fprintf(out, "ports:  mixed %d | controller %d\n", a.Cfg.Ports.Mixed, a.Cfg.Ports.Controller)
			reg, _ := a.Registry()
			switch {
			case reg.Active != "":
				fmt.Fprintf(out, "profile: %s\n", reg.Active)
				fmt.Fprintln(out)
				fmt.Fprintln(out, "start it with:  uclash start")
				if a.Cfg.Shell.Integration {
					fmt.Fprintln(out, "shell proxy:    uclash proxy on      (wrapper installed, new shells)")
				} else {
					fmt.Fprintln(out, "shell proxy:    eval \"$(uclash proxy on)\"   (any shell)")
				}
			default:
				fmt.Fprintln(out)
				fmt.Fprintln(out, "next: add a subscription  ->  uclash sub add <clash-subscription-url>")
				fmt.Fprintln(out, "      or import a YAML   ->  uclash profile import <config.yaml>")
			}
			fmt.Fprintf(out, "dashboard: %s\n", uiurl.Setup(a.Cfg.Ports.Controller, a.Cfg.Secret))
			return nil
		},
	}
	cmd.Flags().StringVar(&subURL, "sub", "", "subscription URL to add right away")
	cmd.Flags().StringVar(&name, "name", "", "profile name for --sub")
	cmd.Flags().StringVar(&mirror, "mirror", "", "GitHub mirror prefix for downloads (e.g. https://gh-proxy.com)")
	cmd.Flags().StringVar(&corePath, "core", "", "install the core from a local mihomo binary instead of downloading")
	cmd.Flags().StringVar(&uiSource, "ui", "", "install the dashboard from a local directory, or 'skip'")
	cmd.Flags().BoolVar(&force, "force", false, "re-download core and dashboard even if present")
	cmd.Flags().BoolVar(&shellIntegration, "shell-integration", false, "install the uclash shell wrapper (makes `uclash proxy on` a single command)")
	cmd.Flags().BoolVar(&noShell, "no-shell", false, "do not print or install shell integration")
	return cmd
}

func isNoProfile(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no active profile")
}

func addSubscription(ctx context.Context, a *app.App, out io.Writer, rawURL, name string) error {
	rawURL, err := normalizeSubURL(rawURL, out)
	if err != nil {
		return err
	}
	sub, err := a.FetchSubscription(ctx, rawURL)
	if err != nil {
		return err
	}
	if name == "" {
		name = suggestName(sub.URL)
	}
	if !app.ValidProfileName(name) {
		return fmt.Errorf("invalid profile name %q (letters, digits, dot, dash, underscore)", name)
	}
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	if reg.Find(name) != nil {
		return fmt.Errorf("profile %q already exists (pick another --name or remove it with `uclash sub rm %s`)", name, name)
	}
	if err := a.WriteProfile(name, sub.Body); err != nil {
		return err
	}
	prevActive := reg.Active
	reg.Profiles = append(reg.Profiles, state.Profile{
		Name:      name,
		URL:       sub.URL,
		Origin:    rawURL,
		File:      name + ".yaml",
		Source:    "sub",
		Converted: sub.Converted,
		UpdatedAt: state.Now(),
	})
	if reg.Active == "" {
		reg.Active = name
	}
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}
	note := activeNote(reg, name)
	if sub.Converted {
		note += " [converted from node links]"
	}
	fmt.Fprintf(out, "profile: %s added%s\n", name, note)
	if err := a.ApplyReload(ctx); err != nil {
		if prevActive != reg.Active {
			reg.Active = prevActive
			_ = a.SaveRegistry(reg)
			_ = a.GenerateConfig()
		}
		return fmt.Errorf("profile %q was saved but mihomo rejected it: %w", name, err)
	}
	return nil
}

func activeNote(reg *state.Registry, name string) string {
	if reg.Active == name {
		return " (active)"
	}
	return ""
}

// normalizeSubURL tolerates the shell-escaped URLs people commonly copy
// (`...\?token\=xx`), and points local files at `uclash profile import`.
func normalizeSubURL(raw string, out io.Writer) (string, error) {
	if strings.Contains(raw, "://") && strings.Contains(raw, `\`) {
		clean := strings.ReplaceAll(raw, `\`, "")
		if clean != raw {
			fmt.Fprintln(out, "note: removed backslashes from the URL (shell escaping is not part of a URL)")
			raw = clean
		}
	}
	if !strings.Contains(raw, "://") {
		if _, err := os.Stat(raw); err == nil {
			return "", fmt.Errorf("%q is a local file; import it with:\n  uclash profile import %s", raw, raw)
		}
		return "", fmt.Errorf("%q is not a subscription URL (it must start with http:// or https://)", raw)
	}
	return raw, nil
}

func suggestName(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "sub"
	}
	host := strings.Split(u.Hostname(), ".")[0]
	host = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, host)
	host = strings.Trim(host, "-_")
	if host == "" {
		host = "sub"
	}
	if len(host) > 32 {
		host = host[:32]
	}
	return strings.ToLower(host)
}

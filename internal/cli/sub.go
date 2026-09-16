package cli

import (
	"fmt"
	"io"
	"os"
	"strconv"
	"text/tabwriter"

	"gitee.com/IKEJAY-code/uclash/internal/app"
	"gitee.com/IKEJAY-code/uclash/internal/download"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"github.com/spf13/cobra"
)

func newSubCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Manage subscription profiles",
	}
	cmd.AddCommand(
		newSubAddCmd(),
		newSubLsCmd(),
		newSubUseCmd(),
		newSubUpdateCmd(),
		newSubRmCmd(),
		newSubAutoCmd(),
	)
	return cmd
}

func newSubAddCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "add <subscription-url>",
		Short: "Add a Clash/Mihomo subscription and make it available",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			if err := a.EnsureDirs(); err != nil {
				return err
			}
			if a.Cfg.Secret == "" {
				a.Cfg.Secret = state.NewSecret()
			}
			if _, err := a.EnsurePorts(); err != nil {
				return err
			}
			if err := a.Save(); err != nil {
				return err
			}
			return addSubscription(cmd.Context(), a, cmd.OutOrStdout(), args[0], name)
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "profile name (default: derived from the URL host)")
	return cmd
}

func newSubLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			return listProfiles(a, cmd.OutOrStdout())
		},
	}
}

func newSubUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name>",
		Short: "Switch the active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			if err := a.SetActive(cmd.Context(), args[0]); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "profile: %s is now active\n", args[0])
			if _, running := a.Running(); running {
				fmt.Fprintln(cmd.OutOrStdout(), "core:    configuration hot-reloaded")
			}
			return nil
		},
	}
}

func newSubUpdateCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "update [name]",
		Short: "Re-download subscription profile(s)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			reg, err := a.Registry()
			if err != nil {
				return err
			}
			var targets []*state.Profile
			switch {
			case all:
				for i := range reg.Profiles {
					if reg.Profiles[i].Source == "sub" && reg.Profiles[i].URL != "" {
						targets = append(targets, &reg.Profiles[i])
					}
				}
			case len(args) == 1:
				p := reg.Find(args[0])
				if p == nil {
					return fmt.Errorf("profile %q not found (see `uclash sub ls`)", args[0])
				}
				if p.Source != "sub" || p.URL == "" {
					return fmt.Errorf("profile %q was imported from a local file; nothing to download", args[0])
				}
				targets = append(targets, p)
			default:
				if reg.Active == "" {
					return fmt.Errorf("no active profile; pass a name or --all")
				}
				p := reg.Find(reg.Active)
				if p == nil || p.Source != "sub" || p.URL == "" {
					return fmt.Errorf("active profile %q is not a subscription", reg.Active)
				}
				targets = append(targets, p)
			}
			if len(targets) == 0 {
				return fmt.Errorf("no subscription profiles to update")
			}
			type undo struct {
				name string
				body []byte
			}
			var backups []undo
			activeTouched := false
			for _, p := range targets {
				sub, err := a.FetchSubscription(cmd.Context(), p.URL)
				if err != nil {
					fmt.Fprintf(out, "fail: %s: %v\n", p.Name, err)
					continue
				}
				if old, rerr := os.ReadFile(a.ProfilePath(p.Name)); rerr == nil {
					backups = append(backups, undo{p.Name, old})
				}
				if err := a.WriteProfile(p.Name, sub.Body); err != nil {
					return err
				}
				p.URL = sub.URL
				p.Converted = sub.Converted
				p.UpdatedAt = state.Now()
				if reg.Active == p.Name {
					activeTouched = true
				}
				fmt.Fprintf(out, "ok:   %s updated\n", p.Name)
			}
			if err := a.SaveRegistry(reg); err != nil {
				return err
			}
			if activeTouched {
				if err := a.ApplyReload(cmd.Context()); err != nil {
					for _, u := range backups {
						_ = download.WriteFileAtomic(a.ProfilePath(u.name), u.body, 0o600)
					}
					return fmt.Errorf("%w (subscription rolled back to the previous version)", err)
				}
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "update every subscription profile")
	return cmd
}

func newSubRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			return removeProfile(a, cmd.OutOrStdout(), args[0])
		},
	}
}

func newSubAutoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "auto [hours]",
		Short: "Show or set the subscription auto-update interval (hours)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(args) == 0 {
				fmt.Fprintf(out, "%d\n", a.Cfg.AutoUpdateHours)
				return nil
			}
			h, err := strconv.Atoi(args[0])
			if err != nil || h < 0 || h > 24*30 {
				return fmt.Errorf("invalid hours %q (0 disables auto-update)", args[0])
			}
			if h == 0 {
				h = 24 * 3650
			}
			a.Cfg.AutoUpdateHours = h
			if err := a.Save(); err != nil {
				return err
			}
			fmt.Fprintf(out, "auto-update: every %s (checked on `uclash start`)\n", humanHours(h))
			return nil
		},
	}
}

func humanHours(h int) string {
	if h%24 == 0 && h >= 24 {
		return fmt.Sprintf("%dd", h/24)
	}
	return fmt.Sprintf("%dh", h)
}

func listProfiles(a *app.App, out io.Writer) error {
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	if len(reg.Profiles) == 0 {
		fmt.Fprintln(out, "no profiles yet; add one with `uclash sub add <url>`")
		return nil
	}
	w := tabwriter.NewWriter(out, 2, 4, 2, ' ', 0)
	fmt.Fprintln(w, "  NAME\tSOURCE\tUPDATED\tLOCATION")
	for _, p := range reg.Profiles {
		marker := " "
		if p.Name == reg.Active {
			marker = "*"
		}
		loc := p.URL
		if loc == "" {
			loc = p.Origin
		}
		if loc == "" {
			loc = p.File
		}
		src := p.Source
		if p.Converted {
			src = "sub(conv)"
		}
		fmt.Fprintf(w, "%s %s\t%s\t%s\t%s\n", marker, p.Name, src, app.HumanSince(p.UpdatedAt), loc)
	}
	return w.Flush()
}

func removeProfile(a *app.App, out io.Writer, name string) error {
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	p := reg.Find(name)
	if p == nil {
		return fmt.Errorf("profile %q not found (see `uclash sub ls`)", name)
	}
	wasActive := reg.Active == name
	if !reg.Remove(name) {
		return fmt.Errorf("profile %q not found", name)
	}
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}
	if err := os.Remove(a.ProfilePath(name)); err != nil && !os.IsNotExist(err) {
		return err
	}
	fmt.Fprintf(out, "profile: %s removed\n", name)
	if wasActive {
		fmt.Fprintln(out, "note: it was the active profile; the running core keeps its last configuration until you switch profiles or stop it")
	}
	return nil
}

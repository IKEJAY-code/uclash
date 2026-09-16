package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/app"
	"gitee.com/IKEJAY-code/uclash/internal/core"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"gitee.com/IKEJAY-code/uclash/internal/uiurl"
	"github.com/spf13/cobra"
)

func newStartCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the mihomo core in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			return startApp(cmd, a, quiet)
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress output")
	return cmd
}

func newStopCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop this user's mihomo core (never touches other users)",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			err = a.Manager().Stop(cmd.Context())
			switch {
			case errors.Is(err, core.ErrNotRunning):
				if !quiet {
					fmt.Fprintln(out, "not running")
				}
				return nil
			case err != nil:
				return err
			}
			if !quiet {
				fmt.Fprintln(out, "stopped")
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress output")
	return cmd
}

func newRestartCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the mihomo core",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if err := a.Manager().Stop(cmd.Context()); err != nil && !errors.Is(err, core.ErrNotRunning) {
				return err
			}
			if !quiet {
				fmt.Fprintln(out, "restarting ...")
			}
			return startApp(cmd, a, quiet)
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "suppress output")
	return cmd
}

func newStatusCmd() *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show core status, ports and dashboard URL",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			pi, running := a.Running()
			if quiet {
				if running {
					fmt.Fprintln(out, "running")
				} else {
					fmt.Fprintln(out, "stopped")
				}
				return nil
			}
			if !running {
				fmt.Fprintln(out, "status:  stopped")
				fmt.Fprintf(out, "ports:   mixed %d | controller %d\n", a.Cfg.Ports.Mixed, a.Cfg.Ports.Controller)
				reg, _ := a.Registry()
				if reg != nil && reg.Active != "" {
					fmt.Fprintf(out, "profile: %s\n", reg.Active)
				}
				fmt.Fprintf(out, "log:     %s\n", a.LogFile())
				return nil
			}
			fmt.Fprintf(out, "status:  running (pid %d, up %s)\n", pi.PID, time.Since(pi.StartedAt).Truncate(time.Second))
			if client, ok := a.Client(); ok {
				if v, err := client.Version(cmd.Context()); err == nil {
					fmt.Fprintf(out, "core:    mihomo %s\n", v)
				}
				if cfg, err := client.Configs(cmd.Context()); err == nil {
					fmt.Fprintf(out, "mode:    %s\n", cfg.Mode)
					fmt.Fprintf(out, "ports:   mixed 127.0.0.1:%d | controller 127.0.0.1:%d\n", cfg.MixedPort, a.Cfg.Ports.Controller)
					fmt.Fprintf(out, "lan:     %v\n", cfg.AllowLan)
				}
			}
			fmt.Fprintf(out, "ui:      %s\n", uiurl.Setup(a.Cfg.Ports.Controller, a.Cfg.Secret))
			reg, _ := a.Registry()
			if reg != nil && reg.Active != "" {
				p := reg.Find(reg.Active)
				if p != nil {
					fmt.Fprintf(out, "profile: %s (updated %s)\n", p.Name, app.HumanSince(p.UpdatedAt))
				}
			}
			fmt.Fprintf(out, "log:     %s\n", a.LogFile())
			return nil
		},
	}
	cmd.Flags().BoolVar(&quiet, "quiet", false, "print only running/stopped")
	return cmd
}

func startApp(cmd *cobra.Command, a *app.App, quiet bool) error {
	ctx := cmd.Context()
	out := cmd.OutOrStdout()
	if _, running := a.Running(); running {
		if !quiet {
			pi, _ := a.Running()
			fmt.Fprintf(out, "already running (pid %d)\n", pi.PID)
		}
		return nil
	}
	if err := a.EnsureDirs(); err != nil {
		return err
	}
	if err := a.Cfg.Validate(); err != nil {
		return err
	}
	if a.Cfg.Secret == "" {
		a.Cfg.Secret = state.NewSecret()
	}
	changed, err := a.EnsurePorts()
	if err != nil {
		return err
	}
	if err := a.Save(); err != nil {
		return err
	}
	if changed && !quiet {
		fmt.Fprintf(out, "ports:  using mixed %d | controller %d\n", a.Cfg.Ports.Mixed, a.Cfg.Ports.Controller)
	}
	if err := maybeAutoUpdate(ctx, a, out, quiet); err != nil && !quiet {
		fmt.Fprintf(out, "warn:   subscription auto-update failed: %v\n", err)
	}
	if err := a.GenerateConfig(); err != nil {
		return err
	}
	pi, err := a.Manager().Start(ctx)
	if err != nil {
		if errors.Is(err, core.ErrAlreadyRunning) {
			if !quiet {
				fmt.Fprintf(out, "already running (pid %d)\n", pi.PID)
			}
			return nil
		}
		return err
	}
	if !quiet {
		version := ""
		if client, ok := a.Client(); ok {
			if v, err := client.Version(ctx); err == nil {
				version = " (" + v + ")"
			}
		}
		fmt.Fprintf(out, "started (pid %d)%s\n", pi.PID, version)
		fmt.Fprintf(out, "ports:  mixed 127.0.0.1:%d | controller 127.0.0.1:%d\n", a.Cfg.Ports.Mixed, a.Cfg.Ports.Controller)
		fmt.Fprintf(out, "ui:     %s\n", uiurl.Setup(a.Cfg.Ports.Controller, a.Cfg.Secret))
		fmt.Fprintln(out, "proxy:  eval \"$(uclash env on)\"  or use the proxyon shell helper")
	}
	return nil
}

func maybeAutoUpdate(ctx context.Context, a *app.App, out io.Writer, quiet bool) error {
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	if reg.Active == "" {
		return nil
	}
	p := reg.Find(reg.Active)
	if p == nil || p.Source != "sub" || p.URL == "" {
		return nil
	}
	if !state.Stale(p.UpdatedAt, a.Cfg.AutoUpdateHours) {
		return nil
	}
	sub, err := a.FetchSubscription(ctx, p.URL)
	if err != nil {
		return err
	}
	if err := a.WriteProfile(p.Name, sub.Body); err != nil {
		return err
	}
	p.URL = sub.URL
	p.Converted = sub.Converted
	p.UpdatedAt = state.Now()
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}
	if !quiet {
		fmt.Fprintf(out, "sub:    refreshed %s\n", p.Name)
	}
	return nil
}

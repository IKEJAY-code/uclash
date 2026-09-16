package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/port"
	"github.com/spf13/cobra"
)

func newModeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mode [rule|global|direct]",
		Short: "Show or change the routing mode",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			if len(args) == 0 {
				mode := a.Cfg.Mode
				if client, ok := a.Client(); ok {
					if cfg, err := client.Configs(cmd.Context()); err == nil {
						mode = cfg.Mode
					}
				}
				fmt.Fprintln(out, mode)
				return nil
			}
			next := strings.ToLower(args[0])
			switch next {
			case "rule", "global", "direct":
			default:
				return fmt.Errorf("invalid mode %q (want rule|global|direct)", args[0])
			}
			a.Cfg.Mode = next
			if err := a.Save(); err != nil {
				return err
			}
			if client, ok := a.Client(); ok {
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := client.PatchConfigs(ctx, map[string]any{"mode": next}); err != nil {
					return fmt.Errorf("saved, but switching the running core failed: %w", err)
				}
			}
			fmt.Fprintf(out, "mode: %s\n", next)
			return nil
		},
	}
}

func newPortCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "port",
		Short: "Show or change proxy/controller ports",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "mixed:      %d\n", a.Cfg.Ports.Mixed)
			fmt.Fprintf(out, "controller: %d\n", a.Cfg.Ports.Controller)
			if client, ok := a.Client(); ok {
				if cfg, err := client.Configs(cmd.Context()); err == nil {
					fmt.Fprintf(out, "live mixed: %d\n", cfg.MixedPort)
				}
			}
			return nil
		},
	}
	set := &cobra.Command{
		Use:   "set mixed|controller <port>",
		Short: "Change a port (mixed applies live; controller restarts the core)",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			var p int
			if _, err := fmt.Sscanf(args[1], "%d", &p); err != nil || p <= 0 || p > 65535 {
				return fmt.Errorf("invalid port %q", args[1])
			}
			which := strings.ToLower(args[0])
			current := a.Cfg.Ports.Mixed
			if which == "controller" {
				current = a.Cfg.Ports.Controller
			} else if which != "mixed" {
				return fmt.Errorf("expected 'mixed' or 'controller', got %q", args[0])
			}
			if p == current {
				fmt.Fprintf(out, "%s port already %d\n", which, p)
				return nil
			}
			if !port.IsFree("127.0.0.1", p) {
				return fmt.Errorf("port %d is not free on 127.0.0.1", p)
			}
			switch which {
			case "mixed":
				a.Cfg.Ports.Mixed = p
				if err := a.Save(); err != nil {
					return err
				}
				if err := a.GenerateConfig(); err != nil && !isNoProfile(err) {
					return err
				}
				if client, ok := a.Client(); ok {
					ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
					defer cancel()
					if err := client.PatchConfigs(ctx, map[string]any{"mixed-port": p}); err != nil {
						return fmt.Errorf("saved, but changing the live port failed: %w", err)
					}
				}
			case "controller":
				a.Cfg.Ports.Controller = p
				if err := a.Save(); err != nil {
					return err
				}
				if err := a.GenerateConfig(); err != nil && !isNoProfile(err) {
					return err
				}
				if _, running := a.Running(); running {
					if err := a.Manager().Stop(cmd.Context()); err != nil {
						return err
					}
					if err := startApp(cmd, a, true); err != nil {
						return err
					}
					fmt.Fprintln(out, "core restarted with the new controller port")
				}
			}
			fmt.Fprintf(out, "%s port -> %d\n", which, p)
			return nil
		},
	}
	cmd.AddCommand(set)
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List ports (same as `uclash port`)",
		RunE:  cmd.RunE,
	})
	return cmd
}

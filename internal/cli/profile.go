package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/app"
	"gitee.com/IKEJAY-code/uclash/internal/download"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"gitee.com/IKEJAY-code/uclash/internal/submerge"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage local YAML profiles",
	}
	cmd.AddCommand(
		newProfileImportCmd(),
		newProfileLsCmd(),
		newProfileUseCmd(),
		newProfileRmCmd(),
	)
	return cmd
}

func newProfileImportCmd() *cobra.Command {
	var name string
	cmd := &cobra.Command{
		Use:   "import <config.yaml>",
		Short: "Import a local Clash/Mihomo YAML as a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			if err := a.EnsureDirs(); err != nil {
				return err
			}
			src := args[0]
			body, err := os.ReadFile(src)
			if err != nil {
				return err
			}
			if err := submerge.Validate(body); err != nil {
				return fmt.Errorf("%s: %w", src, err)
			}
			if name == "" {
				base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
				name = suggestName(base)
			}
			if !app.ValidProfileName(name) {
				return fmt.Errorf("invalid profile name %q (letters, digits, dot, dash, underscore)", name)
			}
			reg, err := a.Registry()
			if err != nil {
				return err
			}
			if reg.Find(name) != nil {
				return fmt.Errorf("profile %q already exists (pick another --name)", name)
			}
			abs, _ := filepath.Abs(src)
			if err := download.WriteFileAtomic(a.ProfilePath(name), body, 0o600); err != nil {
				return err
			}
			reg.Profiles = append(reg.Profiles, state.Profile{
				Name:   name,
				Origin: abs,
				File:   name + ".yaml",
				Source: "file",
			})
			if reg.Active == "" {
				reg.Active = name
			}
			if err := a.SaveRegistry(reg); err != nil {
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
			fmt.Fprintf(cmd.OutOrStdout(), "profile: %s imported from %s%s\n", name, abs, activeNote(reg, name))
			if err := a.ApplyReload(cmd.Context()); err != nil {
				return err
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "profile name (default: file name)")
	return cmd
}

func newProfileLsCmd() *cobra.Command {
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

func newProfileUseCmd() *cobra.Command {
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
			return nil
		},
	}
}

func newProfileRmCmd() *cobra.Command {
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

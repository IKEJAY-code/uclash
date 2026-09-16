package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/fsx"
	"gitee.com/IKEJAY-code/uclash/internal/port"
	"gitee.com/IKEJAY-code/uclash/internal/shellenv"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"github.com/spf13/cobra"
)

type docCheck struct {
	status string
	name   string
	detail string
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check the installation for common problems",
		RunE: func(cmd *cobra.Command, args []string) error {
			a, err := newApp()
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			var checks []docCheck
			add := func(status, name, format string, a ...any) {
				checks = append(checks, docCheck{status, name, fmt.Sprintf(format, a...)})
			}

			if runtime.GOOS == "linux" {
				add("ok", "platform", "%s/%s", runtime.GOOS, runtime.GOARCH)
			} else {
				add("warn", "platform", "%s/%s (core management is Linux-only in this build)", runtime.GOOS, runtime.GOARCH)
			}

			if err := a.EnsureDirs(); err != nil {
				add("fail", "data dir", "%s: %v", a.DataDir, err)
			} else {
				probe := filepath.Join(a.DataDir, ".doctor-write-test")
				if err := os.WriteFile(probe, []byte("ok"), 0o600); err != nil {
					add("fail", "data dir", "%s not writable: %v", a.DataDir, err)
				} else {
					_ = os.Remove(probe)
					add("ok", "data dir", "%s", a.DataDir)
				}
			}

			if fsx.Exists(a.ConfigPath) {
				add("ok", "config", "%s", a.ConfigPath)
			} else {
				add("warn", "config", "%s missing (run `uclash init`)", a.ConfigPath)
			}

			if fsx.Exists(a.CorePath()) {
				add("ok", "core", "%s", a.CorePath())
			} else {
				add("fail", "core", "missing at %s (run `uclash init`, or `uclash init --core <path>`)", a.CorePath())
			}

			if fsx.Exists(filepath.Join(a.UIDir(), "index.html")) {
				add("ok", "dashboard", "%s", a.UIDir())
			} else {
				add("warn", "dashboard", "missing at %s (the REST API still works; run `uclash init` to fetch it)", a.UIDir())
			}

			pi, running := a.Running()
			if running {
				add("ok", "core state", "running (pid %d, up %s)", pi.PID, time.Since(pi.StartedAt).Truncate(time.Second))
				if client, ok := a.Client(); ok {
					if v, err := client.Version(cmd.Context()); err == nil {
						add("ok", "core api", "mihomo %s on 127.0.0.1:%d", v, a.Cfg.Ports.Controller)
					} else {
						add("warn", "core api", "running but API unreachable: %v", err)
					}
				}
			} else {
				add("ok", "core state", "stopped")
				for name, p := range map[string]int{"mixed": a.Cfg.Ports.Mixed, "controller": a.Cfg.Ports.Controller} {
					if p <= 0 {
						add("warn", "port "+name, "not assigned yet (run `uclash init`)")
					} else if port.IsFree("127.0.0.1", p) {
						add("ok", "port "+name, "%d is free", p)
					} else {
						add("warn", "port "+name, "%d is busy (a new free port is picked automatically on start)", p)
					}
				}
			}

			reg, _ := a.Registry()
			if reg == nil || reg.Active == "" {
				add("warn", "profile", "no active profile (run `uclash sub add <url>`)")
			} else if p := reg.Find(reg.Active); p != nil {
				if p.Source == "sub" && state.Stale(p.UpdatedAt, a.Cfg.AutoUpdateHours) {
					add("warn", "profile", "%s last updated %s (auto-update on next start)", p.Name, humanSince(p.UpdatedAt))
				} else {
					add("ok", "profile", "%s (%s)", p.Name, p.Source)
				}
			}

			rc := a.Cfg.Shell.RCFile
			if rc == "" {
				rc = shellenv.DetectRC()
			}
			if rc != "" && shellenv.Installed(rc) {
				add("ok", "shell", "uclash wrapper installed in %s", rc)
			} else {
				add("warn", "shell", "helpers not installed (run `uclash shell install`)")
			}

			if exe, err := os.Executable(); err == nil {
				dir := filepath.Dir(exe)
				if pathHas(os.Getenv("PATH"), dir) {
					add("ok", "PATH", "%s", dir)
				} else {
					add("warn", "PATH", "%s is not in PATH", dir)
				}
			}

			fails := 0
			for _, c := range checks {
				fmt.Fprintf(out, "[%-4s] %-12s %s\n", c.status, c.name, c.detail)
				if c.status == "fail" {
					fails++
				}
			}
			if fails > 0 {
				return fmt.Errorf("%d problem(s) found", fails)
			}
			return nil
		},
	}
}

func pathHas(pathEnv, dir string) bool {
	for _, p := range strings.Split(pathEnv, string(os.PathListSeparator)) {
		if p != "" && strings.EqualFold(filepath.Clean(p), filepath.Clean(dir)) {
			return true
		}
	}
	return false
}

func humanSince(ts string) string {
	if ts == "" {
		return "never"
	}
	if t, err := time.Parse(time.RFC3339, ts); err == nil {
		return time.Since(t).Truncate(time.Minute).String() + " ago"
	}
	return ts
}

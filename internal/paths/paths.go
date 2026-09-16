package paths

import (
	"os"
	"path/filepath"
	"strings"
)

func home() string {
	h, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	return h
}

// ConfigDir returns the directory holding the user configuration file.
// Overridable with UCLASH_CONFIG_DIR (useful for tests and multi-instance use).
func ConfigDir() string {
	if v := strings.TrimSpace(os.Getenv("UCLASH_CONFIG_DIR")); v != "" {
		return ExpandHome(v)
	}
	if v := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); v != "" {
		return filepath.Join(ExpandHome(v), "uclash")
	}
	return filepath.Join(home(), ".config", "uclash")
}

// DataDir returns the directory holding runtime data (core binary, profiles,
// generated config, logs, pid files).
// Overridable with UCLASH_HOME.
func DataDir() string {
	if v := strings.TrimSpace(os.Getenv("UCLASH_HOME")); v != "" {
		return ExpandHome(v)
	}
	if v := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); v != "" {
		return filepath.Join(ExpandHome(v), "uclash")
	}
	return filepath.Join(home(), ".local", "share", "uclash")
}

func ConfigFile() string   { return filepath.Join(ConfigDir(), "config.yaml") }
func BinDir() string       { return filepath.Join(DataDir(), "bin") }
func CorePath() string     { return filepath.Join(BinDir(), "mihomo") }
func UIDir() string        { return filepath.Join(DataDir(), "ui") }
func ProfilesDir() string  { return filepath.Join(DataDir(), "profiles") }
func RunDir() string       { return filepath.Join(DataDir(), "run") }
func LogDir() string       { return filepath.Join(DataDir(), "logs") }
func LogFile() string      { return filepath.Join(LogDir(), "mihomo.log") }
func PIDFile() string      { return filepath.Join(RunDir(), "mihomo.pid") }
func LockFile() string     { return filepath.Join(RunDir(), "uclash.lock") }
func RuntimeConfig() string { return filepath.Join(DataDir(), "config.yaml") }
func RegistryFile() string { return filepath.Join(ProfilesDir(), "registry.yaml") }

// LocalBin is the npm-style per-user install target.
func LocalBin() string { return filepath.Join(home(), ".local", "bin") }

// ExpandHome expands a leading ~ in a path.
func ExpandHome(p string) string {
	if p == "~" {
		return home()
	}
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		return filepath.Join(home(), p[2:])
	}
	return p
}

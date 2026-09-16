package state

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const fileHeader = `# uclash user configuration.
# This file is safe to edit. Ports are re-validated on every start:
# if a port is already taken by another process (e.g. another user's
# uclash instance) a new free port is picked automatically.
`

const defaultNoProxy = "localhost,127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,169.254.0.0/16,.local"

type Ports struct {
	Mixed      int `yaml:"mixed"`
	Controller int `yaml:"controller"`
}

type Core struct {
	Version string `yaml:"version"`
}

type UI struct {
	Name string `yaml:"name"`
}

type Shell struct {
	Integration bool   `yaml:"integration"`
	RCFile      string `yaml:"rc-file"`
}

type Config struct {
	Mode            string `yaml:"mode"`
	LogLevel        string `yaml:"log-level"`
	Ports           Ports  `yaml:"ports"`
	Secret          string `yaml:"secret"`
	Mirror          string `yaml:"mirror"`
	UserAgent       string `yaml:"user-agent"`
	Core            Core   `yaml:"core"`
	UI              UI     `yaml:"ui"`
	Shell           Shell  `yaml:"shell"`
	AutoUpdateHours int    `yaml:"auto-update-hours"`
}

func Default() *Config {
	return &Config{
		Mode:            "rule",
		LogLevel:        "info",
		UI:              UI{Name: "metacubexd"},
		AutoUpdateHours: 24,
	}
}

func NewSecret() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "uclash-insecure-secret"
	}
	return hex.EncodeToString(b)
}

// Load reads the config file, returning defaults if it does not exist.
func Load(path string) (*Config, error) {
	c := Default()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return c, nil
		}
		return nil, err
	}
	if err := yaml.Unmarshal(data, c); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if c.Mode == "" {
		c.Mode = "rule"
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.UI.Name == "" {
		c.UI.Name = "metacubexd"
	}
	if c.AutoUpdateHours <= 0 {
		c.AutoUpdateHours = 24
	}
	return c, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	body, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return writeAtomic(path, append([]byte(fileHeader), body...), 0o600)
}

func (c *Config) Validate() error {
	switch c.Mode {
	case "rule", "global", "direct":
	default:
		return fmt.Errorf("invalid mode %q (want rule|global|direct)", c.Mode)
	}
	switch c.LogLevel {
	case "silent", "error", "warning", "info", "debug":
	default:
		return fmt.Errorf("invalid log-level %q", c.LogLevel)
	}
	return nil
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

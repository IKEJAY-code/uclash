package app

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/convert"
	"gitee.com/IKEJAY-code/uclash/internal/core"
	"gitee.com/IKEJAY-code/uclash/internal/download"
	"gitee.com/IKEJAY-code/uclash/internal/mihomoapi"
	"gitee.com/IKEJAY-code/uclash/internal/paths"
	"gitee.com/IKEJAY-code/uclash/internal/port"
	"gitee.com/IKEJAY-code/uclash/internal/state"
	"gitee.com/IKEJAY-code/uclash/internal/submerge"
	"gitee.com/IKEJAY-code/uclash/internal/version"
)

const listenHost = "127.0.0.1"

var nameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type App struct {
	ConfigPath string
	DataDir    string
	Cfg        *state.Config
}

func New(configPath, dataDir string) (*App, error) {
	if dataDir == "" {
		dataDir = os.Getenv("UCLASH_HOME")
	}
	if dataDir == "" {
		dataDir = paths.DataDir()
	}
	if configPath == "" {
		configPath = os.Getenv("UCLASH_CONFIG")
	}
	if configPath == "" {
		configPath = paths.ConfigFile()
	}
	cfg, err := state.Load(configPath)
	if err != nil {
		return nil, err
	}
	return &App{ConfigPath: configPath, DataDir: dataDir, Cfg: cfg}, nil
}

func (a *App) Save() error { return a.Cfg.Save(a.ConfigPath) }

func (a *App) EnsureDirs() error {
	dirs := []string{
		a.DataDir,
		filepath.Dir(a.ConfigPath),
		a.BinDir(),
		a.UIDir(),
		a.ProfilesDir(),
		a.RunDir(),
		a.LogDir(),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) CorePath() string         { return filepath.Join(a.DataDir, "bin", "mihomo") }
func (a *App) BinDir() string           { return filepath.Join(a.DataDir, "bin") }
func (a *App) UIDir() string            { return filepath.Join(a.DataDir, "ui") }
func (a *App) ProfilesDir() string      { return filepath.Join(a.DataDir, "profiles") }
func (a *App) RunDir() string           { return filepath.Join(a.DataDir, "run") }
func (a *App) LogDir() string           { return filepath.Join(a.DataDir, "logs") }
func (a *App) LogFile() string          { return filepath.Join(a.LogDir(), "mihomo.log") }
func (a *App) PIDFile() string          { return filepath.Join(a.RunDir(), "mihomo.pid") }
func (a *App) LockFile() string         { return filepath.Join(a.RunDir(), "uclash.lock") }
func (a *App) RuntimeConfigPath() string { return filepath.Join(a.DataDir, "config.yaml") }
func (a *App) RegistryPath() string     { return filepath.Join(a.ProfilesDir(), "registry.yaml") }
func (a *App) ProfilePath(name string) string {
	return filepath.Join(a.ProfilesDir(), name+".yaml")
}

func (a *App) ControllerAddr() string {
	return fmt.Sprintf("%s:%d", listenHost, a.Cfg.Ports.Controller)
}

func (a *App) Manager() *core.Manager {
	m := core.NewManager(a.DataDir, a.CorePath(), a.RuntimeConfigPath(), a.LogFile(), a.PIDFile(), a.LockFile())
	m.APIPort = a.Cfg.Ports.Controller
	m.Secret = a.Cfg.Secret
	return m
}

func (a *App) Running() (*core.PIDInfo, bool) { return a.Manager().Running() }

func (a *App) Client() (*mihomoapi.Client, bool) {
	if _, ok := a.Running(); !ok {
		return nil, false
	}
	return mihomoapi.New(a.Cfg.Ports.Controller, a.Cfg.Secret), true
}

func (a *App) Registry() (*state.Registry, error) {
	return state.LoadRegistry(a.RegistryPath())
}

func (a *App) SaveRegistry(r *state.Registry) error {
	r.Sort()
	return r.Save(a.RegistryPath())
}

// EnsurePorts verifies the configured ports are still usable and picks fresh
// ones when another process (typically another user's uclash) took them.
//
// It must never reallocate while our own core is running: the live process
// owns the current ports, and rewriting the state behind its back would make
// hot reloads target the wrong port.
func (a *App) EnsurePorts() (bool, error) {
	if _, running := a.Running(); running {
		return false, nil
	}
	changed := false
	if a.Cfg.Ports.Mixed <= 0 || !port.IsFree(listenHost, a.Cfg.Ports.Mixed) {
		p, err := port.Free(listenHost)
		if err != nil {
			return changed, err
		}
		a.Cfg.Ports.Mixed = p
		changed = true
	}
	if a.Cfg.Ports.Controller <= 0 || a.Cfg.Ports.Controller == a.Cfg.Ports.Mixed ||
		!port.IsFree(listenHost, a.Cfg.Ports.Controller) {
		p, err := port.Free(listenHost)
		if err != nil {
			return changed, err
		}
		a.Cfg.Ports.Controller = p
		changed = true
	}
	return changed, nil
}

// GenerateConfig renders the runtime config from the active profile plus
// uclash's local overrides.
func (a *App) GenerateConfig() error {
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	if reg.Active == "" {
		return errors.New("no active profile; run `uclash sub add <url>` or `uclash profile import <file.yaml>`")
	}
	p := reg.Find(reg.Active)
	if p == nil {
		return fmt.Errorf("active profile %q is missing from the registry", reg.Active)
	}
	raw, err := os.ReadFile(a.ProfilePath(p.Name))
	if err != nil {
		return fmt.Errorf("read profile %q: %w", p.Name, err)
	}
	ov := submerge.Overrides{
		MixedPort:  a.Cfg.Ports.Mixed,
		Controller: a.ControllerAddr(),
		Secret:     a.Cfg.Secret,
		UIDir:      a.UIDir(),
		Mode:       a.Cfg.Mode,
		LogLevel:   a.Cfg.LogLevel,
	}
	if a.Cfg.Mirror != "" {
		ov.GeoxURL = map[string]string{
			"geoip":   download.MirrorURL(a.Cfg.Mirror, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat"),
			"geosite": download.MirrorURL(a.Cfg.Mirror, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat"),
			"mmdb":    download.MirrorURL(a.Cfg.Mirror, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"),
			"asn":     download.MirrorURL(a.Cfg.Mirror, "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/GeoLite2-ASN.mmdb"),
		}
	}
	out, err := submerge.Merge(raw, ov)
	if err != nil {
		return fmt.Errorf("build config: %w", err)
	}
	return download.WriteFileAtomic(a.RuntimeConfigPath(), out, 0o600)
}

// WriteProfile stores a profile body, keeping a .bak of the previous version
// so a bad subscription update can be recovered with a single copy.
func (a *App) WriteProfile(name string, body []byte) error {
	path := a.ProfilePath(name)
	if old, err := os.ReadFile(path); err == nil {
		_ = download.WriteFileAtomic(path+".bak", old, 0o600)
	}
	return download.WriteFileAtomic(path, body, 0o600)
}

// ApplyReload regenerates the config and hot-reloads a running core. If the
// core rejects the new config, the previous runtime config is restored so a
// later restart cannot fail because of a rejected candidate.
func (a *App) ApplyReload(ctx context.Context) error {
	backup, _ := os.ReadFile(a.RuntimeConfigPath())
	if err := a.GenerateConfig(); err != nil {
		return err
	}
	client, ok := a.Client()
	if !ok {
		return nil
	}
	if err := client.Reload(ctx, a.RuntimeConfigPath()); err != nil {
		if backup != nil {
			_ = download.WriteFileAtomic(a.RuntimeConfigPath(), backup, 0o600)
		}
		return fmt.Errorf("the config was rejected by mihomo: %w", err)
	}
	return nil
}

// SetActive switches the active profile, regenerating and reloading as needed.
// A rejected config rolls the active profile back.
func (a *App) SetActive(ctx context.Context, name string) error {
	reg, err := a.Registry()
	if err != nil {
		return err
	}
	if reg.Find(name) == nil {
		return fmt.Errorf("profile %q not found (see `uclash sub ls`)", name)
	}
	prev := reg.Active
	reg.Active = name
	if err := a.SaveRegistry(reg); err != nil {
		return err
	}
	if err := a.ApplyReload(ctx); err != nil {
		reg.Active = prev
		_ = a.SaveRegistry(reg)
		_ = a.GenerateConfig()
		if prev == "" {
			return fmt.Errorf("%w (no active profile)", err)
		}
		return fmt.Errorf("%w (kept profile %q active)", err, prev)
	}
	return nil
}

func (a *App) DownloadOptions(github bool) download.Options {
	ua := a.Cfg.UserAgent
	if ua == "" {
		if github {
			ua = "uclash/" + version.Version
		} else {
			ua = "mihomo/1.19.0"
		}
	}
	opts := download.Options{UserAgent: ua}
	if github {
		opts.Mirror = a.Cfg.Mirror
	}
	return opts
}

// Subscription is the outcome of fetching a subscription URL.
type Subscription struct {
	Body      []byte
	URL       string // the URL that actually worked (may carry clash flags)
	Converted bool   // true when base64/plain node links were converted to YAML
}

// FetchSubscription downloads a subscription and normalizes it into a
// Clash/Mihomo YAML config. Base64 or plain node-link subscriptions are
// converted on the fly (vmess/vless/trojan/ss/ssr/hysteria2/tuic/socks5).
func (a *App) FetchSubscription(ctx context.Context, rawURL string) (*Subscription, error) {
	opts := a.DownloadOptions(false)
	body, err := download.Fetch(ctx, rawURL, opts)
	if err == nil {
		if sub, hint := interpretSubscription(body, rawURL); sub != nil {
			return sub, nil
		} else {
			err = errors.New(hint)
		}
	}
	for _, cand := range clashVariants(rawURL) {
		b, ferr := download.Fetch(ctx, cand, opts)
		if ferr != nil {
			continue
		}
		if sub, _ := interpretSubscription(b, cand); sub != nil {
			return sub, nil
		}
	}
	return nil, err
}

func interpretSubscription(body []byte, src string) (*Subscription, string) {
	if err := submerge.Validate(body); err == nil {
		return &Subscription{Body: body, URL: src}, ""
	} else {
		if converted, ok, cerr := convert.Subscription(body); ok {
			return &Subscription{Body: converted, URL: src, Converted: true}, ""
		} else if cerr != nil {
			return nil, fmt.Sprintf("%v; node-link conversion failed too: %v", err, cerr)
		}
		return nil, err.Error()
	}
}

func clashVariants(raw string) []string {
	u, perr := url.Parse(raw)
	if perr != nil || u.Scheme == "" {
		return nil
	}
	var out []string
	for _, kv := range [][2]string{{"flag", "clash"}, {"target", "clash"}} {
		v := *u
		q := v.Query()
		q.Set(kv[0], kv[1])
		v.RawQuery = q.Encode()
		out = append(out, v.String())
	}
	v := *u
	q := v.Query()
	q.Set("flag", "clash")
	q.Set("target", "clash")
	v.RawQuery = q.Encode()
	return append(out, v.String())
}

func ValidProfileName(name string) bool { return nameRe.MatchString(name) }

func HumanSince(ts string) string {
	if ts == "" {
		return "never"
	}
	t, err := time.Parse(time.RFC3339, ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 48*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

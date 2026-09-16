package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/download"
	"gitee.com/IKEJAY-code/uclash/internal/fsx"
)

// geodataDefaults are the upstream URLs mihomo uses when a profile needs
// GeoIP/GeoSite data and nothing is cached locally.
var geodataDefaults = map[string]string{
	"geoip":   "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geoip.dat",
	"geosite": "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/geosite.dat",
	"mmdb":    "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb",
	"asn":     "https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/GeoLite2-ASN.mmdb",
}

var geodataMirrors = []string{
	"https://gh-proxy.com",
	"https://ghfast.top",
	"https://ghproxy.net",
}

// geodataFiles maps a geox-url key to the file mihomo looks for in its data dir.
var geodataFiles = map[string]string{
	"geoip":   "geoip.dat",
	"geosite": "geosite.dat",
	"mmdb":    "country.mmdb",
	"asn":     "GeoLite2-ASN.mmdb",
}

// GeodataFile returns the file name mihomo expects for a geox-url key.
func GeodataFile(key string) string { return geodataFiles[key] }

// NeededGeodata inspects a rendered config and returns the geox-url keys whose
// databases mihomo will need at load time.
func NeededGeodata(config []byte) []string {
	text := string(config)
	var keys []string
	if strings.Contains(text, "GEOIP,") {
		if strings.Contains(text, "geodata-mode: true") {
			keys = append(keys, "geoip")
		} else {
			keys = append(keys, "mmdb")
		}
	}
	if strings.Contains(text, "GEOSITE,") {
		keys = append(keys, "geosite")
	}
	if strings.Contains(text, "IP-ASN,") {
		keys = append(keys, "asn")
	}
	return keys
}

// GeoxURLs resolves the effective download URLs: built-in defaults, then the
// user's geox-url overrides, then the mirror prefix for anything on github.com.
func (a *App) GeoxURLs() map[string]string {
	urls := make(map[string]string, len(geodataDefaults))
	for k, v := range geodataDefaults {
		urls[k] = v
	}
	for k, v := range a.Cfg.GeoxURL {
		if strings.TrimSpace(v) != "" {
			urls[k] = strings.TrimSpace(v)
		}
	}
	if a.Cfg.Mirror != "" {
		for k, v := range urls {
			if strings.Contains(v, "github.com") {
				urls[k] = download.MirrorURL(a.Cfg.Mirror, v)
			}
		}
	}
	return urls
}

// EnsureGeodata pre-downloads the databases the active config needs, with
// mirror fallback. Best-effort: when it cannot fetch a file the caller should
// continue and let mihomo report the problem.
func (a *App) EnsureGeodata(ctx context.Context) error {
	data, err := os.ReadFile(a.RuntimeConfigPath())
	if err != nil {
		return nil // no rendered config yet
	}
	urls := a.GeoxURLs()
	var firstErr error
	for _, key := range NeededGeodata(data) {
		file, ok := geodataFiles[key]
		if !ok {
			continue
		}
		url := urls[key]
		if url == "" {
			continue
		}
		dest := filepath.Join(a.DataDir, file)
		if fsx.Exists(dest) {
			continue
		}
		if err := a.fetchGeodata(ctx, url, dest); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("%s: %w", file, err)
		}
	}
	return firstErr
}

func (a *App) fetchGeodata(ctx context.Context, url, dest string) error {
	opts := download.Options{UserAgent: a.subscriptionUserAgent()}
	candidates := []string{url}
	if strings.Contains(url, "github.com") {
		for _, m := range geodataMirrors {
			candidates = append(candidates, download.MirrorURL(m, url))
		}
	}
	var lastErr error
	for _, candidate := range candidates {
		data, err := download.Fetch(ctx, candidate, opts)
		if err != nil {
			lastErr = err
			continue
		}
		if download.LooksLikeHTML(data) || len(data) < 50<<10 {
			lastErr = errors.New("response does not look like a geodata file")
			continue
		}
		if err := download.WriteFileAtomic(dest, data, 0o644); err != nil {
			return err
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("no candidate URL worked")
	}
	return lastErr
}

// geodataHint augments mihomo's "can't download MMDB" style errors.
func geodataHint(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(msg, "can't download") || strings.Contains(msg, "MMDB") || strings.Contains(msg, "geodata") {
		return fmt.Errorf("%w\nhint: geodata download failed; retry `uclash sub use`/`uclash restart`, or pin a fast source in ~/.config/uclash/config.yaml:\n  geox-url:\n    mmdb: https://your.mirror/country.mmdb\n    geosite: https://your.mirror/geosite.dat", err)
	}
	return err
}

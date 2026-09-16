package core

import (
	"context"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/download"
)

// FetchCore downloads the mihomo core for the current GOOS/GOARCH into dest.
// version may be empty for "latest stable" or an explicit tag like v1.19.30.
// Returns the resolved tag.
func FetchCore(ctx context.Context, version string, opts download.Options, dest string) (string, error) {
	tag := strings.TrimSpace(version)
	if tag == "" {
		t, err := download.LatestReleaseTag(ctx, "MetaCubeX/mihomo", opts)
		if err != nil {
			return "", err
		}
		tag = t
	}
	if !strings.HasPrefix(tag, "v") {
		tag = "v" + tag
	}

	suffix := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
	candidates := []string{
		fmt.Sprintf("mihomo-%s-compatible-%s.gz", suffix, tag),
		fmt.Sprintf("mihomo-%s-%s.gz", suffix, tag),
	}
	var lastErr error
	for _, name := range candidates {
		target := fmt.Sprintf("https://github.com/MetaCubeX/mihomo/releases/download/%s/%s", tag, name)
		body, err := download.Fetch(ctx, download.MirrorURL(opts.Mirror, target), opts)
		if err != nil {
			lastErr = err
			continue
		}
		data, err := download.Gunzip(body)
		if err != nil {
			lastErr = err
			continue
		}
		if err := download.WriteFileAtomic(dest, data, 0o755); err != nil {
			return "", err
		}
		return tag, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no asset for %s", suffix)
	}
	return "", fmt.Errorf("download mihomo %s for %s: %w", tag, suffix, lastErr)
}

// FetchUI downloads the metacubexd dashboard static files into destDir.
func FetchUI(ctx context.Context, opts download.Options, destDir string) error {
	target := download.MirrorURL(opts.Mirror, "https://github.com/MetaCubeX/metacubexd/archive/refs/heads/gh-pages.zip")
	body, err := download.Fetch(ctx, target, opts)
	if err != nil {
		return err
	}
	if download.LooksLikeHTML(body) {
		return fmt.Errorf("download UI: got HTML instead of zip (blocked or mirror misconfigured?)")
	}
	tmp := destDir + ".tmp"
	if err := removeAll(tmp); err != nil {
		return err
	}
	if err := download.ExtractZip(body, tmp); err != nil {
		return err
	}
	if !download.Exists(path.Join(tmp, "index.html")) {
		entries, _ := os.ReadDir(tmp)
		names := make([]string, 0, 5)
		for i, e := range entries {
			if i >= 5 {
				names = append(names, "...")
				break
			}
			names = append(names, e.Name())
		}
		return fmt.Errorf("download UI: index.html missing in archive (top-level entries: %s)", strings.Join(names, ", "))
	}
	if err := removeAll(destDir); err != nil {
		return err
	}
	return rename(tmp, destDir)
}

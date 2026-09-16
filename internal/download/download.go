package download

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const maxBody = 128 << 20 // 128 MiB

type Options struct {
	// Mirror is a GitHub acceleration prefix, e.g. https://gh-proxy.com.
	// Empty means direct GitHub access.
	Mirror    string
	UserAgent string
	Timeout   time.Duration
}

func (o Options) ua() string {
	if o.UserAgent != "" {
		return o.UserAgent
	}
	return "uclash/dev"
}

func (o Options) timeout() time.Duration {
	if o.Timeout > 0 {
		return o.Timeout
	}
	return 120 * time.Second
}

// MirrorURL prefixes target with the mirror when one is configured.
func MirrorURL(mirror, target string) string {
	m := strings.TrimRight(strings.TrimSpace(mirror), "/")
	if m == "" {
		return target
	}
	return m + "/" + target
}

func Fetch(ctx context.Context, target string, opts Options) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", opts.ua())
	client := &http.Client{Timeout: opts.timeout()}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", target, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s: HTTP %d", target, resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", target, err)
	}
	if len(body) > maxBody {
		return nil, fmt.Errorf("GET %s: response larger than %d bytes", target, maxBody)
	}
	return body, nil
}

func Gunzip(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	defer zr.Close()
	out, err := io.ReadAll(io.LimitReader(zr, maxBody))
	if err != nil {
		return nil, fmt.Errorf("gunzip: %w", err)
	}
	return out, nil
}

// ExtractZip unpacks a zip archive into destDir, stripping a single common
// top-level directory (as produced by GitHub "archive" zips).
func ExtractZip(data []byte, destDir string) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("unzip: %w", err)
	}
	prefix := commonPrefix(zr)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}
	for _, f := range zr.File {
		name := strings.TrimPrefix(f.Name, prefix)
		name = strings.TrimPrefix(name, "/")
		if name == "" {
			continue
		}
		clean := filepath.Clean(filepath.FromSlash(name))
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			return fmt.Errorf("unzip: unsafe path %q", f.Name)
		}
		target := filepath.Join(destDir, clean)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
		if err != nil {
			rc.Close()
			return err
		}
		_, err = io.Copy(out, io.LimitReader(rc, maxBody))
		rc.Close()
		out.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// commonPrefix finds the single top-level directory shared by all entries
// (as produced by GitHub "archive" zips), so it can be stripped on extraction.
// Bare top-level directory entries (e.g. "repo-branch/") are ignored: GitHub
// archives contain them, and treating them as "no common prefix" used to leave
// every file nested one level too deep.
func commonPrefix(zr *zip.Reader) string {
	prefix := ""
	for _, f := range zr.File {
		name := strings.Trim(f.Name, "/")
		if name == "" {
			continue
		}
		if !strings.Contains(name, "/") {
			if f.FileInfo().IsDir() {
				if prefix == "" {
					prefix = name + "/"
				}
				continue
			}
			return ""
		}
		if prefix == "" {
			prefix = strings.SplitN(name, "/", 2)[0] + "/"
		}
		if !strings.HasPrefix(f.Name, prefix) {
			return ""
		}
	}
	return prefix
}

// LatestReleaseTag resolves the latest (non-prerelease) tag of a GitHub repo,
// working through a mirror when configured.
func LatestReleaseTag(ctx context.Context, repo string, opts Options) (string, error) {
	latest := "https://github.com/" + repo + "/releases/latest"
	targets := []string{MirrorURL(opts.Mirror, latest)}
	client := &http.Client{
		Timeout: opts.timeout(),
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	for _, t := range targets {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, t, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", opts.ua())
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		loc := resp.Header.Get("Location")
		resp.Body.Close()
		if resp.StatusCode >= 300 && resp.StatusCode < 400 && loc != "" {
			if u, err := url.Parse(loc); err == nil {
				if i := strings.Index(u.Path, "/tag/"); i >= 0 {
					tag := path.Base(u.Path[i+len("/tag/"):])
					if tag != "" && tag != "." && tag != "/" {
						return tag, nil
					}
				}
			}
		}
	}
	// Fallback: GitHub API (also mirror-prefixed when configured).
	api := MirrorURL(opts.Mirror, "https://api.github.com/repos/"+repo+"/releases/latest")
	body, err := Fetch(ctx, api, opts)
	if err != nil {
		return "", fmt.Errorf("cannot resolve latest release of %s (try --version): %w", repo, err)
	}
	var v struct {
		TagName string `json:"tag_name"`
	}
	if err := json.Unmarshal(body, &v); err != nil || v.TagName == "" {
		return "", fmt.Errorf("cannot resolve latest release of %s (try --version)", repo)
	}
	return v.TagName, nil
}

func WriteFileAtomic(path string, data []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func LooksLikeHTML(data []byte) bool {
	head := strings.TrimSpace(string(data[:min(len(data), 512)]))
	return strings.HasPrefix(strings.ToLower(head), "<!doctype html") || strings.HasPrefix(strings.ToLower(head), "<html")
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

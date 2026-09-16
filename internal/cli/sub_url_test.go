package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeSubURLAcceptsValidURL(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := normalizeSubURL("https://example.com/sub?token=abc", out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com/sub?token=abc" {
		t.Errorf("url changed: %q", got)
	}
	if out.Len() != 0 {
		t.Errorf("unexpected output: %q", out.String())
	}
}

func TestNormalizeSubURLStripsShellEscapes(t *testing.T) {
	out := &bytes.Buffer{}
	got, err := normalizeSubURL(`https://example.com/sub\?token\=abc`, out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "https://example.com/sub?token=abc" {
		t.Errorf("url = %q", got)
	}
	if !strings.Contains(out.String(), "removed backslashes") {
		t.Errorf("expected a warning, got %q", out.String())
	}
}

func TestNormalizeSubURLLocalFileHint(t *testing.T) {
	file := filepath.Join(t.TempDir(), "clash.yaml")
	if err := os.WriteFile(file, []byte("proxies: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := normalizeSubURL(file, &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error for a local path")
	}
	if !strings.Contains(err.Error(), "profile import") {
		t.Errorf("error should point at `uclash profile import`: %v", err)
	}
}

func TestNormalizeSubURLMissingScheme(t *testing.T) {
	_, err := normalizeSubURL("example.com/sub", &bytes.Buffer{})
	if err == nil {
		t.Fatal("expected an error for a scheme-less URL")
	}
	if !strings.Contains(err.Error(), "http://") {
		t.Errorf("error should mention the expected scheme: %v", err)
	}
}

package download

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

// buildZip creates an in-memory zip from name->content pairs. Names ending in
// "/" are stored as directory entries (as GitHub archives do).
func buildZip(t *testing.T, entries map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range entries {
		if name[len(name)-1] == '/' {
			if _, err := zw.Create(name); err != nil {
				t.Fatal(err)
			}
			continue
		}
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

// GitHub archives start with a bare top-level directory entry; extraction must
// still strip the wrapper directory (regression test for the metacubexd UI
// download failing with "index.html missing in archive").
func TestExtractZipStripsWrapperWithDirectoryEntry(t *testing.T) {
	data := buildZip(t, map[string]string{
		"metacubexd-gh-pages/":                 "",
		"metacubexd-gh-pages/index.html":       "<html>ui</html>",
		"metacubexd-gh-pages/_nuxt/app.js":     "console.log(1)",
		"metacubexd-gh-pages/_fonts/font.woff": "font",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err != nil {
		t.Fatalf("ExtractZip: %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "index.html")); got != "<html>ui</html>" {
		t.Errorf("index.html = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "_nuxt", "app.js")); got != "console.log(1)" {
		t.Errorf("_nuxt/app.js = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dest, "metacubexd-gh-pages")); !os.IsNotExist(err) {
		t.Error("wrapper directory should not survive extraction")
	}
}

func TestExtractZipStripsWrapperWithoutDirectoryEntry(t *testing.T) {
	data := buildZip(t, map[string]string{
		"repo-main/index.html": "x",
		"repo-main/a/b.txt":    "y",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "index.html")); err != nil {
		t.Fatalf("index.html not at root: %v", err)
	}
}

func TestExtractZipWithoutWrapperKeepsLayout(t *testing.T) {
	data := buildZip(t, map[string]string{
		"index.html": "root",
		"sub/a.js":   "a",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, filepath.Join(dest, "index.html")); got != "root" {
		t.Errorf("index.html = %q", got)
	}
}

func TestExtractZipMultipleTopLevelDirsKeepsLayout(t *testing.T) {
	data := buildZip(t, map[string]string{
		"a/index.html": "a",
		"b/index.html": "b",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, "a", "index.html")); err != nil {
		t.Error("multiple top-level directories must not be stripped")
	}
	if _, err := os.Stat(filepath.Join(dest, "b", "index.html")); err != nil {
		t.Error("multiple top-level directories must not be stripped")
	}
}

func TestExtractZipRejectsUnsafePaths(t *testing.T) {
	data := buildZip(t, map[string]string{
		"repo-main/../../evil.txt": "boom",
	})
	dest := t.TempDir()
	if err := ExtractZip(data, dest); err == nil {
		t.Fatal("expected unsafe path to be rejected")
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(dest), "evil.txt")); !os.IsNotExist(err) {
		t.Fatal("unsafe file escaped the destination")
	}
}

func TestLooksLikeHTML(t *testing.T) {
	if !LooksLikeHTML([]byte("  <!DOCTYPE html>\n<html>")) {
		t.Error("html should be detected")
	}
	if LooksLikeHTML([]byte("PK\x03\x04zipdata")) {
		t.Error("zip must not be treated as html")
	}
}

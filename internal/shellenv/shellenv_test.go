package shellenv

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportOnSetsProxyVarsAndNoProxy(t *testing.T) {
	out := ExportOn(12345)
	for _, want := range []string{
		`export http_proxy="http://127.0.0.1:12345"`,
		`export https_proxy="http://127.0.0.1:12345"`,
		`export HTTP_PROXY="http://127.0.0.1:12345"`,
		`export no_proxy="localhost,127.0.0.1`,
		`export NO_PROXY="localhost,127.0.0.1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ExportOn missing %q", want)
		}
	}
}

func TestExportOffOnlyUnsetsOurValue(t *testing.T) {
	out := ExportOff(12345)
	if !strings.Contains(out, `[ "${http_proxy:-}" = "http://127.0.0.1:12345" ] && unset http_proxy`) {
		t.Fatalf("ExportOff should guard the unset with a value check:\n%s", out)
	}
	if strings.Contains(out, "export ") {
		t.Error("ExportOff must not export anything")
	}
}

func TestFishOnOff(t *testing.T) {
	on := FishOn(12345)
	for _, want := range []string{
		`set -gx http_proxy "http://127.0.0.1:12345"`,
		`set -gx HTTPS_PROXY "http://127.0.0.1:12345"`,
		`set -gx NO_PROXY "localhost,127.0.0.1`,
	} {
		if !strings.Contains(on, want) {
			t.Errorf("FishOn missing %q", want)
		}
	}
	off := FishOff(12345)
	if !strings.Contains(off, `if test "$http_proxy" = "http://127.0.0.1:12345"; set -e http_proxy; end`) {
		t.Errorf("FishOff should guard the unset:\n%s", off)
	}
	if strings.Contains(off, "set -gx") {
		t.Error("FishOff must not set anything")
	}
}

func TestRCForShellRespectsZDOTDIRAndXDG(t *testing.T) {
	t.Setenv("ZDOTDIR", "/custom/zdotdir")
	if got, want := RCForShell("zsh"), filepath.Join("/custom/zdotdir", ".zshrc"); got != want {
		t.Errorf("zsh rc = %q, want %q", got, want)
	}
	t.Setenv("ZDOTDIR", "")
	if got := RCForShell("zsh"); !strings.HasSuffix(got, ".zshrc") {
		t.Errorf("zsh rc without ZDOTDIR = %q", got)
	}
	t.Setenv("XDG_CONFIG_HOME", "/custom/xdg")
	if got, want := RCForShell("fish"), filepath.Join("/custom/xdg", "fish", "config.fish"); got != want {
		t.Errorf("fish rc = %q, want %q", got, want)
	}
	if got := RCForShell("bash"); !strings.HasSuffix(got, ".bashrc") {
		t.Errorf("bash rc = %q", got)
	}
}

func TestBlockDefinesUclashWrapper(t *testing.T) {
	b := Block()
	for _, want := range []string{`uclash()`, `command uclash proxy "$2"`, `command uclash "$@"`, beginMarker, endMarker} {
		if !strings.Contains(b, want) {
			t.Errorf("Block missing %q", want)
		}
	}
	for _, banned := range []string{"proxyon", "proxyoff"} {
		if strings.Contains(b, banned) {
			t.Errorf("Block should no longer define %q", banned)
		}
	}
}

func TestFishBlockDefinesUclashWrapper(t *testing.T) {
	f := FishBlock()
	for _, want := range []string{`function uclash`, `command uclash proxy on --fish | source`, `command uclash $argv`} {
		if !strings.Contains(f, want) {
			t.Errorf("FishBlock missing %q", want)
		}
	}
}

func TestInstallReplacesOldBlockAndIsIdempotent(t *testing.T) {
	rc := filepath.Join(t.TempDir(), ".zshrc")
	old := "# >>> uclash initialize >>>\nproxyon() { :; }\n# <<< uclash initialize <<<\n"
	if err := os.WriteFile(rc, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	changed, err := Install(rc, "zsh")
	if err != nil || !changed {
		t.Fatalf("Install: changed=%v err=%v", changed, err)
	}
	content, _ := os.ReadFile(rc)
	if strings.Contains(string(content), "proxyon") {
		t.Error("old helper block should have been replaced")
	}
	if !strings.Contains(string(content), "command uclash proxy") {
		t.Error("wrapper missing after install")
	}
	if changed, err := Install(rc, "zsh"); err != nil || changed {
		t.Errorf("second Install should be a no-op: changed=%v err=%v", changed, err)
	}
	if !Installed(rc) {
		t.Error("Installed() should report true")
	}
	if changed, err := Uninstall(rc); err != nil || !changed {
		t.Errorf("Uninstall: changed=%v err=%v", changed, err)
	}
	if Installed(rc) {
		t.Error("Installed() should report false after uninstall")
	}
}

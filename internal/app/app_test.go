package app

import (
	"strings"
	"testing"

	"gitee.com/IKEJAY-code/uclash/internal/state"
)

func TestValidProfileName(t *testing.T) {
	valid := []string{"local", "MyAirport", "sub-1", "a.b_c", "x"}
	for _, v := range valid {
		if !ValidProfileName(v) {
			t.Errorf("%q should be valid", v)
		}
	}
	invalid := []string{"", "-x", ".x", "../etc/passwd", "a/b", "with space", "工具", strings.Repeat("a", 65)}
	for _, v := range invalid {
		if ValidProfileName(v) {
			t.Errorf("%q should be invalid", v)
		}
	}
}

func TestHumanSince(t *testing.T) {
	if got := HumanSince(""); got != "never" {
		t.Errorf("empty timestamp: got %q", got)
	}
	if got := HumanSince("not-a-time"); got != "not-a-time" {
		t.Errorf("unparsable timestamp should pass through, got %q", got)
	}
}

func TestSubscriptionUserAgent(t *testing.T) {
	a := &App{DataDir: t.TempDir(), Cfg: state.Default()}

	a.Cfg.Core.Version = "v1.19.31"
	if got, want := a.subscriptionUserAgent(), "mihomo/1.19.31"; got != want {
		t.Errorf("core version UA = %q, want %q", got, want)
	}

	a.Cfg.UserAgent = "clash-verge/v2.2.0"
	if got, want := a.subscriptionUserAgent(), "clash-verge/v2.2.0"; got != want {
		t.Errorf("configured UA should win: got %q, want %q", got, want)
	}

	// Without a pinned tag and with no usable core binary, fall back to "mihomo".
	a.Cfg.UserAgent = ""
	a.Cfg.Core.Version = ""
	if got, want := a.subscriptionUserAgent(), "mihomo"; got != want {
		t.Errorf("fallback UA = %q, want %q", got, want)
	}
}

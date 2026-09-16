package app

import (
	"strings"
	"testing"
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

package core

import "testing"

func TestNormalizeVersion(t *testing.T) {
	cases := map[string]string{
		"v1.19.31": "1.19.31",
		"1.19.31":  "1.19.31",
		" v0.2.2":  "0.2.2",
		"":         "",
	}
	for in, want := range cases {
		if got := NormalizeVersion(in); got != want {
			t.Errorf("NormalizeVersion(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestBinaryVersionMissingBinary(t *testing.T) {
	if got := BinaryVersion("/nonexistent/mihomo"); got != "" {
		t.Errorf("BinaryVersion on a missing binary = %q, want empty", got)
	}
}

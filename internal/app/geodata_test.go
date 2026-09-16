package app

import (
	"errors"
	"strings"
	"testing"

	"gitee.com/IKEJAY-code/uclash/internal/state"
)

func TestNeededGeodata(t *testing.T) {
	cases := []struct {
		name string
		cfg  string
		want []string
	}{
		{"geoip default", "rules:\n  - GEOIP,CN,DIRECT\n", []string{"mmdb"}},
		{"geoip geodata-mode", "geodata-mode: true\nrules:\n  - GEOIP,CN,DIRECT\n", []string{"geoip"}},
		{"geosite", "rules:\n  - GEOSITE,cn,DIRECT\n", []string{"geosite"}},
		{"asn", "rules:\n  - IP-ASN,1234,DIRECT\n", []string{"asn"}},
		{"none", "rules:\n  - MATCH,PROXY\n", nil},
	}
	for _, tc := range cases {
		got := NeededGeodata([]byte(tc.cfg))
		if strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestGeodataFileNames(t *testing.T) {
	if GeodataFile("mmdb") != "country.mmdb" {
		t.Errorf("mmdb -> %q", GeodataFile("mmdb"))
	}
	if GeodataFile("geosite") != "geosite.dat" {
		t.Errorf("geosite -> %q", GeodataFile("geosite"))
	}
}

func TestGeoxURLsMirrorAndOverride(t *testing.T) {
	a := &App{Cfg: state.Default()}
	a.Cfg.Mirror = "https://gh-proxy.com"

	urls := a.GeoxURLs()
	if want := "https://gh-proxy.com/https://github.com/MetaCubeX/meta-rules-dat/releases/download/latest/country.mmdb"; urls["mmdb"] != want {
		t.Errorf("mirrored mmdb = %q, want %q", urls["mmdb"], want)
	}

	// A user override on a non-GitHub host must not be mirror-prefixed.
	a.Cfg.GeoxURL = map[string]string{"mmdb": "https://cdn.example/country.mmdb"}
	urls = a.GeoxURLs()
	if urls["mmdb"] != "https://cdn.example/country.mmdb" {
		t.Errorf("override mmdb = %q", urls["mmdb"])
	}
	if !strings.HasPrefix(urls["geosite"], "https://gh-proxy.com/") {
		t.Errorf("untouched entries should stay mirrored: %q", urls["geosite"])
	}

	// Overrides that point at GitHub are still mirrored.
	a.Cfg.GeoxURL = map[string]string{"mmdb": "https://github.com/x/y/releases/download/1/country.mmdb"}
	urls = a.GeoxURLs()
	if urls["mmdb"] != "https://gh-proxy.com/https://github.com/x/y/releases/download/1/country.mmdb" {
		t.Errorf("github override should be mirrored: %q", urls["mmdb"])
	}
}

func TestGeodataHint(t *testing.T) {
	base := errors.New(`rules[5221] [GEOIP,CN,DIRECT] error: can't download MMDB: EOF`)
	hinted := geodataHint(base)
	if !strings.Contains(hinted.Error(), "geox-url") {
		t.Errorf("expected a geox-url hint, got: %v", hinted)
	}
	if !errors.Is(hinted, base) {
		t.Error("hint should wrap the original error")
	}
	other := errors.New("connection reset")
	if errors.Is(geodataHint(other), other) != true {
		t.Error("unrelated errors should pass through untouched")
	}
	if strings.Contains(geodataHint(other).Error(), "geox-url") {
		t.Error("unrelated errors should not gain the geodata hint")
	}
}

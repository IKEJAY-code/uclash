package convert

import (
	"encoding/base64"
	"strings"
	"testing"

	"gitee.com/IKEJAY-code/uclash/internal/submerge"
)

func b64(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

func b64url(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }

func mustParse(t *testing.T, link string) *entry {
	t.Helper()
	e, err := ParseURI(link)
	if err != nil {
		t.Fatalf("ParseURI(%s): %v", link, err)
	}
	return e
}

func wantFields(t *testing.T, e *entry, want map[string]any) {
	t.Helper()
	for k, v := range want {
		if got := e.get(k); got != v {
			t.Errorf("field %q = %v, want %v", k, got, v)
		}
	}
}

func TestParseVmessJSON(t *testing.T) {
	payload := `{"v":"2","ps":"Vmess-WS","add":"1.2.3.4","port":"443","id":"b831381d-6324-4d53-ad4f-8cda48b30811","aid":"0","scy":"auto","net":"ws","type":"none","host":"cdn.example.com","path":"/ws","tls":"tls","sni":"cdn.example.com"}`
	e := mustParse(t, "vmess://"+b64(payload))
	wantFields(t, e, map[string]any{
		"name":   "Vmess-WS",
		"type":   "vmess",
		"server": "1.2.3.4",
		"port":   443,
		"uuid":   "b831381d-6324-4d53-ad4f-8cda48b30811",
		"alterId": 0,
		"tls":    true,
		"network": "ws",
	})
	opts, ok := e.get("ws-opts").(*entry)
	if !ok {
		t.Fatalf("ws-opts missing: %#v", e.get("ws-opts"))
	}
	if opts.get("path") != "/ws" {
		t.Errorf("ws path = %v", opts.get("path"))
	}
	hdr, _ := opts.get("headers").(*entry)
	if hdr == nil || hdr.get("Host") != "cdn.example.com" {
		t.Errorf("ws headers = %#v", opts.get("headers"))
	}
}

func TestParseVmessLegacy(t *testing.T) {
	payload := "auto:11111111-2222-3333-4444-555555555555@5.6.7.8:8080"
	e := mustParse(t, "vmess://"+b64(payload))
	wantFields(t, e, map[string]any{
		"type":    "vmess",
		"server":  "5.6.7.8",
		"port":    8080,
		"uuid":    "11111111-2222-3333-4444-555555555555",
		"cipher":  "auto",
		"alterId": 0,
	})
}

func TestParseVmessRawJSON(t *testing.T) {
	payload := `{"ps":"Raw","add":"9.9.9.9","port":"8443","id":"11111111-2222-3333-4444-555555555555","net":"ws","path":"/v","tls":"tls"}`
	e := mustParse(t, "vmess://"+payload)
	wantFields(t, e, map[string]any{
		"name":   "Raw",
		"type":   "vmess",
		"server": "9.9.9.9",
		"port":   8443,
		"tls":    true,
		"network": "ws",
	})
}

func TestParseVlessRealityGRPC(t *testing.T) {
	link := "vless://11111111-2222-3333-4444-555555555555@1.2.3.4:443?encryption=none&security=reality&sni=www.microsoft.com&fp=chrome&pbk=PUBKEY123&sid=ab12&type=grpc&serviceName=grpcsvc&flow=xtls-rprx-vision#Reality-GRPC"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":                 "Reality-GRPC",
		"type":                 "vless",
		"server":               "1.2.3.4",
		"port":                 443,
		"tls":                  true,
		"servername":           "www.microsoft.com",
		"client-fingerprint":   "chrome",
		"flow":                 "xtls-rprx-vision",
		"network":              "grpc",
	})
	ro, _ := e.get("reality-opts").(*entry)
	if ro == nil || ro.get("public-key") != "PUBKEY123" || ro.get("short-id") != "ab12" {
		t.Errorf("reality-opts = %#v", e.get("reality-opts"))
	}
	go_, _ := e.get("grpc-opts").(*entry)
	if go_ == nil || go_.get("grpc-service-name") != "grpcsvc" {
		t.Errorf("grpc-opts = %#v", e.get("grpc-opts"))
	}
}

func TestParseTrojanWS(t *testing.T) {
	link := "trojan://s3cr3t@1.2.3.4:443?security=tls&sni=t.example.com&type=ws&path=%2Ftj&host=cdn.example.com&allowInsecure=1#TrojanWS"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":             "TrojanWS",
		"type":             "trojan",
		"password":         "s3cr3t",
		"sni":              "t.example.com",
		"network":          "ws",
		"skip-cert-verify": true,
	})
	opts, _ := e.get("ws-opts").(*entry)
	if opts == nil || opts.get("path") != "/tj" {
		t.Errorf("ws-opts = %#v", e.get("ws-opts"))
	}
}

func TestParseSSSIP002(t *testing.T) {
	link := "ss://" + b64("aes-256-gcm:passw0rd") + "@1.2.3.4:8388#SS-Node"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":     "SS-Node",
		"type":     "ss",
		"server":   "1.2.3.4",
		"port":     8388,
		"cipher":   "aes-256-gcm",
		"password": "passw0rd",
	})
}

func TestParseSSLegacyAndCipherAlias(t *testing.T) {
	link := "ss://" + b64("chacha20-poly1305:pwd@1.2.3.4:5432") + "#Legacy"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":   "Legacy",
		"cipher": "chacha20-ietf-poly1305",
		"port":   5432,
	})
}

func TestParseSSWithObfsPlugin(t *testing.T) {
	link := "ss://" + b64("aes-128-gcm:pw") + "@1.2.3.4:8388/?plugin=obfs-local%3Bobfs%3Dhttp%3Bobfs-host%3Dbing.com#Obfs"
	e := mustParse(t, link)
	if e.get("plugin") != "obfs" {
		t.Fatalf("plugin = %v", e.get("plugin"))
	}
	opts, _ := e.get("plugin-opts").(*entry)
	if opts == nil || opts.get("mode") != "http" || opts.get("host") != "bing.com" {
		t.Errorf("plugin-opts = %#v", e.get("plugin-opts"))
	}
}

func TestParseSSR(t *testing.T) {
	payload := "1.2.3.4:8888:origin:aes-256-cfb:plain:" + b64url("pass") +
		"/?obfsparam=" + b64url("") + "&protoparam=" + b64url("") + "&remarks=" + b64url("SSR Node")
	e := mustParse(t, "ssr://"+b64url(payload))
	wantFields(t, e, map[string]any{
		"name":     "SSR Node",
		"type":     "ssr",
		"server":   "1.2.3.4",
		"port":     8888,
		"cipher":   "aes-256-cfb",
		"password": "pass",
		"protocol": "origin",
		"obfs":     "plain",
	})
}

func TestParseHysteria2(t *testing.T) {
	link := "hysteria2://letmein@1.2.3.4:443?sni=h.example.com&insecure=1&obfs=salamander&obfs-password=xyz#Hy2"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":             "Hy2",
		"type":             "hysteria2",
		"password":         "letmein",
		"sni":              "h.example.com",
		"obfs":             "salamander",
		"obfs-password":    "xyz",
		"skip-cert-verify": true,
	})
}

func TestParseTUIC(t *testing.T) {
	link := "tuic://11111111-2222-3333-4444-555555555555:pass@1.2.3.4:443?congestion_control=bbr&alpn=h3&sni=t.example.com&udp_relay_mode=native#Tuic"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":                  "Tuic",
		"type":                  "tuic",
		"uuid":                  "11111111-2222-3333-4444-555555555555",
		"password":              "pass",
		"congestion-controller": "bbr",
		"udp-relay-mode":        "native",
		"sni":                   "t.example.com",
	})
	if alpn, ok := e.get("alpn").([]string); !ok || len(alpn) != 1 || alpn[0] != "h3" {
		t.Errorf("alpn = %#v", e.get("alpn"))
	}
}

func TestParseSocks5(t *testing.T) {
	e := mustParse(t, "socks5://user:pass@1.2.3.4:1080#Socks")
	wantFields(t, e, map[string]any{
		"type":     "socks5",
		"username": "user",
		"password": "pass",
		"port":     1080,
	})
}

func TestSubscriptionFromBase64(t *testing.T) {
	links := strings.Join([]string{
		"vmess://" + b64(`{"ps":"A","add":"1.2.3.4","port":"443","id":"11111111-2222-3333-4444-555555555555","net":"tcp"}`),
		"trojan://pw@1.2.3.4:443?sni=x.com#B",
		"ss://" + b64("aes-128-gcm:pw") + "@1.2.3.4:8388#C",
	}, "\n")
	res, err := Subscription([]byte(b64(links)))
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	if !res.Converted {
		t.Fatal("converted = false")
	}
	if res.Stats.Parsed != 3 {
		t.Errorf("parsed = %d, want 3", res.Stats.Parsed)
	}
	s := string(res.YAML)
	for _, want := range []string{"type: vmess", "type: trojan", "type: ss", "name: A", "name: PROXY", "name: AUTO", "MATCH,PROXY"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
	if err := submerge.Validate(res.YAML); err != nil {
		t.Errorf("generated YAML rejected by Validate: %v", err)
	}
}

func TestSubscriptionFromPlainLinks(t *testing.T) {
	res, err := Subscription([]byte("trojan://pw@1.2.3.4:443#Plain\n"))
	if err != nil || !res.Converted {
		t.Fatalf("plain links: res=%+v err=%v", res, err)
	}
	if !strings.Contains(string(res.YAML), "name: Plain") {
		t.Errorf("missing node name:\n%s", res.YAML)
	}
}

func TestSubscriptionRejectsGarbage(t *testing.T) {
	if _, err := Subscription([]byte("hello world\n")); err == nil {
		t.Error("expected error for non-link content")
	}
}

func TestSubscriptionReportsSkippedSchemes(t *testing.T) {
	links := strings.Join([]string{
		"trojan://pw@1.2.3.4:443#Good",
		"ssh://user@1.2.3.4:22#Unsupported",
		"ssh://user@1.2.3.5:22#AlsoUnsupported",
		"vless://not-a-valid-link",
	}, "\n")
	res, err := Subscription([]byte(links))
	if err != nil {
		t.Fatalf("Subscription: %v", err)
	}
	if res.Stats.Parsed != 1 {
		t.Errorf("parsed = %d, want 1", res.Stats.Parsed)
	}
	if res.Stats.Unsupported["ssh"] != 2 {
		t.Errorf("unsupported ssh = %d, want 2", res.Stats.Unsupported["ssh"])
	}
	if res.Stats.Invalid["vless"] != 1 {
		t.Errorf("invalid vless = %d, want 1", res.Stats.Invalid["vless"])
	}
	summary := res.Stats.Summary()
	if !strings.Contains(summary, "1 nodes") || !strings.Contains(summary, "2 ssh") || !strings.Contains(summary, "1 vless") {
		t.Errorf("summary = %q", summary)
	}
}

func TestSubscriptionErrorNamesUnsupportedSchemes(t *testing.T) {
	_, err := Subscription([]byte("ssh://user@1.2.3.4:22#OnlyUnsupported\n"))
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ssh") {
		t.Errorf("error should name the unsupported scheme: %v", err)
	}
}

func TestDuplicateNamesGetSuffix(t *testing.T) {
	links := "trojan://pw@1.2.3.4:443#Dup\ntrojan://pw@1.2.3.5:443#Dup\n"
	res, err := Subscription([]byte(links))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(res.YAML), "name: Dup") || !strings.Contains(string(res.YAML), "name: Dup-2") {
		t.Errorf("dedupe failed:\n%s", res.YAML)
	}
}

func TestParseAnyTLS(t *testing.T) {
	link := "anytls://0fdf77d7-d4ba-455e-9ed9-a98dd6d5489a@1.2.3.4/?sni=real.example.com&insecure=1#AnyTLS%20Node"
	e := mustParse(t, link)
	wantFields(t, e, map[string]any{
		"name":             "AnyTLS Node",
		"type":             "anytls",
		"server":           "1.2.3.4",
		"port":             443,
		"password":         "0fdf77d7-d4ba-455e-9ed9-a98dd6d5489a",
		"sni":              "real.example.com",
		"skip-cert-verify": true,
		"udp":              true,
	})
}

func TestParseAnyTLSCustomPort(t *testing.T) {
	e := mustParse(t, "anytls://letmein@example.com:8964/?sni=example.com#P")
	wantFields(t, e, map[string]any{"port": 8964, "password": "letmein", "sni": "example.com"})
}

func TestParseAnyTLSMissingPassword(t *testing.T) {
	if _, err := ParseURI("anytls://1.2.3.4:443#nopass"); err == nil {
		t.Error("expected an error for a missing password")
	}
}

func TestExtractLinksIgnoresRandomText(t *testing.T) {
	if links := ExtractLinks([]byte("not a sub at all\nhttps://example.com/page\n")); len(links) != 0 {
		t.Errorf("unexpected links: %v", links)
	}
}

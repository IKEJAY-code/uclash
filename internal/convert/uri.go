package convert

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

// ParseURI turns a single node share link into a Clash proxy mapping.
func ParseURI(link string) (*entry, error) {
	link = strings.TrimSpace(link)
	i := strings.Index(link, "://")
	if i <= 0 {
		return nil, fmt.Errorf("not a node link")
	}
	scheme := strings.ToLower(link[:i])
	switch scheme {
	case "vmess":
		return parseVmess(link)
	case "vless":
		return parseVless(link)
	case "trojan":
		return parseTrojan(link)
	case "ss":
		return parseSS(link)
	case "ssr":
		return parseSSR(link)
	case "hysteria2", "hy2":
		return parseHysteria2(link)
	case "hysteria":
		return parseHysteria(link)
	case "tuic":
		return parseTUIC(link)
	case "anytls":
		return parseAnyTLS(link)
	case "socks5", "socks":
		return parseSocks(link)
	default:
		return nil, fmt.Errorf("unsupported scheme %q", scheme)
	}
}

func parseVmess(link string) (*entry, error) {
	raw := strings.TrimPrefix(link, "vmess://")
	if i := strings.Index(raw, "#"); i >= 0 {
		raw = raw[:i]
	}
	body := strings.TrimSpace(raw)
	if !strings.HasPrefix(body, "{") {
		dec, err := decodeB64(raw)
		if err != nil {
			return nil, fmt.Errorf("vmess: invalid base64 payload")
		}
		body = strings.TrimSpace(string(dec))
	}
	if strings.HasPrefix(body, "{") {
		var v struct {
			PS   string `json:"ps"`
			Add  string `json:"add"`
			Port any    `json:"port"`
			ID   string `json:"id"`
			Aid  any    `json:"aid"`
			Scy  string `json:"scy"`
			Net  string `json:"net"`
			Type string `json:"type"`
			Host string `json:"host"`
			Path string `json:"path"`
			TLS  string `json:"tls"`
			SNI  string `json:"sni"`
			ALPN any    `json:"alpn"`
			FP   string `json:"fp"`
		}
		if err := json.Unmarshal([]byte(body), &v); err != nil {
			return nil, fmt.Errorf("vmess: bad JSON payload: %w", err)
		}
		if v.Add == "" || v.ID == "" {
			return nil, fmt.Errorf("vmess: missing server or uuid")
		}
		port := anyInt(v.Port)
		if port <= 0 {
			return nil, fmt.Errorf("vmess: missing port")
		}
		e := newEntry(v.PS, "vmess", v.Add, port)
		e.set("uuid", v.ID)
		// alterId is required by mihomo even when 0; with() skips the
		// zero-value filtering that set() applies.
		e.with("alterId", anyInt(v.Aid))
		cipher := strings.ToLower(v.Scy)
		if cipher == "" {
			cipher = "auto"
		}
		e.set("cipher", cipher)
		e.set("udp", true)
		if strings.EqualFold(v.TLS, "tls") {
			e.set("tls", true)
			e.set("servername", v.SNI)
		}
		applyNetwork(e, v.Net, v.Path, v.Host, anyString(v.Type))
		e.set("client-fingerprint", v.FP)
		e.set("alpn", alpnFromAny(v.ALPN))
		return e, nil
	}
	// Legacy: base64("method:uuid@host:port")
	at := strings.LastIndex(body, "@")
	if at < 0 {
		return nil, fmt.Errorf("vmess: malformed legacy payload")
	}
	creds := strings.SplitN(body[:at], ":", 2)
	if len(creds) != 2 {
		return nil, fmt.Errorf("vmess: malformed legacy credentials")
	}
	host, port, err := splitHostPort(body[at+1:])
	if err != nil {
		return nil, err
	}
	e := newEntry("", "vmess", host, port)
	e.set("uuid", creds[1])
	e.with("alterId", 0)
	e.set("cipher", strings.ToLower(creds[0]))
	e.set("udp", true)
	return e, nil
}

func parseVless(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("vless: %w", err)
	}
	uuid := username(u)
	if uuid == "" {
		return nil, fmt.Errorf("vless: missing uuid")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "vless", host, port)
	e.set("uuid", uuid)
	e.set("udp", true)
	switch strings.ToLower(q.Get("security")) {
	case "tls", "xtls":
		e.set("tls", true)
		e.set("servername", firstNonEmpty(q.Get("sni"), q.Get("host")))
		e.set("client-fingerprint", q.Get("fp"))
	case "reality":
		pbk := q.Get("pbk")
		if pbk == "" {
			return nil, fmt.Errorf("vless: reality link without pbk")
		}
		e.set("tls", true)
		e.set("servername", q.Get("sni"))
		e.set("client-fingerprint", q.Get("fp"))
		ro := &entry{}
		ro.set("public-key", pbk)
		ro.set("short-id", q.Get("sid"))
		e.set("reality-opts", ro)
	}
	e.set("flow", q.Get("flow"))
	applyNetwork(e, q.Get("type"), q.Get("path"), q.Get("host"), q.Get("serviceName"))
	if truthy(q.Get("allowInsecure")) {
		e.set("skip-cert-verify", true)
	}
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	return e, nil
}

func parseTrojan(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("trojan: %w", err)
	}
	password := username(u)
	if password == "" {
		return nil, fmt.Errorf("trojan: missing password")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "trojan", host, port)
	e.set("password", password)
	e.set("udp", true)
	e.set("sni", firstNonEmpty(q.Get("sni"), q.Get("peer")))
	if truthy(q.Get("allowInsecure")) {
		e.set("skip-cert-verify", true)
	}
	applyNetwork(e, q.Get("type"), q.Get("path"), q.Get("host"), q.Get("serviceName"))
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	return e, nil
}

func parseSS(link string) (*entry, error) {
	body, name := cutFragment(link)
	body = strings.TrimPrefix(body, "ss://")
	var query string
	if i := strings.Index(body, "?"); i >= 0 {
		query = body[i+1:]
		body = body[:i]
	}
	body = strings.TrimSuffix(body, "/")

	var method, password, host string
	var port int
	var err error
	if at := strings.LastIndex(body, "@"); at >= 0 {
		userinfo := body[:at]
		host, port, err = splitHostPort(body[at+1:])
		if err != nil {
			return nil, err
		}
		if dec, derr := decodeB64(userinfo); derr == nil && strings.Contains(string(dec), ":") {
			method, password = splitCreds(string(dec))
		} else {
			ui := userinfo
			if d, uerr := url.PathUnescape(ui); uerr == nil {
				ui = d
			}
			method, password = splitCreds(ui)
		}
	} else {
		dec, derr := decodeB64(body)
		if derr != nil {
			return nil, fmt.Errorf("ss: invalid payload")
		}
		s := string(dec)
		at := strings.LastIndex(s, "@")
		if at < 0 {
			return nil, fmt.Errorf("ss: malformed payload")
		}
		method, password = splitCreds(s[:at])
		host, port, err = splitHostPort(s[at+1:])
		if err != nil {
			return nil, err
		}
	}
	method = normalizeSSMethod(method)
	if method == "" || password == "" || host == "" {
		return nil, fmt.Errorf("ss: incomplete credentials")
	}
	e := newEntry(name, "ss", host, port)
	e.set("cipher", method)
	e.set("password", password)
	e.set("udp", true)
	if plugin := rawQueryValue(query, "plugin"); plugin != "" {
		applySSPlugin(e, plugin)
	}
	return e, nil
}

func parseSSR(link string) (*entry, error) {
	raw := strings.TrimPrefix(link, "ssr://")
	dec, err := decodeB64(raw)
	if err != nil {
		return nil, fmt.Errorf("ssr: invalid base64 payload")
	}
	s := string(dec)
	main, params := s, ""
	if i := strings.Index(s, "/?"); i >= 0 {
		main, params = s[:i], s[i+2:]
	} else if i := strings.Index(s, "?"); i >= 0 {
		main, params = s[:i], s[i+1:]
	}
	parts := strings.SplitN(main, ":", 6)
	if len(parts) != 6 {
		return nil, fmt.Errorf("ssr: malformed payload")
	}
	host := parts[0]
	port, err := strconv.Atoi(parts[1])
	if err != nil || port <= 0 {
		return nil, fmt.Errorf("ssr: bad port")
	}
	protocol, method, obfs := parts[2], parts[3], parts[4]
	passRaw, err := decodeB64(parts[5])
	if err != nil {
		return nil, fmt.Errorf("ssr: bad password")
	}
	name := ""
	if remarks, ok := decodeB64(rawQueryValue(params, "remarks")); ok == nil {
		name = string(remarks)
	}
	e := newEntry(name, "ssr", host, port)
	e.set("cipher", normalizeSSMethod(method))
	e.set("password", string(passRaw))
	e.set("protocol", protocol)
	e.set("obfs", obfs)
	e.set("udp", true)
	if v, err := decodeB64(rawQueryValue(params, "protoparam")); err == nil && len(v) > 0 {
		e.set("protocol-param", string(v))
	}
	if v, err := decodeB64(rawQueryValue(params, "obfsparam")); err == nil && len(v) > 0 {
		e.set("obfs-param", string(v))
	}
	return e, nil
}

func parseHysteria2(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("hysteria2: %w", err)
	}
	auth := authString(u)
	if auth == "" {
		return nil, fmt.Errorf("hysteria2: missing password")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "hysteria2", host, port)
	e.set("password", auth)
	e.set("sni", q.Get("sni"))
	e.set("obfs", q.Get("obfs"))
	e.set("obfs-password", q.Get("obfs-password"))
	e.set("up", q.Get("up"))
	e.set("down", q.Get("down"))
	if truthy(q.Get("insecure")) || truthy(q.Get("allowInsecure")) || truthy(q.Get("allow_insecure")) {
		e.set("skip-cert-verify", true)
	}
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	return e, nil
}

func parseHysteria(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("hysteria: %w", err)
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "hysteria", host, port)
	e.set("auth-str", firstNonEmpty(q.Get("auth"), q.Get("auth_str")))
	e.set("protocol", firstNonEmpty(q.Get("protocol"), "udp"))
	e.set("sni", firstNonEmpty(q.Get("peer"), q.Get("sni")))
	e.set("up", q.Get("upmbps"))
	e.set("down", q.Get("downmbps"))
	if truthy(q.Get("insecure")) {
		e.set("skip-cert-verify", true)
	}
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	return e, nil
}

func parseTUIC(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("tuic: %w", err)
	}
	uuid := username(u)
	password, _ := u.User.Password()
	if uuid == "" || password == "" {
		return nil, fmt.Errorf("tuic: missing uuid or password")
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "tuic", host, port)
	e.set("uuid", uuid)
	e.set("password", password)
	e.set("congestion-controller", firstNonEmpty(q.Get("congestion_control"), q.Get("congestion-controller")))
	e.set("udp-relay-mode", firstNonEmpty(q.Get("udp_relay_mode"), q.Get("udp-relay-mode")))
	e.set("sni", q.Get("sni"))
	if truthy(q.Get("allow_insecure")) || truthy(q.Get("allowInsecure")) {
		e.set("skip-cert-verify", true)
	}
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	return e, nil
}

func parseAnyTLS(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("anytls: %w", err)
	}
	password := username(u)
	if password == "" {
		return nil, fmt.Errorf("anytls: missing password")
	}
	host := u.Hostname()
	if host == "" {
		return nil, fmt.Errorf("anytls: missing server")
	}
	// AnyTLS shares the Hysteria2 URI style: the port defaults to 443.
	port := 443
	if ps := u.Port(); ps != "" {
		p, perr := strconv.Atoi(ps)
		if perr != nil || p <= 0 || p > 65535 {
			return nil, fmt.Errorf("anytls: bad port %q", ps)
		}
		port = p
	}
	q := u.Query()
	e := newEntry(fragmentName(u), "anytls", host, port)
	e.set("password", password)
	e.set("sni", q.Get("sni"))
	e.set("udp", true)
	if truthy(q.Get("insecure")) {
		e.set("skip-cert-verify", true)
	}
	e.set("alpn", alpnFromQuery(q.Get("alpn")))
	e.set("client-fingerprint", q.Get("fp"))
	return e, nil
}

func parseSocks(link string) (*entry, error) {
	u, err := url.Parse(link)
	if err != nil {
		return nil, fmt.Errorf("socks5: %w", err)
	}
	host, port, err := hostPort(u)
	if err != nil {
		return nil, err
	}
	e := newEntry(fragmentName(u), "socks5", host, port)
	if u.User != nil {
		e.set("username", u.User.Username())
		if p, ok := u.User.Password(); ok {
			e.set("password", p)
		}
	}
	e.set("udp", true)
	return e, nil
}

// ---- helpers ----

func applyNetwork(e *entry, network, path, host, serviceName string) {
	switch strings.ToLower(network) {
	case "", "tcp":
		return
	case "ws", "websocket":
		e.set("network", "ws")
		opts := &entry{}
		opts.set("path", path)
		if host != "" {
			h := &entry{}
			h.set("Host", host)
			opts.set("headers", h)
		}
		if len(opts.keys) > 0 {
			e.set("ws-opts", opts)
		}
	case "grpc":
		e.set("network", "grpc")
		svc := firstNonEmpty(serviceName, path, host)
		if svc != "" {
			o := &entry{}
			o.set("grpc-service-name", svc)
			e.set("grpc-opts", o)
		}
	case "h2", "http", "http2":
		e.set("network", "h2")
		o := &entry{}
		o.set("path", path)
		if host != "" {
			o.set("host", []string{host})
		}
		if len(o.keys) > 0 {
			e.set("h2-opts", o)
		}
	}
}

func applySSPlugin(e *entry, plugin string) {
	parts := strings.Split(plugin, ";")
	base := parts[0]
	kv := map[string]string{}
	for _, p := range parts[1:] {
		if k, v, ok := strings.Cut(p, "="); ok {
			kv[strings.TrimSpace(k)] = strings.TrimSpace(v)
		} else if p != "" {
			kv[strings.TrimSpace(p)] = "true"
		}
	}
	switch base {
	case "obfs-local", "simple-obfs", "obfs":
		e.set("plugin", "obfs")
		o := &entry{}
		o.set("mode", firstNonEmpty(kv["obfs"], kv["mode"]))
		o.set("host", firstNonEmpty(kv["obfs-host"], kv["host"]))
		if len(o.keys) > 0 {
			e.set("plugin-opts", o)
		}
	case "v2ray-plugin":
		e.set("plugin", "v2ray-plugin")
		o := &entry{}
		o.set("mode", firstNonEmpty(kv["mode"], "websocket"))
		o.set("host", kv["host"])
		o.set("path", kv["path"])
		if _, ok := kv["tls"]; ok {
			o.set("tls", true)
		}
		e.set("plugin-opts", o)
	}
}

func normalizeSSMethod(method string) string {
	m := strings.ToLower(strings.TrimSpace(method))
	switch m {
	case "chacha20-poly1305", "chacha20":
		return "chacha20-ietf-poly1305"
	case "aes-256-cfb":
		return "aes-256-cfb"
	}
	return m
}

func newEntry(name, typ, server string, port int) *entry {
	e := &entry{}
	e.set("name", name)
	e.set("type", typ)
	e.set("server", server)
	if port > 0 {
		e.set("port", port)
	}
	return e
}

func username(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	return u.User.Username()
}

func authString(u *url.URL) string {
	if u.User == nil {
		return ""
	}
	if p, ok := u.User.Password(); ok {
		return u.User.Username() + ":" + p
	}
	return u.User.Username()
}

func hostPort(u *url.URL) (string, int, error) {
	host := u.Hostname()
	if host == "" {
		return "", 0, fmt.Errorf("%s: missing host", u.Scheme)
	}
	portStr := u.Port()
	if portStr == "" {
		switch strings.ToLower(u.Scheme) {
		case "hysteria", "hysteria2", "hy2", "tuic":
			return "", 0, fmt.Errorf("%s: missing port", u.Scheme)
		default:
			return "", 0, fmt.Errorf("%s: missing port", u.Scheme)
		}
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("%s: bad port %q", u.Scheme, portStr)
	}
	return host, port, nil
}

func splitHostPort(s string) (string, int, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", 0, fmt.Errorf("bad host:port %q", s)
		}
		host := s[1:end]
		rest := strings.TrimPrefix(s[end+1:], ":")
		port, err := strconv.Atoi(rest)
		if err != nil || port <= 0 {
			return "", 0, fmt.Errorf("bad host:port %q", s)
		}
		return host, port, nil
	}
	i := strings.LastIndex(s, ":")
	if i < 0 {
		return "", 0, fmt.Errorf("bad host:port %q", s)
	}
	port, err := strconv.Atoi(s[i+1:])
	if err != nil || port <= 0 || port > 65535 {
		return "", 0, fmt.Errorf("bad host:port %q", s)
	}
	return s[:i], port, nil
}

func splitCreds(s string) (string, string) {
	if i := strings.Index(s, ":"); i >= 0 {
		return s[:i], s[i+1:]
	}
	return s, ""
}

func cutFragment(link string) (string, string) {
	i := strings.Index(link, "#")
	if i < 0 {
		return link, ""
	}
	name := link[i+1:]
	if d, err := url.PathUnescape(name); err == nil {
		name = d
	}
	return link[:i], name
}

func fragmentName(u *url.URL) string {
	f := u.Fragment
	if f == "" {
		return ""
	}
	f = strings.TrimSpace(f)
	if d, err := url.PathUnescape(f); err == nil {
		f = d
	}
	return f
}

func decodeB64(s string) ([]byte, error) {
	s = strings.Join(strings.Fields(s), "")
	if s == "" {
		return nil, fmt.Errorf("empty base64")
	}
	s = strings.ReplaceAll(s, "-", "+")
	s = strings.ReplaceAll(s, "_", "/")
	if rem := len(s) % 4; rem != 0 {
		s += strings.Repeat("=", 4-rem)
	}
	return base64.StdEncoding.DecodeString(s)
}

func anyInt(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case int:
		return t
	case string:
		n, _ := strconv.Atoi(strings.TrimSpace(t))
		return n
	}
	return 0
}

func anyString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func truthy(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func alpnFromAny(v any) []string {
	switch t := v.(type) {
	case string:
		return alpnFromQuery(t)
	case []any:
		var out []string
		for _, x := range t {
			if s, ok := x.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		if len(out) == 0 {
			return nil
		}
		return out
	}
	return nil
}

func alpnFromQuery(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// rawQueryValue reads key from a raw query string without url.ParseQuery,
// which rejects the semicolons found in ss plugin parameters.
func rawQueryValue(raw, key string) string {
	for _, part := range strings.Split(raw, "&") {
		k, v, ok := strings.Cut(part, "=")
		if !ok || k != key {
			continue
		}
		if d, err := url.QueryUnescape(v); err == nil {
			return d
		}
		return v
	}
	return ""
}

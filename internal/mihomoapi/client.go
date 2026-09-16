package mihomoapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client talks to the mihomo external controller REST API.
type Client struct {
	Base   string
	Secret string
	HTTP   *http.Client
}

func New(port int, secret string) *Client {
	return &Client{
		Base:   fmt.Sprintf("http://127.0.0.1:%d", port),
		Secret: secret,
		HTTP:   &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) do(ctx context.Context, method, path string, body any, out any) error {
	var rdr io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Base+path, rdr)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Secret != "" {
		req.Header.Set("Authorization", "Bearer "+c.Secret)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		msg := strings.TrimSpace(string(data))
		if msg == "" {
			msg = resp.Status
		}
		return fmt.Errorf("%s %s: %s", method, path, msg)
	}
	if out != nil && len(data) > 0 {
		return json.Unmarshal(data, out)
	}
	return nil
}

func (c *Client) Version(ctx context.Context) (string, error) {
	var v struct {
		Version string `json:"version"`
		Meta    bool   `json:"meta"`
	}
	if err := c.do(ctx, http.MethodGet, "/version", nil, &v); err != nil {
		return "", err
	}
	if v.Version == "" {
		return "", fmt.Errorf("empty version response")
	}
	return v.Version, nil
}

type Configs struct {
	Port            int    `json:"port"`
	SocksPort       int    `json:"socks-port"`
	MixedPort       int    `json:"mixed-port"`
	Mode            string `json:"mode"`
	LogLevel        string `json:"log-level"`
	AllowLan        bool   `json:"allow-lan"`
	ExternalControl string `json:"external-controller"`
}

func (c *Client) Configs(ctx context.Context) (*Configs, error) {
	cfg := &Configs{}
	if err := c.do(ctx, http.MethodGet, "/configs", nil, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Client) PatchConfigs(ctx context.Context, patch map[string]any) error {
	return c.do(ctx, http.MethodPatch, "/configs", patch, nil)
}

// Reload hot-reloads the running core from an absolute config file path.
func (c *Client) Reload(ctx context.Context, path string) error {
	return c.do(ctx, http.MethodPut, "/configs?force=true", map[string]string{"path": path}, nil)
}

type Proxies struct {
	Proxies map[string]Proxy `json:"proxies"`
}

type Proxy struct {
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Now     string   `json:"now,omitempty"`
	All     []string `json:"all,omitempty"`
	History []struct {
		Delay int `json:"delay"`
	} `json:"history,omitempty"`
}

func (c *Client) GetProxies(ctx context.Context) (*Proxies, error) {
	ps := &Proxies{}
	if err := c.do(ctx, http.MethodGet, "/proxies", nil, ps); err != nil {
		return nil, err
	}
	return ps, nil
}

func (c *Client) GetProxy(ctx context.Context, name string) (*Proxy, error) {
	p := &Proxy{}
	if err := c.do(ctx, http.MethodGet, "/proxies/"+url.PathEscape(name), nil, p); err != nil {
		return nil, err
	}
	return p, nil
}

// Select picks node inside a select-type proxy group.
func (c *Client) Select(ctx context.Context, group, node string) error {
	return c.do(ctx, http.MethodPut, "/proxies/"+url.PathEscape(group), map[string]string{"name": node}, nil)
}

// Delay runs a latency test for a proxy or group, returning milliseconds.
func (c *Client) Delay(ctx context.Context, name, testURL string, timeoutMS int) (int, error) {
	q := url.Values{}
	if testURL != "" {
		q.Set("url", testURL)
	}
	if timeoutMS > 0 {
		q.Set("timeout", fmt.Sprintf("%d", timeoutMS))
	}
	var out struct {
		Delay int `json:"delay"`
	}
	path := "/proxies/" + url.PathEscape(name) + "/delay"
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	if err := c.do(ctx, http.MethodGet, path, nil, &out); err != nil {
		return 0, err
	}
	return out.Delay, nil
}

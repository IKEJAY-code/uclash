package shellenv

import (
	"fmt"
	"strings"
)

const NoProxy = "localhost,127.0.0.1,::1,10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,169.254.0.0/16,.local"

var proxyVars = []string{"http_proxy", "https_proxy", "all_proxy", "HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY"}

func ProxyURL(port int) string { return fmt.Sprintf("http://127.0.0.1:%d", port) }

// ExportOn renders shell statements enabling the terminal proxy.
func ExportOn(port int) string {
	u := ProxyURL(port)
	var b strings.Builder
	for _, k := range proxyVars {
		fmt.Fprintf(&b, "export %s=%q\n", k, u)
	}
	for _, k := range []string{"no_proxy", "NO_PROXY"} {
		fmt.Fprintf(&b, "export %s=%q\n", k, NoProxy)
	}
	return b.String()
}

// ExportOff renders shell statements disabling only the values that point at
// our own proxy port, so a pre-existing unrelated proxy setup is not clobbered.
func ExportOff(port int) string {
	u := ProxyURL(port)
	var b strings.Builder
	for _, k := range proxyVars {
		fmt.Fprintf(&b, "[ \"${%s:-}\" = %q ] && unset %s\n", k, u, k)
	}
	for _, k := range []string{"no_proxy", "NO_PROXY"} {
		fmt.Fprintf(&b, "[ \"${%s:-}\" = %q ] && unset %s\n", k, NoProxy, k)
	}
	return b.String()
}

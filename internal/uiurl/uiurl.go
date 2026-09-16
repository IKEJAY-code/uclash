package uiurl

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
)

func Serve(port int) string {
	return fmt.Sprintf("http://127.0.0.1:%d/ui/", port)
}

// Setup builds a dashboard URL that prefills the controller address + secret.
func Setup(port int, secret string) string {
	q := url.Values{}
	q.Set("hostname", "127.0.0.1")
	q.Set("port", strconv.Itoa(port))
	if secret != "" {
		q.Set("secret", secret)
	}
	return Serve(port) + "#/setup?" + q.Encode()
}

func Hyperlink(u, label string) string {
	return "\x1b]8;;" + u + "\x1b\\" + label + "\x1b]8;;\x1b\\"
}

func IsTerminal(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

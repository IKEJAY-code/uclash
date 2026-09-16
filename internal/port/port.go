package port

import (
	"fmt"
	"net"
)

// Free asks the kernel for an unused TCP port on host.
func Free(host string) (int, error) {
	l, err := net.Listen("tcp", net.JoinHostPort(host, "0"))
	if err != nil {
		return 0, err
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// IsFree reports whether host:port can be bound right now.
func IsFree(host string, p int) bool {
	if p <= 0 || p > 65535 {
		return false
	}
	l, err := net.Listen("tcp", net.JoinHostPort(host, fmt.Sprintf("%d", p)))
	if err != nil {
		return false
	}
	_ = l.Close()
	return true
}

// Ensure returns a currently free port, preferring want when available.
// The boolean result reports whether the preferred port was kept.
func Ensure(host string, want int) (int, bool, error) {
	if IsFree(host, want) {
		return want, true, nil
	}
	p, err := Free(host)
	if err != nil {
		return 0, false, err
	}
	return p, false, nil
}

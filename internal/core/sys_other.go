//go:build !linux

package core

import (
	"errors"
	"time"
)

var errUnsupported = errors.New("uclash core management currently supports Linux only (build with GOOS=linux)")

func spawnDetached(bin string, args []string, logFile string, env []string) (int, error) {
	return 0, errUnsupported
}

func isAlive(pid int) bool { return false }

func processMatches(pid int, corePath, configPath string) bool { return false }

func terminate(pid int, timeout time.Duration) error { return errUnsupported }

func lockFile(path string) (func(), error) { return func() {}, nil }

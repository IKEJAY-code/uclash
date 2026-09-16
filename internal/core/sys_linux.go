//go:build linux

package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// spawnDetached starts bin with args in a new session so it survives the
// caller's terminal/session and is reparented to init.
func spawnDetached(bin string, args []string, logFile string, env []string) (int, error) {
	if err := os.MkdirAll(filepath.Dir(logFile), 0o755); err != nil {
		return 0, err
	}
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	cmd := exec.Command(bin, args...)
	cmd.Stdout = f
	cmd.Stderr = f
	cmd.Stdin = nil
	cmd.Env = env
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("spawn %s: %w", bin, err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	return pid, nil
}

func isAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	if err := syscall.Kill(pid, 0); err != nil {
		return false
	}
	// Filter out zombies: they can still be signalled but are already dead.
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if err != nil {
		return true
	}
	fields := strings.Fields(string(data))
	if len(fields) > 2 && fields[2] == "Z" {
		return false
	}
	return true
}

// processMatches verifies the pid belongs to our mihomo instance by checking
// /proc/<pid>/cmdline. This guarantees `uclash stop` can never kill another
// user's process even if a pid file was copied or recycled.
func processMatches(pid int, corePath, configPath string) bool {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/cmdline", pid))
	if err != nil {
		return false
	}
	args := strings.Split(strings.TrimRight(string(data), "\x00"), "\x00")
	hasCore, hasConfig := false, false
	for _, a := range args {
		if a == corePath || filepath.Base(a) == filepath.Base(corePath) {
			hasCore = true
		}
		if a == configPath {
			hasConfig = true
		}
	}
	return hasCore && hasConfig
}

func terminate(pid int, timeout time.Duration) error {
	if !isAlive(pid) {
		return nil
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !isAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return fmt.Errorf("kill %d: %w", pid, err)
	}
	for i := 0; i < 30; i++ {
		if !isAlive(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("process %d did not exit", pid)
}

// lockFile takes an exclusive advisory lock held until the returned function
// runs. The lock disappears with the process, so crashes cannot wedge it.
func lockFile(path string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}

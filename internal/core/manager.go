package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/mihomoapi"
)

var (
	ErrAlreadyRunning = errors.New("mihomo is already running")
	ErrNotRunning     = errors.New("mihomo is not running")
)

type PIDInfo struct {
	PID       int       `json:"pid"`
	StartedAt time.Time `json:"started_at"`
	Core      string    `json:"core"`
	Config    string    `json:"config"`
	DataDir   string    `json:"data_dir"`
}

type Manager struct {
	DataDir    string
	CorePath   string
	ConfigPath string
	LogFile    string
	PIDFile    string
	LockFile   string
	APIPort    int
	Secret     string
}

func NewManager(dataDir, corePath, configPath, logFile, pidFile, lockFile string) *Manager {
	return &Manager{
		DataDir:    dataDir,
		CorePath:   corePath,
		ConfigPath: configPath,
		LogFile:    logFile,
		PIDFile:    pidFile,
		LockFile:   lockFile,
	}
}

func (m *Manager) readPID() (*PIDInfo, error) {
	data, err := os.ReadFile(m.PIDFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	pi := &PIDInfo{}
	if err := json.Unmarshal(data, pi); err != nil {
		return nil, nil
	}
	if pi.PID <= 0 {
		return nil, nil
	}
	return pi, nil
}

func (m *Manager) writePID(pi *PIDInfo) error {
	data, err := json.MarshalIndent(pi, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.PIDFile, data, 0o600)
}

func (m *Manager) removePID() {
	_ = os.Remove(m.PIDFile)
}

// Running returns info about the managed instance if it is alive and is
// verifiably our own process (never another user's or an unrelated process).
func (m *Manager) Running() (*PIDInfo, bool) {
	pi, err := m.readPID()
	if err != nil || pi == nil {
		return nil, false
	}
	if !isAlive(pi.PID) {
		return nil, false
	}
	if !processMatches(pi.PID, m.CorePath, m.ConfigPath) {
		return nil, false
	}
	return pi, true
}

// Start spawns mihomo detached, waits for its API to come up, and records the pid.
func (m *Manager) Start(ctx context.Context) (*PIDInfo, error) {
	if pi, ok := m.Running(); ok {
		return pi, ErrAlreadyRunning
	}
	release, err := lockFile(m.LockFile)
	if err != nil {
		return nil, fmt.Errorf("lock %s: %w", m.LockFile, err)
	}
	defer release()
	// Re-check after acquiring the lock (another process may have won the race).
	if pi, ok := m.Running(); ok {
		return pi, ErrAlreadyRunning
	}
	m.removePID()

	if _, err := os.Stat(m.CorePath); err != nil {
		return nil, fmt.Errorf("mihomo core not found at %s (run `uclash init`)", m.CorePath)
	}
	if _, err := os.Stat(m.ConfigPath); err != nil {
		return nil, fmt.Errorf("no active configuration (run `uclash sub add <url>` or `uclash profile import <file.yaml>`)")
	}

	pid, err := spawnDetached(m.CorePath, []string{"-d", m.DataDir, "-f", m.ConfigPath}, m.LogFile, os.Environ())
	if err != nil {
		return nil, err
	}
	pi := &PIDInfo{
		PID:       pid,
		StartedAt: time.Now(),
		Core:      m.CorePath,
		Config:    m.ConfigPath,
		DataDir:   m.DataDir,
	}
	if err := m.writePID(pi); err != nil {
		return pi, err
	}
	if err := m.waitReady(ctx, 10*time.Second); err != nil {
		_ = terminate(pid, 3*time.Second)
		m.removePID()
		return nil, fmt.Errorf("%w\n--- last log lines ---\n%s", err, TailLog(m.LogFile, 20))
	}
	return pi, nil
}

func (m *Manager) waitReady(ctx context.Context, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := mihomoapi.New(m.APIPort, m.Secret)
	for time.Now().Before(deadline) {
		if !isAlive(mustPID(m.PIDFile)) {
			return fmt.Errorf("mihomo exited during startup")
		}
		if _, err := client.Version(ctx); err == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("mihomo API did not become ready within %s", timeout)
}

func mustPID(pidFile string) int {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return -1
	}
	pi := &PIDInfo{}
	if err := json.Unmarshal(data, pi); err != nil {
		return -1
	}
	return pi.PID
}

// Stop terminates only this user's instance.
func (m *Manager) Stop(ctx context.Context) error {
	pi, err := m.readPID()
	if err != nil {
		return err
	}
	if pi == nil {
		return ErrNotRunning
	}
	if !isAlive(pi.PID) {
		m.removePID()
		return ErrNotRunning
	}
	if !processMatches(pi.PID, m.CorePath, m.ConfigPath) {
		m.removePID()
		return fmt.Errorf("refusing to kill pid %d: it does not look like this uclash instance (stale pid file removed)", pi.PID)
	}
	if err := terminate(pi.PID, 10*time.Second); err != nil {
		return err
	}
	m.removePID()
	return nil
}

// TailLog returns the last n lines of the core log.
func TailLog(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

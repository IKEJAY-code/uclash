package shellenv

import (
	"os"
	"path/filepath"
	"strings"

	"gitee.com/IKEJAY-code/uclash/internal/paths"
)

const (
	beginMarker = "# >>> uclash initialize >>>"
	endMarker   = "# <<< uclash initialize <<<"
)

// DetectRC guesses the interactive rc file to modify.
func DetectRC() string {
	home := paths.ExpandHome("~")
	sh := strings.ToLower(os.Getenv("SHELL"))
	switch {
	case strings.Contains(sh, "zsh"):
		return filepath.Join(home, ".zshrc")
	case strings.Contains(sh, "bash"):
		return filepath.Join(home, ".bashrc")
	}
	if _, err := os.Stat(filepath.Join(home, ".bashrc")); err == nil {
		return filepath.Join(home, ".bashrc")
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); err == nil {
		return filepath.Join(home, ".zshrc")
	}
	return ""
}

// Block is the shell snippet appended to the user's rc file.
func Block() string {
	return beginMarker + `
# Managed by uclash. Remove with: uclash shell uninstall
proxyon() {
  command -v uclash >/dev/null 2>&1 || { echo "uclash: command not found in PATH" >&2; return 1; }
  uclash start --quiet >/dev/null 2>&1 || true
  eval "$(uclash env on)" && echo "uclash: terminal proxy enabled"
}
proxyoff() {
  eval "$(uclash env off)"
  echo "uclash: terminal proxy disabled (core keeps running; use 'uclash stop' to stop it)"
}
uclash-ui() { uclash ui "$@"; }
` + endMarker + "\n"
}

// Install adds or refreshes the managed block in rcFile.
func Install(rcFile string) (bool, error) {
	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	cleaned := removeBlock(string(content))
	var next string
	if strings.TrimSpace(cleaned) == "" {
		next = Block()
	} else {
		next = strings.TrimRight(cleaned, "\n") + "\n\n" + Block()
	}
	if next == string(content) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(rcFile), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(rcFile, []byte(next), 0o644)
}

// Uninstall removes the managed block from rcFile.
func Uninstall(rcFile string) (bool, error) {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	cleaned := removeBlock(string(content))
	if cleaned == string(content) {
		return false, nil
	}
	return true, os.WriteFile(rcFile, []byte(cleaned), 0o644)
}

// Installed reports whether the managed block exists in rcFile.
func Installed(rcFile string) bool {
	content, err := os.ReadFile(rcFile)
	if err != nil {
		return false
	}
	return strings.Contains(string(content), beginMarker)
}

func removeBlock(content string) string {
	for {
		start := strings.Index(content, beginMarker)
		if start < 0 {
			return content
		}
		end := strings.Index(content[start:], endMarker)
		if end < 0 {
			return content
		}
		end += start + len(endMarker)
		for end < len(content) && (content[end] == '\n' || content[end] == '\r') {
			end++
		}
		content = content[:start] + content[end:]
	}
}

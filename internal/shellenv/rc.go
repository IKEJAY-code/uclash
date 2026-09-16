package shellenv

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitee.com/IKEJAY-code/uclash/internal/paths"
)

const (
	beginMarker = "# >>> uclash initialize >>>"
	endMarker   = "# <<< uclash initialize <<<"
)

// DetectShell guesses the interactive shell that launched uclash.
// /proc/<ppid>/exe wins over $SHELL because users often run a custom zsh that
// $SHELL does not describe.
func DetectShell() string {
	if exe, err := os.Readlink("/proc/" + strconv.Itoa(os.Getppid()) + "/exe"); err == nil {
		base := strings.TrimPrefix(filepath.Base(exe), "-")
		switch base {
		case "bash", "zsh", "fish":
			return base
		}
	}
	sh := strings.ToLower(filepath.Base(os.Getenv("SHELL")))
	switch {
	case strings.Contains(sh, "zsh"):
		return "zsh"
	case strings.Contains(sh, "fish"):
		return "fish"
	case strings.Contains(sh, "bash"):
		return "bash"
	}
	return ""
}

// RCForShell returns the interactive rc file for a given shell, honouring
// ZDOTDIR for zsh and XDG_CONFIG_HOME for fish.
func RCForShell(shell string) string {
	home := paths.ExpandHome("~")
	switch shell {
	case "zsh":
		dir := os.Getenv("ZDOTDIR")
		if dir == "" {
			dir = home
		}
		return filepath.Join(dir, ".zshrc")
	case "fish":
		cfg := os.Getenv("XDG_CONFIG_HOME")
		if cfg == "" {
			cfg = filepath.Join(home, ".config")
		}
		return filepath.Join(cfg, "fish", "config.fish")
	default:
		return filepath.Join(home, ".bashrc")
	}
}

// ResolveRC finds the rc file a shell will actually read, together with a
// human-readable explanation of how it was found.
//
// ZDOTDIR is often set in ~/.zshenv without being exported, so the child
// process cannot see it; asking zsh itself (zsh -c reads .zshenv) closes that
// gap. The same trick works for fish's __fish_config_dir.
func ResolveRC(shell string) (string, string) {
	switch shell {
	case "zsh":
		if dir := os.Getenv("ZDOTDIR"); dir != "" {
			return filepath.Join(dir, ".zshrc"), "ZDOTDIR from the environment"
		}
		if dir := probeZshConfigDir(); dir != "" {
			return filepath.Join(dir, ".zshrc"), "ZDOTDIR set in .zshenv (probed with zsh -c)"
		}
		return RCForShell("zsh"), "default location"
	case "fish":
		if dir := os.Getenv("__fish_config_dir"); dir != "" {
			return filepath.Join(dir, "config.fish"), "__fish_config_dir from the environment"
		}
		if dir := probeFishConfigDir(); dir != "" {
			return filepath.Join(dir, "config.fish"), "probed with fish -c"
		}
		return RCForShell("fish"), "default location"
	default:
		return RCForShell("bash"), "default location"
	}
}

// BashLoginRisk reports the file that a bash login shell would read when it
// does not source ~/.bashrc, so the caller can warn the user.
func BashLoginRisk() string {
	home := paths.ExpandHome("~")
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		path := filepath.Join(home, name)
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if strings.Contains(string(data), ".bashrc") {
			return "" // the login shell already pulls in .bashrc
		}
		return path
	}
	return ""
}

func probeZshConfigDir() string {
	zsh, err := exec.LookPath("zsh")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, zsh, "-c", `printf %s "${ZDOTDIR:-}"`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func probeFishConfigDir() string {
	fish, err := exec.LookPath("fish")
	if err != nil {
		return ""
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, fish, "-c", `printf %s "$__fish_config_dir"`).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// DetectRC returns the rc file for the detected shell. When the shell cannot
// be detected it falls back to whichever rc file exists.
func DetectRC() string {
	if sh := DetectShell(); sh != "" {
		return RCForShell(sh)
	}
	home := paths.ExpandHome("~")
	for _, candidate := range []string{".zshrc", ".bashrc"} {
		if _, err := os.Stat(filepath.Join(home, candidate)); err == nil {
			return filepath.Join(home, candidate)
		}
	}
	return ""
}

// Block is the bash/zsh snippet appended to the user's rc file. It defines a
// `uclash` wrapper that evaluates `uclash proxy on|off` in the current shell
// (a child process cannot change its parent's environment, so the shell has to
// cooperate exactly once) and forwards every other subcommand untouched.
func Block() string {
	return beginMarker + `
# Managed by uclash. Remove with: uclash shell uninstall
uclash() {
    if [ "$1" = "proxy" ] && { [ "$2" = "on" ] || [ "$2" = "off" ]; }; then
        _uclash_out="$(command uclash proxy "$2")" || return $?
        eval "$_uclash_out" || { unset _uclash_out; return $?; }
        unset _uclash_out
        if [ "$2" = "on" ]; then echo "uclash: proxy on"; else echo "uclash: proxy off (core keeps running; 'uclash stop' stops it)"; fi
        return 0
    fi
    command uclash "$@"
}
` + endMarker + "\n"
}

// FishBlock is the fish equivalent of Block.
func FishBlock() string {
	return beginMarker + `
# Managed by uclash. Remove with: uclash shell uninstall
function uclash
    if test "$argv[1]" = proxy; and test "$argv[2]" = on -o "$argv[2]" = off
        if test "$argv[2]" = on
            command uclash proxy on --fish | source; or return $status
            echo "uclash: proxy on"
        else
            command uclash proxy off --fish | source; or return $status
            echo "uclash: proxy off (core keeps running; 'uclash stop' stops it)"
        end
        return 0
    end
    command uclash $argv
end
` + endMarker + "\n"
}

// BlockForShell picks the snippet matching the shell.
func BlockForShell(shell string) string {
	if shell == "fish" {
		return FishBlock()
	}
	return Block()
}

// Install adds or refreshes the managed block in rcFile.
func Install(rcFile, shell string) (bool, error) {
	content, err := os.ReadFile(rcFile)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	cleaned := removeBlock(string(content))
	block := BlockForShell(shell)
	var next string
	if strings.TrimSpace(cleaned) == "" {
		next = block
	} else {
		next = strings.TrimRight(cleaned, "\n") + "\n\n" + block
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

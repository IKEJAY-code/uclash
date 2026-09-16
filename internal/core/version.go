package core

import (
	"os/exec"
	"regexp"
	"strings"
)

var versionRe = regexp.MustCompile(`v?(\d+\.\d+\.\d+)`)

// BinaryVersion extracts the semantic version from `mihomo -v` output.
// Returns "" when the binary is missing or unparsable.
func BinaryVersion(path string) string {
	out, err := exec.Command(path, "-v").CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	if m := versionRe.FindStringSubmatch(string(out)); m != nil {
		return m[1]
	}
	return ""
}

// NormalizeVersion strips a leading v from a release tag.
func NormalizeVersion(tag string) string {
	return strings.TrimPrefix(strings.TrimSpace(tag), "v")
}

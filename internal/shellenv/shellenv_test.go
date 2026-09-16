package shellenv

import (
	"strings"
	"testing"
)

func TestExportOnSetsProxyVarsAndNoProxy(t *testing.T) {
	out := ExportOn(12345)
	for _, want := range []string{
		`export http_proxy="http://127.0.0.1:12345"`,
		`export https_proxy="http://127.0.0.1:12345"`,
		`export HTTP_PROXY="http://127.0.0.1:12345"`,
		`export no_proxy="localhost,127.0.0.1`,
		`export NO_PROXY="localhost,127.0.0.1`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("ExportOn missing %q", want)
		}
	}
}

func TestExportOffOnlyUnsetsOurValue(t *testing.T) {
	out := ExportOff(12345)
	if !strings.Contains(out, `[ "${http_proxy:-}" = "http://127.0.0.1:12345" ] && unset http_proxy`) {
		t.Fatalf("ExportOff should guard the unset with a value check:\n%s", out)
	}
	if strings.Contains(out, "export ") {
		t.Error("ExportOff must not export anything")
	}
}

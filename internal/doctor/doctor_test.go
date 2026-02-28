package doctor

import (
	"strings"
	"testing"

	"github.com/boyand/codex-review-loop/internal/config"
)

func TestRun(t *testing.T) {
	cfg := config.Config{}
	var buf strings.Builder
	Run(&buf, cfg)
	out := buf.String()

	if !strings.Contains(out, "Codex Review Loop Doctor") {
		t.Error("missing header")
	}
	if !strings.Contains(out, "Overall:") {
		t.Error("missing overall result")
	}
	if !strings.Contains(out, "Go toolchain") {
		t.Error("missing Go check")
	}
}

func TestCheckGoToolchain(t *testing.T) {
	c := checkGoToolchain()
	if c.Result != "PASS" {
		t.Errorf("Go toolchain check = %q, want PASS", c.Result)
	}
	if !strings.Contains(c.Details, "go") {
		t.Errorf("expected go version in details, got %q", c.Details)
	}
}

package doctor

import (
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	var buf strings.Builder
	Run(&buf)
	out := buf.String()

	if !strings.Contains(out, "Codex Review Doctor") {
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

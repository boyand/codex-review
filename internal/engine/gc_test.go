package engine

import (
	"os"
	"strings"
	"testing"
)

func TestRunGC(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	var buf strings.Builder
	code, err := RunGC(&buf, 30)
	if err != nil {
		t.Fatalf("RunGC: %v", err)
	}
	if code != 0 {
		t.Fatalf("code=%d want 0", code)
	}
	if !strings.Contains(buf.String(), "Codex Review GC") {
		t.Fatalf("unexpected output: %s", buf.String())
	}
}

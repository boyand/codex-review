package app

import (
	"os"
	"testing"
)

func TestNew(t *testing.T) {
	a := New(os.Stdout, os.Stderr)
	if a.Config.CodexModel == "" {
		t.Error("expected non-empty default CodexModel")
	}
	if a.Stdout == nil {
		t.Error("Stdout should not be nil")
	}
}

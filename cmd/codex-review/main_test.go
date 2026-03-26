package main

import (
	"os"
	"testing"
)

func TestRunVersion(t *testing.T) {
	code := run([]string{"--version"})
	if code != 0 {
		t.Errorf("--version returned %d, want 0", code)
	}
}

func TestRunNoArgs(t *testing.T) {
	code := run(nil)
	if code != 1 {
		t.Errorf("no args returned %d, want 1", code)
	}
}

func TestRunUnknownCommand(t *testing.T) {
	code := run([]string{"bogus"})
	if code != 1 {
		t.Errorf("unknown command returned %d, want 1", code)
	}
}

func TestRunRemovedCommandsAreUnknown(t *testing.T) {
	tests := [][]string{
		{"cancel"},
		{"gc"},
	}
	for _, tt := range tests {
		if code := run(tt); code != 1 {
			t.Errorf("run(%v)=%d want 1", tt, code)
		}
	}
}

func TestRunInternalApproveBadArgs(t *testing.T) {
	if code := run([]string{"__approve", "a", "b"}); code != 1 {
		t.Errorf("run(__approve a b)=%d want 1", code)
	}
}

func TestRunInternalRepeatBadArgs(t *testing.T) {
	if code := run([]string{"__repeat", "a", "b"}); code != 1 {
		t.Errorf("run(__repeat a b)=%d want 1", code)
	}
}

func TestRunInternalDoneBadArgs(t *testing.T) {
	if code := run([]string{"__done", "a", "b"}); code != 1 {
		t.Errorf("run(__done a b)=%d want 1", code)
	}
}

func TestRunSummaryNoLoop(t *testing.T) {
	orig, _ := os.Getwd()
	dir := t.TempDir()
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	defer os.Chdir(orig)

	if code := run([]string{"summary"}); code != 0 {
		t.Errorf("run(summary)=%d want 0", code)
	}
}

func TestRunRequiresSessionIDValue(t *testing.T) {
	if code := run([]string{"--session-id"}); code != 1 {
		t.Errorf("run(--session-id)=%d want 1", code)
	}
}

func TestRunRejectsSessionIDWithoutSessionPID(t *testing.T) {
	if code := run([]string{"--session-id", "sess-hint", "plan", "review this"}); code != 1 {
		t.Errorf("run(--session-id sess-hint plan ...)=%d want 1", code)
	}
}

func TestRunRequiresValidSessionPIDValue(t *testing.T) {
	tests := [][]string{
		{"--session-pid"},
		{"--session-pid", "0"},
		{"--session-pid", "abc"},
	}
	for _, tt := range tests {
		if code := run(tt); code != 1 {
			t.Errorf("run(%v)=%d want 1", tt, code)
		}
	}
}

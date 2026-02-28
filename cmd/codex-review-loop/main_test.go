package main

import "testing"

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

func TestRunHookMissingStop(t *testing.T) {
	code := run([]string{"hook"})
	if code != 1 {
		t.Errorf("hook without stop returned %d, want 1", code)
	}
}

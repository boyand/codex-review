package fsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	data := []byte("hello world")
	if err := AtomicWrite(path, data, 0644); err != nil {
		t.Fatalf("AtomicWrite: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("got %q, want %q", got, "hello world")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if info.Mode().Perm() != 0644 {
		t.Errorf("perm = %v, want 0644", info.Mode().Perm())
	}
}

func TestAtomicWriteOverwrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.txt")

	if err := AtomicWrite(path, []byte("first"), 0644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if err := AtomicWrite(path, []byte("second"), 0644); err != nil {
		t.Fatalf("second write: %v", err)
	}

	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Errorf("got %q, want %q", got, "second")
	}
}

func TestAtomicWriteBadDir(t *testing.T) {
	err := AtomicWrite("/nonexistent/dir/file.txt", []byte("x"), 0644)
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

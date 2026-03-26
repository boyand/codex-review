package lock

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestAcquireRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test.lock")
	l := New(dir)

	if err := l.Acquire(); err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !l.Held() {
		t.Error("expected Held() = true")
	}

	// PID file should exist
	data, err := os.ReadFile(filepath.Join(dir, "pid"))
	if err != nil {
		t.Fatalf("read pid: %v", err)
	}
	pid, _ := strconv.Atoi(string(data))
	if pid != os.Getpid() {
		t.Errorf("pid = %d, want %d", pid, os.Getpid())
	}

	l.Release()
	if l.Held() {
		t.Error("expected Held() = false after release")
	}

	// Dir should be removed
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Error("lock dir should not exist after release")
	}
}

func TestAcquireConflictStaleLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test.lock")

	// Simulate a stale lock with a dead PID
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "pid"), []byte("99999999"), 0644)

	l := New(dir)
	if err := l.Acquire(); err != nil {
		t.Fatalf("Acquire with stale lock should succeed: %v", err)
	}
	l.Release()
}

func TestAcquireConflictLiveLock(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test.lock")

	// Simulate a live lock with our own PID
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "pid"), []byte(strconv.Itoa(os.Getpid())), 0644)

	l := New(dir)
	err := l.Acquire()
	if err == nil {
		l.Release()
		t.Fatal("Acquire with live lock should fail")
	}
	var held *HeldError
	if !errors.As(err, &held) {
		t.Fatalf("expected HeldError, got %T", err)
	}
	if held.OwnerPID != os.Getpid() {
		t.Fatalf("OwnerPID=%d want %d", held.OwnerPID, os.Getpid())
	}
	if held.LockDir != dir {
		t.Fatalf("LockDir=%q want %q", held.LockDir, dir)
	}
}

func TestDoubleRelease(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test.lock")
	l := New(dir)
	l.Acquire()
	l.Release()
	l.Release() // should not panic
}

// Package lock provides a mkdir-based lock with PID file and stale detection.
package lock

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// HeldError reports a live lock owned by another process.
type HeldError struct {
	OwnerPID int
	LockDir  string
}

func (e *HeldError) Error() string {
	return fmt.Sprintf("another codex-review command is already running (pid %d)", e.OwnerPID)
}

// Lock represents a directory-based lock.
type Lock struct {
	dir  string
	held bool
	sigs chan os.Signal
}

// New creates a lock at the given directory path.
func New(dir string) *Lock {
	return &Lock{dir: dir}
}

// Acquire attempts to acquire the lock. Returns an error describing
// the conflict if another process holds it.
func (l *Lock) Acquire() error {
	// Ensure parent dir exists
	if err := os.MkdirAll(filepath.Dir(l.dir), 0755); err != nil {
		return fmt.Errorf("lock: mkdir parent: %w", err)
	}

	err := os.Mkdir(l.dir, 0755)
	if err == nil {
		l.held = true
		l.writePID()
		l.installSignalHandler()
		return nil
	}

	if !os.IsExist(err) {
		return fmt.Errorf("lock: mkdir: %w", err)
	}

	// Lock dir exists — check if owner is still alive
	ownerPID := l.readOwnerPID()
	if ownerPID > 0 && processAlive(ownerPID) {
		return &HeldError{OwnerPID: ownerPID, LockDir: l.dir}
	}

	// Stale lock — reclaim
	os.RemoveAll(l.dir)
	if err := os.Mkdir(l.dir, 0755); err != nil {
		return fmt.Errorf("lock: reclaim: %w", err)
	}
	l.held = true
	l.writePID()
	l.installSignalHandler()
	return nil
}

// Release releases the lock if held.
func (l *Lock) Release() {
	if !l.held {
		return
	}
	os.RemoveAll(l.dir)
	l.held = false
	if l.sigs != nil {
		signal.Stop(l.sigs)
	}
}

// Held returns whether the lock is currently held.
func (l *Lock) Held() bool {
	return l.held
}

func (l *Lock) writePID() {
	pidFile := filepath.Join(l.dir, "pid")
	os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0644)
}

func (l *Lock) readOwnerPID() int {
	data, err := os.ReadFile(filepath.Join(l.dir, "pid"))
	if err != nil {
		return 0
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0
	}
	return pid
}

func (l *Lock) installSignalHandler() {
	l.sigs = make(chan os.Signal, 1)
	signal.Notify(l.sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		<-l.sigs
		l.Release()
		os.Exit(130)
	}()
}

func processAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil
}

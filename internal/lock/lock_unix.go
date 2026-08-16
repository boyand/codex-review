//go:build !windows

package lock

import "syscall"

// processAlive reports whether pid refers to a live process. Signal 0 performs
// an error-check without delivering a signal: nil means the process exists and
// is signalable, EPERM means it exists but is owned by another user (so it is
// alive — treating it as dead would let us steal a live lock), any other error
// (ESRCH) means it is gone.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

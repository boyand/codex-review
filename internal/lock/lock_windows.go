//go:build windows

package lock

import "syscall"

const (
	// Constants not exported by the syscall package.
	processQueryLimitedInformation = 0x1000 // PROCESS_QUERY_LIMITED_INFORMATION
	stillActive                    = 259    // STILL_ACTIVE
)

// processAlive reports whether pid refers to a live process.
//
// Windows has no signal-0 liveness probe. os.FindProcess is not sufficient: a
// terminated process stays openable while any handle to it remains, so a
// successful open does not imply the process is running, and an access-denied
// error actually proves the process exists (e.g. an elevated owner). We open
// with PROCESS_QUERY_LIMITED_INFORMATION — which succeeds across elevation
// boundaries — and read the exit code to distinguish a live process from a
// still-openable corpse.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	if err != nil {
		// Access denied means the process exists but is protected; treating it
		// as dead would let us steal a live lock. Any other error (e.g. invalid
		// parameter for an unknown pid) means it is gone.
		return err == syscall.ERROR_ACCESS_DENIED
	}
	defer syscall.CloseHandle(h)
	var code uint32
	if err := syscall.GetExitCodeProcess(h, &code); err != nil {
		return true // we opened it; fail toward "held"
	}
	// A process that deliberately exits with 259 reads as alive; Microsoft
	// documents not using STILL_ACTIVE as an exit code, so this is acceptable.
	return code == stillActive
}

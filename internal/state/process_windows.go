//go:build windows

package state

import (
	"fmt"

	"golang.org/x/sys/windows"
)

// ProcessIdentity returns the process creation timestamp used to distinguish a
// live process from a recycled PID on Windows.
func ProcessIdentity(pid int) (uint64, error) {
	handle, err := openProcess(pid)
	if err != nil {
		return 0, err
	}
	defer windows.CloseHandle(handle)

	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(handle, &created, &exited, &kernel, &user); err != nil {
		return 0, fmt.Errorf("read process %d creation time: %w", pid, err)
	}
	return uint64(created.HighDateTime)<<32 | uint64(created.LowDateTime), nil
}

// ProcessAlive reports whether a process with pid exists and has not exited.
func ProcessAlive(pid int) bool {
	handle, err := openProcess(pid)
	if err != nil {
		return false
	}
	defer windows.CloseHandle(handle)

	result, err := windows.WaitForSingleObject(handle, 0)
	return err == nil && result == uint32(windows.WAIT_TIMEOUT)
}

func legacyEntryAlive(Entry) bool {
	return true
}

func openProcess(pid int) (windows.Handle, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid process id %d", pid)
	}
	handle, err := windows.OpenProcess(
		windows.PROCESS_QUERY_LIMITED_INFORMATION|windows.SYNCHRONIZE,
		false,
		uint32(pid),
	)
	if err != nil {
		return 0, fmt.Errorf("open process %d: %w", pid, err)
	}
	return handle, nil
}

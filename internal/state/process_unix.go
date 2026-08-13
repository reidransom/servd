//go:build !windows

package state

import (
	"errors"
	"syscall"
)

// ProcessIdentity returns the process group identifier used to distinguish a
// live process from a recycled PID on Unix.
func ProcessIdentity(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, syscall.EINVAL
	}
	pgid, err := syscall.Getpgid(pid)
	return uint64(pgid), err
}

// ProcessAlive reports whether a process with pid exists.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

func legacyEntryAlive(entry Entry) bool {
	if entry.PGID <= 0 {
		return true
	}
	pgid, err := syscall.Getpgid(entry.PID)
	return err == nil && pgid == entry.PGID
}

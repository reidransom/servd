//go:build !windows

package supervisor

import (
	"os/exec"
	"syscall"

	"github.com/reidransom/servd/internal/state"
)

func newShellCommand(command string) *exec.Cmd {
	return exec.Command("sh", "-c", command)
}

func prepareCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func processGroupID(pid int) int {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return pid
	}
	return pgid
}

func terminateProcess(entry state.Entry, force bool) error {
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}
	if entry.PGID > 0 {
		if err := syscall.Kill(-entry.PGID, signal); err == nil {
			return nil
		}
	}
	return syscall.Kill(entry.PID, signal)
}

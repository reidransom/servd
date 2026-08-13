//go:build windows

package supervisor

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"syscall"

	"github.com/reidransom/servd/internal/state"
	"golang.org/x/sys/windows"
)

func newShellCommand(command string) *exec.Cmd {
	return exec.Command("cmd.exe", "/d", "/s", "/v:off", "/c", command)
}

func prepareCommand(command *exec.Cmd) {
	command.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
		HideWindow:    true,
	}
}

func processGroupID(int) int {
	return 0
}

func terminateProcess(entry state.Entry, force bool) error {
	if !state.EntryAlive(entry) {
		return nil
	}
	arguments := []string{"/PID", strconv.Itoa(entry.PID), "/T"}
	if force {
		arguments = append(arguments, "/F")
	}
	command := exec.Command("taskkill.exe", arguments...)
	command.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := command.CombinedOutput()
	if err != nil && state.EntryAlive(entry) {
		message := strings.TrimSpace(string(output))
		if message == "" {
			message = err.Error()
		}
		return fmt.Errorf("terminate process tree %d: %s", entry.PID, message)
	}
	return nil
}

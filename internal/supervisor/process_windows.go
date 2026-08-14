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
	shell := exec.Command("cmd.exe")
	shell.Args = nil
	shell.SysProcAttr = &syscall.SysProcAttr{
		CmdLine: `cmd.exe /d /s /v:off /c "` + command + `"`,
	}
	return shell
}

func prepareCommand(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.CreationFlags = windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS
	command.SysProcAttr.HideWindow = true
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

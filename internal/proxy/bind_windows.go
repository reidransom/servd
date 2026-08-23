//go:build windows

package proxy

import (
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/state"
)

type workerProcess struct {
	PID      int
	PGID     int
	Identity uint64
}

func startElevatedWorker(config.Settings) (workerProcess, error) {
	return workerProcess{}, fmt.Errorf("elevation is unavailable on Windows")
}

func startUserWorker(settings config.Settings, listener net.Listener) (workerProcess, error) {
	if listener == nil {
		return workerProcess{}, fmt.Errorf("proxy listener was not acquired")
	}
	if err := listener.Close(); err != nil {
		return workerProcess{}, err
	}
	worker, err := os.Executable()
	if err != nil {
		return workerProcess{}, err
	}
	command := exec.Command(worker, workerArgs(settings.Hostnames.HTTPPort, settings.Hostnames.LAN)...)
	command.Stdin = nil
	if err := command.Start(); err != nil {
		return workerProcess{}, err
	}
	if err := waitForWindowsWorker(settings); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		return workerProcess{}, err
	}
	identity, err := state.ProcessIdentity(command.Process.Pid)
	if err != nil {
		return workerProcess{}, err
	}
	if err := command.Process.Release(); err != nil {
		return workerProcess{}, err
	}
	return workerProcess{PID: command.Process.Pid, PGID: command.Process.Pid, Identity: identity}, nil
}

func waitForWindowsWorker(settings config.Settings) error {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if netcheck.PortAccepting(settings.BindHost, settings.Hostnames.HTTPPort) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("proxy worker did not become ready")
}

func isPermissionError(err error) bool {
	return errors.Is(err, os.ErrPermission)
}

func terminateWorker(process workerProcess) error {
	target, err := os.FindProcess(process.PID)
	if err != nil {
		return err
	}
	return target.Kill()
}

//go:build !windows

package proxy

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

func startUserWorker(settings config.Settings, listener net.Listener) (workerProcess, error) {
	if listener == nil {
		return workerProcess{}, fmt.Errorf("proxy listener was not acquired")
	}
	defer func() { _ = listener.Close() }()
	listenerFile, err := listenerFile(listener)
	if err != nil {
		return workerProcess{}, err
	}
	defer func() { _ = listenerFile.Close() }()
	worker, err := os.Executable()
	if err != nil {
		return workerProcess{}, err
	}
	return spawnWorker(workerStartRequest{Worker: worker, Port: settings.Hostnames.HTTPPort, LAN: settings.Hostnames.LAN, Listener: listenerFile})
}

type workerStartRequest struct {
	Worker     string
	Port       int
	LAN        bool
	Listener   *os.File
	Home       string
	ConfigHome string
	StateHome  string
	Credential *syscall.Credential
}

func spawnWorker(request workerStartRequest) (workerProcess, error) {
	readyRead, readyWrite, err := os.Pipe()
	if err != nil {
		return workerProcess{}, err
	}
	defer func() { _ = readyRead.Close() }()
	defer func() { _ = readyWrite.Close() }()

	cmd := exec.Command(request.Worker, workerArgs(request.Port, request.LAN)...)
	cmd.ExtraFiles = []*os.File{request.Listener, readyWrite}
	cmd.Stdin = nil
	cmd.Env = workerEnvironment(request)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true, Credential: request.Credential}
	if err := cmd.Start(); err != nil {
		return workerProcess{}, err
	}
	_ = readyWrite.Close()

	if err := waitForWorkerReady(readyRead); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return workerProcess{}, err
	}
	identity, err := state.ProcessIdentity(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
		return workerProcess{}, fmt.Errorf("identify proxy worker %d: %w", cmd.Process.Pid, err)
	}
	process := workerProcess{PID: cmd.Process.Pid, PGID: processGroupID(cmd.Process.Pid), Identity: identity}
	if err := cmd.Process.Release(); err != nil {
		return workerProcess{}, err
	}
	return process, nil
}

func RunInheritedWorker(settings config.Settings, listenerFile, readyFile *os.File) error {
	listener, err := net.FileListener(listenerFile)
	if err != nil {
		_, _ = fmt.Fprintf(readyFile, "error: %v\n", err)
		return err
	}
	defer func() { _ = listener.Close() }()
	defer func() { _ = readyFile.Close() }()
	return New(settings).serve(listener, func() { _, _ = fmt.Fprintln(readyFile, "ready") })
}

func listenerFile(listener net.Listener) (*os.File, error) {
	tcpListener, ok := listener.(*net.TCPListener)
	if !ok {
		return nil, fmt.Errorf("unsupported listener type %T", listener)
	}
	return tcpListener.File()
}

func workerEnvironment(request workerStartRequest) []string {
	env := os.Environ()
	if request.Home != "" {
		env = replaceEnvironment(env, "HOME="+request.Home)
	}
	if request.ConfigHome != "" {
		env = replaceEnvironment(env, "XDG_CONFIG_HOME="+request.ConfigHome)
	}
	if request.StateHome != "" {
		env = replaceEnvironment(env, "XDG_STATE_HOME="+request.StateHome)
	}
	return env
}

func replaceEnvironment(env []string, value string) []string {
	key, _, _ := strings.Cut(value, "=")
	prefix := key + "="
	for index, existing := range env {
		if strings.HasPrefix(existing, prefix) {
			env[index] = value
			return env
		}
	}
	return append(env, value)
}

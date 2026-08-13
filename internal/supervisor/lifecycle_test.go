package supervisor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/state"
)

func TestStartStopProcessTree(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	childPIDPath := filepath.Join(t.TempDir(), "child.pid")
	t.Setenv("SERVD_SUPERVISOR_PARENT", "1")
	t.Setenv("SERVD_SUPERVISOR_CHILD_PID", childPIDPath)

	port := availablePort(t)
	settings := config.DefaultSettings()
	site := config.Site{
		Slug: "process-tree",
		Port: port,
		Cmd:  launcher.ShellJoin([]string{os.Args[0], "-test.run=^TestSupervisorParentProcess$"}),
	}
	if err := Start(site, settings); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Stop(site.Slug) })
	if err := WaitReady(site, 5*time.Second); err != nil {
		t.Fatal(err)
	}

	childPID := waitForChildPID(t, childPIDPath)
	if !state.ProcessAlive(childPID) {
		t.Fatalf("child process %d is not alive after Start returned", childPID)
	}
	if err := Stop(site.Slug); err != nil {
		t.Fatal(err)
	}
	waitForProcessExit(t, childPID)
}

func TestStartReportsFailureLog(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("SERVD_SUPERVISOR_FAILURE", "1")
	settings := config.DefaultSettings()
	site := config.Site{
		Slug: "startup-failure",
		Port: availablePort(t),
		Cmd:  launcher.ShellJoin([]string{os.Args[0], "-test.run=^TestSupervisorFailureProcess$"}),
	}
	err := Start(site, settings)
	if err == nil || !strings.Contains(err.Error(), "intentional startup failure") {
		t.Fatalf("Start() error = %v, want log-tail failure", err)
	}
}

func TestSupervisorParentProcess(t *testing.T) {
	if os.Getenv("SERVD_SUPERVISOR_PARENT") != "1" {
		return
	}
	command := exec.Command(os.Args[0], "-test.run=^TestSupervisorChildProcess$")
	command.Env = append(os.Environ(), "SERVD_SUPERVISOR_CHILD=1")
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(os.Getenv("SERVD_SUPERVISOR_CHILD_PID"), []byte(strconv.Itoa(command.Process.Pid)), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := command.Wait(); err != nil {
		t.Fatal(err)
	}
}

func TestSupervisorChildProcess(t *testing.T) {
	if os.Getenv("SERVD_SUPERVISOR_CHILD") != "1" {
		return
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(os.Getenv("HOST"), os.Getenv("PORT")))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("close listener: %v", err)
		}
	}()
	for {
		connection, err := listener.Accept()
		if err != nil {
			return
		}
		_ = connection.Close()
	}
}

func TestSupervisorFailureProcess(t *testing.T) {
	if os.Getenv("SERVD_SUPERVISOR_FAILURE") != "1" {
		return
	}
	fmt.Println("intentional startup failure")
	os.Exit(7)
}

func availablePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func waitForChildPID(t *testing.T, path string) int {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil {
			pid, err := strconv.Atoi(string(data))
			if err != nil {
				t.Fatal(err)
			}
			return pid
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child PID was not written to %s", path)
	return 0
}

func waitForProcessExit(t *testing.T, pid int) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if !state.ProcessAlive(pid) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d survived Stop", pid)
}

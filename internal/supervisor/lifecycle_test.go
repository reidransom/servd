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
		Path: t.TempDir(),
		Cmd:  launcher.ShellJoin([]string{os.Args[0], "-test.run=^TestSupervisorParentProcess$"}),
	}
	if err := Start(site, settings); err != nil {
		t.Fatal(err)
	}
	if err := WaitReady(site, settings, 5*time.Second); err != nil {
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
		Path: t.TempDir(),
		Port: availablePort(t),
		Cmd:  launcher.ShellJoin([]string{os.Args[0], "-test.run=^TestSupervisorFailureProcess$"}),
	}
	err := Start(site, settings)
	if err == nil || !strings.Contains(err.Error(), "intentional startup failure") {
		t.Fatalf("Start() error = %v, want log-tail failure", err)
	}
}

func TestStartDoesNotPersistStaticResolutionFailure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{
		Slug: "missing-project",
		Path: filepath.Join(t.TempDir(), "missing"),
		Port: availablePort(t),
		Cmd:  "sleep 30",
	}
	settings := config.DefaultSettings()
	if err := Start(site, settings); err == nil {
		t.Fatal("Start() with missing project should fail")
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(site.Slug); ok {
		t.Fatal("static resolution failure should not persist a runtime attempt")
	}
	if err := os.MkdirAll(site.Path, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(site, settings, st); got.Kind != Stopped {
		t.Errorf("repaired static status = %#v, want stopped", got)
	}
}
func TestStartRecordsExitFailure(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{
		Slug: "exit-failure",
		Path: t.TempDir(),
		Port: availablePort(t),
		Cmd:  "exit 7",
	}
	if err := Start(site, config.DefaultSettings()); err == nil {
		t.Fatal("Start() with exit 7 should fail")
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := st.Get(site.Slug)
	if !ok {
		t.Fatal("failed start was not retained")
	}
	if entry.Failure == "" || entry.FailedAt.IsZero() {
		t.Errorf("failure record = %#v, want concise failure and time", entry)
	}
}

func TestSuccessfulStartReplacesFailedEntry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{
		Slug: "replaces-failure",
		Path: t.TempDir(),
		Port: availablePort(t),
		Cmd:  "exit 7",
	}
	settings := config.DefaultSettings()
	if err := Start(site, settings); err == nil {
		t.Fatal("Start() with exit 7 should fail")
	}
	site.Cmd = "sleep 30"
	if err := Start(site, settings); err != nil {
		t.Fatalf("Start() after failure = %v", err)
	}
	t.Cleanup(func() { _ = Stop(site.Slug) })
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := st.Get(site.Slug)
	if !ok {
		t.Fatal("successful start was not recorded")
	}
	if entry.Failure != "" || !entry.FailedAt.IsZero() || !state.EntryAlive(entry) {
		t.Errorf("success record = %#v, want live entry without failure", entry)
	}
}

func TestStopClearsFailedEntry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{
		Slug: "stops-failure",
		Path: t.TempDir(),
		Port: availablePort(t),
		Cmd:  "exit 7",
	}
	if err := Start(site, config.DefaultSettings()); err == nil {
		t.Fatal("Start() with exit 7 should fail")
	}
	if err := Stop(site.Slug); err != nil {
		t.Fatal(err)
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(site.Slug); ok {
		t.Fatal("Stop() did not clear failed entry")
	}
}

func TestInvalidNextCommandKeepsRunningSiteStoppable(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{
		Slug: "invalid-next-command",
		Path: t.TempDir(),
		Port: availablePort(t),
		Cmd:  "sleep 30",
	}
	settings := config.DefaultSettings()
	if err := Start(site, settings); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = Stop(site.Slug) })

	site.Cmd = ""
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := Evaluate(site, settings, st); got.Kind != Error || !strings.Contains(got.Reason, "no command configured") {
		t.Fatalf("invalid running site status = %#v, want command-resolution error", got)
	}
	if err := Stop(site.Slug); err != nil {
		t.Fatalf("Stop() invalid running site: %v", err)
	}
	st, err = state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := st.Get(site.Slug); ok {
		t.Fatal("Stop() left invalid running site in state")
	}
}

func TestRecordFailedStartKeepsConcurrentLiveEntry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	pid := os.Getpid()
	identity, err := state.ProcessIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}
	site := config.Site{Slug: "concurrent", Path: t.TempDir(), Port: availablePort(t), Cmd: "exit 7"}
	if err := state.Mutate(func(s *state.State) error {
		s.Entries[site.Slug] = state.Entry{Slug: site.Slug, PID: pid, Identity: identity, StartedAt: time.Now()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := recordFailedStart(site, launcher.Resolved{Cmd: site.Cmd, Dir: site.Path}, "shell start failed"); err != nil {
		t.Fatal(err)
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := st.Get(site.Slug)
	if !ok {
		t.Fatal("live entry missing")
	}
	if entry.PID != pid || entry.Failure != "" {
		t.Errorf("live entry = %#v, want original live entry", entry)
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

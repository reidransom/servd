// Package supervisor starts, stops and inspects dev-server processes.
//
// Each server is launched detached in its own process group (Setpgid) so it
// survives the CLI/TUI exiting. Output is captured to a per-site logfile, and
// the live pid/port is recorded in the state package. Stopping signals the
// whole process group, so child processes (e.g. npm -> node) die too.
package supervisor

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/state"
)

// Status is a site's current runtime status.
type Status int

const (
	Stopped  Status = iota // no live process
	Starting               // process alive but port not yet accepting
	Running                // process alive and port accepting connections
)

func (s Status) String() string {
	switch s {
	case Running:
		return "running"
	case Starting:
		return "starting"
	default:
		return "stopped"
	}
}

// logPath returns the per-site logfile path.
func logPath(slug string) string {
	return filepath.Join(config.LogDir(), slug+".log")
}

// LogPath exposes the logfile path for a slug (used by `servd logs`).
func LogPath(slug string) string { return logPath(slug) }

// Start launches the site's dev server detached. It is a no-op (no error) if
// the site is already running.
func Start(site config.Site, settings config.Settings, st *state.State) error {
	if e, ok := st.Get(site.Slug); ok && state.ProcessAlive(e.PID) {
		return nil
	}
	res, err := launcher.Resolve(site, settings)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(config.LogDir(), 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(logPath(site.Slug), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logf.Close()

	header := fmt.Sprintf("\n=== servd starting %q at %s ===\n$ %s\n",
		site.Slug, time.Now().Format(time.RFC3339), res.Cmd)
	_, _ = logf.WriteString(header)

	cmd := exec.Command("sh", "-c", res.Cmd)
	cmd.Dir = res.Dir
	cmd.Env = append(os.Environ(),
		"PORT="+strconv.Itoa(site.Port),
		"HOST="+settings.BindHost,
	)
	cmd.Stdout = logf
	cmd.Stderr = logf
	cmd.Stdin = nil
	// New process group so we can signal the whole tree on stop.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		return err
	}
	pid := cmd.Process.Pid
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		pgid = pid
	}

	// Verify the process survives a brief grace period. A dev server that
	// fails to bind its port (e.g. address already in use) exits almost
	// immediately; catch that and surface the log tail instead of silently
	// reporting success. The Wait goroutine also reaps the child if it exits.
	waited := make(chan error, 1)
	go func() { waited <- cmd.Wait() }()
	select {
	case werr := <-waited:
		tail := lastLines(logPath(site.Slug), 12)
		if tail != "" {
			return fmt.Errorf("%s exited on startup (%v):\n%s", site.Slug, werr, tail)
		}
		return fmt.Errorf("%s exited on startup: %v", site.Slug, werr)
	case <-time.After(600 * time.Millisecond):
		// Still alive — leave the Wait goroutine running; when we exit, init reaps.
	}

	return st.Set(state.Entry{
		Slug:      site.Slug,
		PID:       pid,
		PGID:      pgid,
		Port:      site.Port,
		Cmd:       res.Cmd,
		Log:       logPath(site.Slug),
		StartedAt: time.Now(),
	})
}

// Stop signals the site's process group (SIGTERM, then SIGKILL after a grace
// period) and clears its state entry. No-op if not running.
func Stop(slug string, st *state.State) error {
	e, ok := st.Get(slug)
	if !ok {
		return nil
	}
	signalGroup(e.PGID, e.PID, syscall.SIGTERM)

	// Wait up to ~5s for graceful exit, then force-kill.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if !state.ProcessAlive(e.PID) {
			return st.Delete(slug)
		}
		time.Sleep(100 * time.Millisecond)
	}
	signalGroup(e.PGID, e.PID, syscall.SIGKILL)
	time.Sleep(200 * time.Millisecond)
	return st.Delete(slug)
}

// signalGroup signals the process group if pgid is valid, else the pid.
func signalGroup(pgid, pid int, sig syscall.Signal) {
	if pgid > 0 {
		if err := syscall.Kill(-pgid, sig); err == nil {
			return
		}
	}
	_ = syscall.Kill(pid, sig)
}

// Restart stops then starts the site.
func Restart(site config.Site, settings config.Settings, st *state.State) error {
	if err := Stop(site.Slug, st); err != nil {
		return err
	}
	return Start(site, settings, st)
}

// StatusOf reports the runtime status of a site given current state.
func StatusOf(site config.Site, st *state.State) Status {
	e, ok := st.Get(site.Slug)
	if !ok || !state.ProcessAlive(e.PID) {
		return Stopped
	}
	if portAccepting(site.Port) {
		return Running
	}
	return Starting
}

// lastLines returns the last n non-empty-trimmed lines of a file.
func lastLines(path string, n int) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

// portAccepting reports whether something is listening on 127.0.0.1:port.
func portAccepting(port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// Uptime returns how long the site has been running, or 0 if stopped.
func Uptime(slug string, st *state.State) time.Duration {
	e, ok := st.Get(slug)
	if !ok || !state.ProcessAlive(e.PID) {
		return 0
	}
	return time.Since(e.StartedAt)
}

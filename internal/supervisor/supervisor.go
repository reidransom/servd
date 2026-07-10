// Package supervisor starts, stops and inspects dev-server processes.
//
// Each server is launched detached in its own process group (Setpgid) so it
// survives the CLI/TUI exiting. Output is captured to a per-site logfile, and
// the live pid/port is recorded in the state package. Stopping signals the
// whole process group, so child processes (e.g. npm -> node) die too.
package supervisor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
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

// LogPath returns the per-site logfile path.
func LogPath(slug string) string {
	return filepath.Join(config.LogDir(), slug+".log")
}

// Start launches the site's dev server detached. It is a no-op (no error) if
// the site is already running.
func Start(site config.Site, settings config.Settings) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if e, ok := st.Get(site.Slug); ok && state.EntryAlive(e) {
		return nil
	}
	res, err := launcher.Resolve(site, settings)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(config.LogDir(), 0o755); err != nil {
		return err
	}
	logf, err := os.OpenFile(LogPath(site.Slug), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = logf.Close() }()

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
		tail := lastLines(LogPath(site.Slug), 12)
		if tail != "" {
			return fmt.Errorf("%s exited on startup (%v):\n%s", site.Slug, werr, tail)
		}
		return fmt.Errorf("%s exited on startup: %v", site.Slug, werr)
	case <-time.After(600 * time.Millisecond):
		// Still alive — leave the Wait goroutine running; when we exit, init reaps.
	}

	return state.Mutate(func(s *state.State) error {
		if e, ok := s.Get(site.Slug); ok && state.EntryAlive(e) && e.PID != pid {
			// Someone else started this site while we were spawning; keep
			// theirs and tear ours down.
			signalGroup(pgid, pid, syscall.SIGTERM)
			return nil
		}
		s.Entries[site.Slug] = state.Entry{
			Slug:      site.Slug,
			PID:       pid,
			PGID:      pgid,
			Port:      site.Port,
			Cmd:       res.Cmd,
			Log:       LogPath(site.Slug),
			StartedAt: time.Now(),
		}
		return nil
	})
}

// Stop signals the site's process group (SIGTERM, then SIGKILL after a grace
// period) and clears its state entry. No-op if not running.
//
// The state lock is only taken for the final entry removal — never across the
// signal/poll window.
func Stop(slug string) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	e, ok := st.Get(slug)
	if !ok {
		return nil
	}
	// SIGTERM with a grace period, then force-kill. A SIGKILL survivor (e.g.
	// stuck in uninterruptible sleep) keeps its entry so `up` can't double-start.
	signalGroup(e.PGID, e.PID, syscall.SIGTERM)
	if waitDead(e, 5*time.Second) {
		return state.Delete(slug)
	}
	signalGroup(e.PGID, e.PID, syscall.SIGKILL)
	if waitDead(e, 2*time.Second) {
		return state.Delete(slug)
	}
	return fmt.Errorf("%s (pid %d) survived SIGKILL; keeping its state entry", slug, e.PID)
}

// waitDead polls until e's process dies or the timeout elapses.
func waitDead(e state.Entry, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if !state.EntryAlive(e) {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
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

// WaitReady blocks until the site's port accepts connections, its process
// dies, or the timeout elapses. Nil means Running; any error carries the log
// tail so callers can show why the server never came up.
func WaitReady(site config.Site, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		st, err := state.Load()
		if err != nil {
			return err
		}
		switch StatusOf(site, st) {
		case Running:
			return nil
		case Stopped:
			return waitErr(site.Slug, "exited before accepting connections")
		}
		if time.Now().After(deadline) {
			return waitErr(site.Slug, fmt.Sprintf("still not accepting connections on :%d after %s", site.Port, timeout))
		}
		time.Sleep(250 * time.Millisecond)
	}
}

// waitErr builds a WaitReady failure, appending the site's log tail if any.
func waitErr(slug, msg string) error {
	if tail := lastLines(LogPath(slug), 12); tail != "" {
		return fmt.Errorf("%s %s:\n%s", slug, msg, tail)
	}
	return fmt.Errorf("%s %s", slug, msg)
}

// Restart stops then starts the site.
func Restart(site config.Site, settings config.Settings) error {
	if err := Stop(site.Slug); err != nil {
		return err
	}
	return Start(site, settings)
}

// StatusOf reports the runtime status of a site given current state.
func StatusOf(site config.Site, st *state.State) Status {
	e, ok := st.Get(site.Slug)
	if !ok || !state.EntryAlive(e) {
		return Stopped
	}
	if netcheck.PortAccepting("127.0.0.1", site.Port) {
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

// Uptime returns how long the site has been running, or 0 if stopped.
func Uptime(slug string, st *state.State) time.Duration {
	e, ok := st.Get(slug)
	if !ok || !state.EntryAlive(e) {
		return 0
	}
	return time.Since(e.StartedAt)
}

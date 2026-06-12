package proxy

import (
	"net"
	"os"
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

// Slug is the reserved state key for the background proxy process.
const Slug = "__proxy"

// StartBackground launches `servd proxy` detached and records it in state.
// It is a no-op if the proxy is already running.
func StartBackground(settings config.Settings) error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	if e, ok := st.Get(Slug); ok && state.ProcessAlive(e.PID) {
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(config.LogDir(), 0o755); err != nil {
		return err
	}
	logPath := config.LogDir() + "/__proxy.log"
	logf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer logf.Close()

	c := exec.Command(self, "proxy")
	c.Stdout = logf
	c.Stderr = logf
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := c.Start(); err != nil {
		return err
	}
	pgid, _ := syscall.Getpgid(c.Process.Pid)
	go c.Wait()
	return st.Set(state.Entry{
		Slug: Slug, PID: c.Process.Pid, PGID: pgid,
		Port: settings.ProxyPort, Cmd: "servd proxy",
		Log: logPath, StartedAt: time.Now(),
	})
}

// StopBackground signals the background proxy and clears its state entry.
func StopBackground() error {
	st, err := state.Load()
	if err != nil {
		return err
	}
	e, ok := st.Get(Slug)
	if !ok || !state.ProcessAlive(e.PID) {
		return st.Delete(Slug)
	}
	if e.PGID > 0 {
		_ = syscall.Kill(-e.PGID, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(e.PID, syscall.SIGTERM)
	}
	return st.Delete(Slug)
}

// Running reports whether the background proxy process is alive, with its pid.
func Running(st *state.State) (bool, int) {
	e, ok := st.Get(Slug)
	if ok && state.ProcessAlive(e.PID) {
		return true, e.PID
	}
	return false, 0
}

// Accepting reports whether the proxy port is accepting connections.
func Accepting(settings config.Settings) bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(settings.BindHost, strconv.Itoa(settings.ProxyPort)),
		200*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

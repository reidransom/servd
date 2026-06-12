package proxy

import (
	"os"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// Slug is the reserved state key for the background proxy process.
const Slug = "__proxy"

// proxySite models the background proxy as a supervised site so start/stop
// share the supervisor's lifecycle handling (detached spawn, logfile,
// startup-failure detection, SIGTERM→SIGKILL escalation).
func proxySite(settings config.Settings) (config.Site, error) {
	self, err := os.Executable()
	if err != nil {
		return config.Site{}, err
	}
	return config.Site{
		Slug: Slug,
		Port: settings.ProxyPort,
		Cmd:  launcher.ShellQuote(self) + " proxy",
	}, nil
}

// StartBackground launches `servd proxy` detached and records it in state.
// It is a no-op if the proxy is already running.
func StartBackground(settings config.Settings) error {
	site, err := proxySite(settings)
	if err != nil {
		return err
	}
	return supervisor.Start(site, settings)
}

// StopBackground signals the background proxy and clears its state entry.
func StopBackground() error {
	return supervisor.Stop(Slug)
}

// Running reports whether the background proxy process is alive, with its pid.
func Running(st *state.State) (bool, int) {
	e, ok := st.Get(Slug)
	if ok && state.EntryAlive(e) {
		return true, e.PID
	}
	return false, 0
}

// Accepting reports whether the proxy port is accepting connections.
func Accepting(settings config.Settings) bool {
	return netcheck.PortAccepting(settings.BindHost, settings.ProxyPort)
}

package proxy

import (
	"fmt"
	"net"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// Slug is the reserved state key for the background proxy process.
const Slug = "__proxy"

// StartResult describes the listener selected for a background proxy.
type StartResult struct {
	Port         int
	UsedFallback bool
	PreferredErr error
}

func startConfiguredPort(port int, direct func(int) error, elevate func(int) error) (StartResult, error) {
	if err := direct(port); err == nil {
		return StartResult{Port: port}, nil
	} else if isPermissionError(err) && port < 1024 {
		if elevatedErr := elevate(port); elevatedErr == nil {
			return StartResult{Port: port}, nil
		} else {
			return StartResult{}, fmt.Errorf("could not bind configured proxy port %d: %w", port, elevatedErr)
		}
	} else {
		return StartResult{}, fmt.Errorf("could not bind configured proxy port %d: %w", port, err)
	}
}

func startFirstRunPort(direct func(int) error, elevate func(int) error) (StartResult, error) {
	if err := direct(80); err == nil {
		return StartResult{Port: 80}, nil
	} else {
		preferredErr := err
		if isPermissionError(err) {
			if elevatedErr := elevate(80); elevatedErr == nil {
				return StartResult{Port: 80}, nil
			} else {
				preferredErr = elevatedErr
			}
		}
		if fallbackErr := direct(8080); fallbackErr == nil {
			return StartResult{Port: 8080, UsedFallback: true, PreferredErr: preferredErr}, nil
		} else {
			return StartResult{}, fmt.Errorf("could not acquire port 80: %v; could not acquire port 8080: %w", preferredErr, fallbackErr)
		}
	}
}

// StartBackground launches the proxy worker with a listener acquired by the
// invoking user or, only for a low port, the narrow bind helper.
func StartBackground(settings config.Settings) (StartResult, error) {
	st, err := state.Load()
	if err != nil {
		return StartResult{}, err
	}
	if entry, ok := st.Get(Slug); ok && state.EntryAlive(entry) {
		return StartResult{Port: entry.Port}, nil
	}

	_, source, err := config.LoadSettingsWithSource()
	if err != nil {
		return StartResult{}, err
	}

	var listener net.Listener
	var elevated workerProcess
	direct := func(port int) error {
		var bindErr error
		listener, bindErr = net.Listen("tcp", net.JoinHostPort(settings.BindHost, fmt.Sprint(port)))
		return bindErr
	}
	elevate := func(port int) error {
		var startErr error
		elevated, startErr = startElevatedWorker(settingsWithPort(settings, port))
		return startErr
	}
	var result StartResult
	if source.ConfigPresent {
		result, err = startConfiguredPort(settings.Hostnames.HTTPPort, direct, elevate)
	} else {
		result, err = startFirstRunPort(direct, elevate)
	}
	if err != nil {
		return result, err
	}

	settings = settingsWithPort(settings, result.Port)
	process := elevated
	if process.PID == 0 {
		process, err = startUserWorker(settings, listener)
		if err != nil {
			return result, err
		}
	}
	if err := recordProxyWorker(process, settings); err != nil {
		_ = terminateWorker(process)
		return result, err
	}
	return result, nil
}

func settingsWithPort(settings config.Settings, port int) config.Settings {
	settings.Hostnames.HTTPPort = port
	return settings
}

func recordProxyWorker(process workerProcess, settings config.Settings) error {
	return state.Mutate(func(s *state.State) error {
		if existing, ok := s.Get(Slug); ok && state.EntryAlive(existing) {
			return fmt.Errorf("proxy already running (pid %d)", existing.PID)
		}
		s.Entries[Slug] = state.Entry{
			Slug:      Slug,
			PID:       process.PID,
			PGID:      process.PGID,
			Identity:  process.Identity,
			Port:      settings.Hostnames.HTTPPort,
			Cmd:       "servd proxy",
			Log:       supervisor.LogPath(Slug),
			StartedAt: time.Now(),
		}
		return nil
	})
}

// EffectiveSettings substitutes the live listener port while the proxy is
// running. The caller's settings remain the preferred settings for next start.
func EffectiveSettings(settings config.Settings, st *state.State) config.Settings {
	if entry, ok := st.Get(Slug); ok && state.EntryAlive(entry) {
		return settingsWithPort(settings, entry.Port)
	}
	return settings
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

// Accepting reports whether the active HTTP listener is accepting connections.
func Accepting(settings config.Settings) bool {
	return netcheck.PortAccepting(settings.BindHost, settings.Hostnames.HTTPPort)
}

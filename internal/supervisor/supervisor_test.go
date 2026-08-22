package supervisor

import (
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

func TestWaitReady(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := config.DefaultSettings()

	// Not running at all: WaitReady must fail immediately, not spin until the
	// timeout.
	stopped := config.Site{Slug: "gone", Path: t.TempDir(), Port: 1, Cmd: "sleep 30"}
	start := time.Now()
	err := WaitReady(stopped, settings, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Errorf("WaitReady(stopped) = %v, want 'exited' error", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("WaitReady(stopped) took %s, want immediate return", time.Since(start))
	}

	// Static evaluation failures surface their concise reason immediately.
	missing := config.Site{Slug: "missing", Path: filepath.Join(t.TempDir(), "missing"), Port: 1, Cmd: "sleep 30"}
	err = WaitReady(missing, settings, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Errorf("WaitReady(missing) = %v, want missing-path reason", err)
	}

	// Alive and accepting: register this test process as the site and listen
	// on its port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	site := config.Site{Slug: "ready", Path: t.TempDir(), Port: port, Cmd: "sleep 30"}
	identity, err := state.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	err = state.Mutate(func(s *state.State) error {
		s.Entries["ready"] = state.Entry{
			Slug: "ready", PID: os.Getpid(), Identity: identity, Port: port, StartedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WaitReady(site, settings, 5*time.Second); err != nil {
		t.Errorf("WaitReady(ready) = %v, want nil", err)
	}

	// Alive but never accepting: times out with a helpful error.
	lnBusy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deafPort := lnBusy.Addr().(*net.TCPAddr).Port
	_ = lnBusy.Close() // free the port so nothing accepts on it
	deaf := config.Site{Slug: "deaf", Path: t.TempDir(), Port: deafPort, Cmd: "sleep 30"}
	err = state.Mutate(func(s *state.State) error {
		s.Entries["deaf"] = state.Entry{
			Slug: "deaf", PID: os.Getpid(), Identity: identity, Port: deafPort, StartedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WaitReady(deaf, settings, 300*time.Millisecond); err == nil || !strings.Contains(err.Error(), "not accepting") {
		t.Errorf("WaitReady(deaf) = %v, want timeout error", err)
	}
}

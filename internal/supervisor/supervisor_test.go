package supervisor

import (
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

func TestWaitReady(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	// Not running at all: WaitReady must fail immediately, not spin until the
	// timeout.
	stopped := config.Site{Slug: "gone", Port: 1}
	start := time.Now()
	err := WaitReady(stopped, 5*time.Second)
	if err == nil || !strings.Contains(err.Error(), "exited") {
		t.Errorf("WaitReady(stopped) = %v, want 'exited' error", err)
	}
	if time.Since(start) > time.Second {
		t.Errorf("WaitReady(stopped) took %s, want immediate return", time.Since(start))
	}

	// Alive and accepting: register this test process as the site and listen
	// on its port.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	site := config.Site{Slug: "ready", Port: port}
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
	if err := WaitReady(site, 5*time.Second); err != nil {
		t.Errorf("WaitReady(ready) = %v, want nil", err)
	}

	// Alive but never accepting: times out with a helpful error.
	lnBusy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	deafPort := lnBusy.Addr().(*net.TCPAddr).Port
	_ = lnBusy.Close() // free the port so nothing accepts on it
	deaf := config.Site{Slug: "deaf", Port: deafPort}
	err = state.Mutate(func(s *state.State) error {
		s.Entries["deaf"] = state.Entry{
			Slug: "deaf", PID: os.Getpid(), Identity: identity, Port: deafPort, StartedAt: time.Now(),
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := WaitReady(deaf, 300*time.Millisecond); err == nil || !strings.Contains(err.Error(), "not accepting") {
		t.Errorf("WaitReady(deaf) = %v, want timeout error", err)
	}
}

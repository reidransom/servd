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

func TestEvaluate(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := config.DefaultSettings()
	project := t.TempDir()
	base := config.Site{Slug: "site", Path: project, Port: availablePort(t), Cmd: "sleep 30"}

	assertStatus := func(name string, site config.Site, entry *state.Entry, want Status, reason string) {
		t.Helper()
		if err := state.Delete(site.Slug); err != nil {
			t.Fatal(err)
		}
		if entry != nil {
			if err := state.Mutate(func(s *state.State) error {
				s.Entries[site.Slug] = *entry
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		}
		st, err := state.Load()
		if err != nil {
			t.Fatal(err)
		}
		got := Evaluate(site, settings, st)
		if got.Kind != want {
			t.Errorf("%s: status = %s (%q), want %s", name, got.Kind, got.Reason, want)
		}
		if reason != "" && !strings.Contains(got.Reason, reason) {
			t.Errorf("%s: reason = %q, want substring %q", name, got.Reason, reason)
		}
	}

	assertStatus("stopped", base, nil, Stopped, "")
	assertStatus("missing path", config.Site{Slug: base.Slug, Path: filepath.Join(project, "missing"), Port: base.Port, Cmd: base.Cmd}, nil, Error, "unavailable")
	assertStatus("missing command", config.Site{Slug: base.Slug, Path: project, Port: base.Port}, nil, Error, "no command configured")
	assertStatus("recorded failure", base, &state.Entry{Slug: base.Slug, Failure: "shell start failed", FailedAt: time.Now()}, Error, "shell start failed")
	assertStatus("dead process", base, &state.Entry{Slug: base.Slug, PID: 1<<30 - 7, Log: "/tmp/site.log", StartedAt: time.Now()}, Error, "process exited")

	pid := os.Getpid()
	identity, err := state.ProcessIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}
	assertStatus("starting", base, &state.Entry{Slug: base.Slug, PID: pid, Identity: identity, StartedAt: time.Now()}, Starting, "")
	assertStatus("readiness timeout", base, &state.Entry{Slug: base.Slug, PID: pid, Identity: identity, StartedAt: time.Now().Add(-31 * time.Second)}, Error, "not accepting")

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	ready := base
	ready.Port = listener.Addr().(*net.TCPAddr).Port
	assertStatus("running after readiness window", ready, &state.Entry{Slug: ready.Slug, PID: pid, Identity: identity, StartedAt: time.Now().Add(-31 * time.Second)}, Running, "")
}

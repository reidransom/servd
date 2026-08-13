package commands

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

func TestNewSiteInfo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := config.DefaultSettings()

	// A dead site: no state entry, nothing listening.
	dead := config.Site{Slug: "dead", Path: "/tmp/dead", Port: 1, Launcher: "static"}
	// A live site: a state entry pointing at this test process, and a real
	// listener so StatusOf sees the port accepting.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	live := config.Site{Slug: "live", Path: "/tmp/live", Port: port, Enabled: true}

	identity, err := state.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	err = state.Mutate(func(s *state.State) error {
		s.Entries["live"] = state.Entry{
			Slug: "live", PID: os.Getpid(), Identity: identity, Port: port,
			Cmd: "sleep 999", StartedAt: time.Now().Add(-time.Minute),
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}

	di := newSiteInfo(settings, dead, st)
	if di.Status != "stopped" || di.PID != 0 || di.StartedAt != nil {
		t.Errorf("dead site info = %+v, want stopped with no pid/started_at", di)
	}
	if want := settings.SiteURL(dead); di.URL != want {
		t.Errorf("dead URL = %q, want %q", di.URL, want)
	}
	if want, ok := settings.FallbackURL(dead); !ok || di.FallbackURL != want {
		t.Errorf("dead fallback URL = %q, want %q", di.FallbackURL, want)
	}
	// omitempty must drop the runtime fields entirely for stopped sites.
	data, err := json.Marshal(di)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"pid", "started_at", "uptime_seconds", "cmd"} {
		if strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("stopped-site JSON contains %q: %s", key, data)
		}
	}

	settings.Hostnames.NipIO = false
	withoutFallback, err := json.Marshal(newSiteInfo(settings, dead, st))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(withoutFallback), `"fallback_url"`) {
		t.Errorf("disabled fallback still appears in JSON: %s", withoutFallback)
	}

	li := newSiteInfo(settings, live, st)
	if li.Status != "running" {
		t.Errorf("live status = %q, want running", li.Status)
	}
	if li.PID != os.Getpid() || li.Cmd != "sleep 999" || li.StartedAt == nil || li.UptimeSeconds < 59 {
		t.Errorf("live site info = %+v, want this pid, cmd and ~1m uptime", li)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d/", port); li.DirectURL != want {
		t.Errorf("live direct_url = %q, want %q", li.DirectURL, want)
	}
}

func TestNewProxyInfoUsesRoutingNeutralPatterns(t *testing.T) {
	settings := config.DefaultSettings()
	info := newProxyInfo(settings, &state.State{})
	if info.Port != settings.Hostnames.HTTPPort || info.PrimaryURLPattern != "http://<slug>.localhost:8080/" || info.FallbackURLPattern != "http://<slug>.127.0.0.1.nip.io:8080/" || !info.NipIO {
		t.Fatalf("proxy info = %+v", info)
	}

	settings.Hostnames.NipIO = false
	data, err := json.Marshal(newProxyInfo(settings, &state.State{}))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"fallback_url_pattern"`) {
		t.Errorf("disabled fallback pattern appears in JSON: %s", data)
	}
}

package commands

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

func TestNewSiteInfo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := config.DefaultSettings()

	// A stopped site: no state entry, nothing listening.
	project := t.TempDir()
	dead := config.Site{Slug: "dead", Path: project, Port: 1, Cmd: "sleep 30"}
	// A live site: a state entry pointing at this test process, and a real
	// listener so StatusOf sees the port accepting.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	live := config.Site{Slug: "live", Path: project, Port: port, Cmd: "sleep 30"}

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
	if strings.Contains(string(data), `"error"`) || di.Error != "" {
		t.Errorf("stopped-site JSON contains error: %s", data)
	}
	if strings.Contains(string(data), `"enabled"`) {
		t.Errorf("site JSON contains removed enabled field: %s", data)
	}
	for _, key := range []string{"launcher", "source", "command_source"} {
		if strings.Contains(string(data), `"`+key+`"`) {
			t.Errorf("site JSON contains removed command metadata %q: %s", key, data)
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
	if li.Error != "" {
		t.Errorf("live site error = %q, want empty", li.Error)
	}
	if want := fmt.Sprintf("http://127.0.0.1:%d/", port); li.DirectURL != want {
		t.Errorf("live direct_url = %q, want %q", li.DirectURL, want)
	}
}

func TestNewSiteInfoError(t *testing.T) {
	settings := config.DefaultSettings()
	site := config.Site{Slug: "failed", Path: t.TempDir(), Port: 4011, Cmd: "sleep 30"}
	st := &state.State{Entries: map[string]state.Entry{
		site.Slug: {Slug: site.Slug, Failure: "shell start failed", FailedAt: time.Now()},
	}}
	info := newSiteInfo(settings, site, st)
	if info.Status != "error" || info.Error != "shell start failed" {
		t.Errorf("failed info = %+v, want status and error reason", info)
	}
	if info.PID != 0 || info.Cmd != "" || info.StartedAt != nil || info.UptimeSeconds != 0 {
		t.Errorf("failed runtime fields = %+v, want omitted", info)
	}
	data, err := json.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"error":"shell start failed"`) {
		t.Errorf("failed-site JSON omits error reason: %s", data)
	}

	dead := config.Site{Slug: "exited", Path: filepath.Join(site.Path, "."), Port: 4012, Cmd: "sleep 30"}
	deadInfo := newSiteInfo(settings, dead, &state.State{Entries: map[string]state.Entry{
		dead.Slug: {Slug: dead.Slug, PID: 1<<30 - 7, Cmd: "sleep 30", StartedAt: time.Now()},
	}})
	if deadInfo.Status != "error" || deadInfo.PID != 0 || deadInfo.UptimeSeconds != 0 {
		t.Errorf("dead info = %+v, want error without runtime fields", deadInfo)
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

func TestNewProxyInfoUsesLiveRuntimePort(t *testing.T) {
	identity, err := state.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.Hostnames.HTTPPort = 80
	st := &state.State{Entries: map[string]state.Entry{
		"__proxy": {Slug: "__proxy", PID: os.Getpid(), Identity: identity, Port: 8080},
	}}

	info := newProxyInfo(settings, st)
	if info.Port != 8080 || info.PrimaryURLPattern != "http://<slug>.localhost:8080/" {
		t.Fatalf("proxy info = %+v, want live port 8080", info)
	}
}

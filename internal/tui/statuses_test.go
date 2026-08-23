package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

func TestStatusGlyphs(t *testing.T) {
	cases := []struct {
		status supervisor.SiteStatus
		want   string
	}{
		{supervisor.SiteStatus{Kind: supervisor.Stopped}, "○"},
		{supervisor.SiteStatus{Kind: supervisor.Starting}, "◐"},
		{supervisor.SiteStatus{Kind: supervisor.Running}, "●"},
		{supervisor.SiteStatus{Kind: supervisor.Error}, "✕"},
	}
	for _, tc := range cases {
		glyph := statusGlyph(tc.status)
		if !strings.Contains(glyph, tc.want) {
			t.Errorf("status %s glyph = %q, want %q", tc.status.Kind, glyph, tc.want)
		}
		if got := lipgloss.Width(glyph); got != 1 {
			t.Errorf("status %s glyph width = %d, want 1", tc.status.Kind, got)
		}
	}
}

func TestBuildStatusesReflectsAndClearsStaticError(t *testing.T) {
	settings := config.DefaultSettings()
	project := filepath.Join(t.TempDir(), "missing")
	site := config.Site{Slug: "site", Path: project, Port: 4011, Cmd: "sleep 30"}
	registry := &config.Registry{Sites: []config.Site{site}}

	broken := buildStatuses(settings, registry, &state.State{Entries: map[string]state.Entry{}})
	status := broken.statuses[site.Slug]
	if status.Kind != supervisor.Error || !strings.Contains(status.Reason, "unavailable") {
		t.Fatalf("broken status = %#v, want missing-path error", status)
	}
	if !strings.Contains(broken.rows[0][0], "✕") {
		t.Errorf("broken row = %#v, want error glyph before slug", broken.rows[0])
	}

	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	repaired := buildStatuses(settings, registry, &state.State{Entries: map[string]state.Entry{}})
	if got := repaired.statuses[site.Slug]; got.Kind != supervisor.Stopped {
		t.Errorf("repaired status = %#v, want stopped", got)
	}
	if got := repaired.rows[0]; got[0] != "○" || got[1] != site.Slug {
		t.Errorf("repaired row = %#v, want stopped glyph before slug", got)
	}

	m := &model{
		cmdCache: map[string]string{site.Slug: "stale command"},
		statuses: broken.statuses,
		table:    table.New(table.WithColumns([]table.Column{{Title: "", Width: 1}, {Title: "SLUG", Width: 19}})),
	}
	m.applyStatuses(repaired)
	if _, ok := m.cmdCache[site.Slug]; ok {
		t.Error("status change did not invalidate cached launch command")
	}
}

func TestBuildStatusesUsesLiveProxyPort(t *testing.T) {
	identity, err := state.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.Hostnames.HTTPPort = 80
	message := buildStatuses(settings, &config.Registry{}, &state.State{Entries: map[string]state.Entry{
		"__proxy": {Slug: "__proxy", PID: os.Getpid(), Identity: identity, Port: 8080},
	}})
	if got := message.settings.Hostnames.HTTPPort; got != 8080 {
		t.Fatalf("display port = %d, want 8080", got)
	}
}

func TestProxyStatusShowsLandingURL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.settings.Hostnames.HTTPPort = 42200
	m.proxyRunning = true
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})

	firstLine, _, _ := strings.Cut(ansi.Strip(m.View()), "\n")
	if !strings.Contains(firstLine, "● proxy on http://127.0.0.1:42200/") {
		t.Errorf("proxy status does not show the landing URL:\n%s", firstLine)
	}
	for _, unwanted := range []string{"nip.io", "<slug>"} {
		if strings.Contains(firstLine, unwanted) {
			t.Errorf("proxy status contains %q:\n%s", unwanted, firstLine)
		}
	}
}

func TestSidebarUsesOneSpaceBetweenStatusAndSlug(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{Slug: "widget", Path: t.TempDir(), Port: 4011, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{site}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	view := ansi.Strip(m.sidebarTableView())
	if strings.Contains(view, "SLUG") {
		t.Errorf("sidebar still renders the SLUG header:\n%s", view)
	}
	firstLine, _, _ := strings.Cut(view, "\n")
	if !strings.Contains(firstLine, "widget") {
		t.Errorf("sidebar has a blank row before its first site:\n%s", view)
	}
	if !strings.Contains(view, "○ widget") {
		t.Errorf("sidebar row does not use one space between status and slug:\n%s", view)
	}
	if strings.Contains(view, "○  widget") {
		t.Errorf("sidebar row uses multiple spaces between status and slug:\n%s", view)
	}
}

func TestSelectedErrorShowsReason(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	missing := filepath.Join(t.TempDir(), "missing")
	if err := (&config.Registry{Sites: []config.Site{{Slug: "broken", Path: missing, Port: 4011, Cmd: "sleep 30"}}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	if len(m.reg.Sites) != 1 {
		t.Fatalf("model sites = %#v, want broken site", m.reg.Sites)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	view := m.View()
	if !strings.Contains(view, "ERROR:") || !strings.Contains(view, "unavailable") {
		t.Errorf("selected error reason missing from view:\n%s", view)
	}
}

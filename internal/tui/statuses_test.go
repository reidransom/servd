package tui

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	"github.com/muesli/termenv"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
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
	if row := broken.rows[slices.Index(broken.slugs, site.Slug)]; row[0] != "✕" {
		t.Errorf("broken row = %#v, want error glyph before slug", row)
	}

	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	repaired := buildStatuses(settings, registry, &state.State{Entries: map[string]state.Entry{}})
	if got := repaired.statuses[site.Slug]; got.Kind != supervisor.Stopped {
		t.Errorf("repaired status = %#v, want stopped", got)
	}
	if got := repaired.rows[slices.Index(repaired.slugs, site.Slug)]; got[0] != "○" || got[1] != site.Slug {
		t.Errorf("repaired row = %#v, want stopped glyph before slug", got)
	}
}

func TestProxySelectionShowsLiveLandingURL(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	identity, err := state.ProcessIdentity(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	m.settings.BindHost = "0.0.0.0"
	m.settings.Hostnames.TLD = "example.com"
	m.Update(buildStatuses(m.settings, &config.Registry{}, &state.State{Entries: map[string]state.Entry{
		proxy.Slug: {Slug: proxy.Slug, PID: os.Getpid(), Identity: identity, Port: 42200},
	}}))
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "●") || !strings.Contains(view, "→ http://servd.example.com:42200/") {
		t.Fatalf("selected proxy does not show its live listener:\n%s", view)
	}
	firstLine, _, _ := strings.Cut(view, "\n")
	if strings.ContainsAny(firstLine, "●○") {
		t.Errorf("proxy status still appears in the title:\n%s", firstLine)
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
	if !strings.Contains(view, "○ widget") {
		t.Errorf("sidebar row does not use one space between status and slug:\n%s", view)
	}
	if strings.Contains(view, "○  widget") {
		t.Errorf("sidebar row uses multiple spaces between status and slug:\n%s", view)
	}
}

func TestSidebarRendersColoredErrorGlyph(t *testing.T) {
	profile := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	t.Cleanup(func() {
		lipgloss.SetColorProfile(profile)
	})
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	missing := filepath.Join(root, "missing")
	broken := config.Site{Slug: "broken", Path: missing, Port: 4011, Cmd: "sleep 30"}
	healthy := config.Site{Slug: "healthy", Path: root, Port: 4012, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{broken, healthy}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	selectDashboardSite(t, m, "broken")

	view := m.sidebarTableView()
	if strings.Contains(view, "\x1b…") {
		t.Errorf("sidebar contains a truncated ANSI escape: %q", view)
	}
	if stripped := ansi.Strip(view); !strings.Contains(stripped, "✕ broken") {
		t.Errorf("sidebar error row = %q, want error glyph before slug", stripped)
	}
	if !strings.Contains(view, sidebarSelectedErrStyle.Render("✕")) {
		t.Errorf("selected error glyph is not red: %q", view)
	}

	selectDashboardSite(t, m, "healthy")
	view = m.sidebarTableView()
	if !strings.Contains(view, errStyle.Render("✕")) {
		t.Errorf("unselected error glyph is not red: %q", view)
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
	selectDashboardSite(t, m, "broken")
	view := m.View()
	if !strings.Contains(view, "ERROR:") || !strings.Contains(view, "unavailable") {
		t.Errorf("selected error reason missing from view:\n%s", view)
	}
}

package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	if !strings.Contains(broken.rows[0][1], "✕") {
		t.Errorf("broken row = %#v, want error glyph", broken.rows[0])
	}

	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}
	repaired := buildStatuses(settings, registry, &state.State{Entries: map[string]state.Entry{}})
	if got := repaired.statuses[site.Slug]; got.Kind != supervisor.Stopped {
		t.Errorf("repaired status = %#v, want stopped", got)
	}
	if got := repaired.rows[0][1]; got != "○" {
		t.Errorf("repaired glyph = %q, want stopped glyph", got)
	}

	m := &model{
		cmdCache: map[string]string{site.Slug: "stale command"},
		statuses: broken.statuses,
		table:    table.New(table.WithColumns([]table.Column{{Title: "SLUG", Width: 16}, {Title: "", Width: 2}})),
	}
	m.applyStatuses(repaired)
	if _, ok := m.cmdCache[site.Slug]; ok {
		t.Error("status change did not invalidate cached launch command")
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

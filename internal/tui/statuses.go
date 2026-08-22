package tui

import (
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/lipgloss"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// statusesMsg carries a freshly loaded registry/state and the table rows
// computed from them. A nil reg means the load failed and should be ignored.
type statusesMsg struct {
	reg          *config.Registry
	st           *state.State
	rows         []table.Row
	slugs        []string
	statuses     map[string]supervisor.SiteStatus
	proxyRunning bool
}

// buildStatuses computes table rows and shared health results off the Update
// goroutine because evaluating a status may dial a port.
func buildStatuses(settings config.Settings, reg *config.Registry, st *state.State) statusesMsg {
	running, _ := proxy.Running(st)
	rows := make([]table.Row, 0, len(reg.Sites))
	slugs := make([]string, 0, len(reg.Sites))
	statuses := make(map[string]supervisor.SiteStatus, len(reg.Sites))
	for _, site := range reg.Sites {
		status := supervisor.Evaluate(site, settings, st)
		statuses[site.Slug] = status
		rows = append(rows, table.Row{site.Slug, statusGlyph(status)})
		slugs = append(slugs, site.Slug)
	}
	return statusesMsg{reg: reg, st: st, rows: rows, slugs: slugs, statuses: statuses, proxyRunning: running}
}

func statusGlyph(status supervisor.SiteStatus) string {
	switch status.Kind {
	case supervisor.Running:
		return "●"
	case supervisor.Starting:
		return "◐"
	case supervisor.Error:
		return errStyle.Render("✕")
	default:
		return "○"
	}
}

func glyphWidth(status supervisor.SiteStatus) int {
	return lipgloss.Width(statusGlyph(status))
}

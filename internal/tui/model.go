// Package tui implements servd's interactive dashboard (Bubble Tea).
package tui

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/scan"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// Run starts the TUI event loop.
func Run() error {
	m, err := newModel()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

type mode int

const (
	modeTable mode = iota
	modeLogs
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// actionDoneMsg reports the result of an async supervisor/proxy action.
type actionDoneMsg struct {
	verb string // "started", "stopped", ...
	slug string // empty for bulk actions
	n    int    // count, for bulk actions
	bulk bool   // true when the action covered multiple sites
	err  error
}

// statusesMsg carries a freshly loaded registry/state and the table rows
// computed from them. A nil reg means the load failed and should be ignored.
type statusesMsg struct {
	reg          *config.Registry
	st           *state.State
	rows         []table.Row
	slugs        []string
	proxyRunning bool
}

type model struct {
	settings config.Settings
	reg      *config.Registry
	st       *state.State

	table    table.Model
	slugs    []string // parallel to table rows
	mode     mode
	logSlug  string
	viewport viewport.Model

	proxyRunning bool
	width        int
	height       int
	status       string // transient status line
	busy         bool   // an async action is in flight
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	offStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
)

func newModel() (*model, error) {
	settings, reg, st, err := app.Load()
	if err != nil {
		return nil, err
	}
	cols := []table.Column{
		{Title: "SLUG", Width: 20},
		{Title: "PORT", Width: 6},
		{Title: "LAUNCHER", Width: 12},
		{Title: "EN", Width: 4},
		{Title: "STATUS", Width: 9},
		{Title: "UPTIME", Width: 8},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("#7aa2f7")).BorderBottom(true)
	s.Selected = s.Selected.Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("#7aa2f7"))
	t.SetStyles(s)

	m := &model{settings: settings, reg: reg, st: st, table: t, viewport: viewport.New(80, 20)}
	m.applyStatuses(buildStatuses(reg, st))
	return m, nil
}

// buildStatuses computes the table rows for a registry+state snapshot. It
// dials ports (StatusOf), so it must run off the Update goroutine.
func buildStatuses(reg *config.Registry, st *state.State) statusesMsg {
	running, _ := proxy.Running(st)
	rows := make([]table.Row, 0, len(reg.Sites))
	slugs := make([]string, 0, len(reg.Sites))
	for _, s := range reg.Sites {
		stat := supervisor.StatusOf(s, st)
		up := ""
		if d := supervisor.Uptime(s.Slug, st); d > 0 {
			up = app.FmtDuration(d)
		}
		en := "✓"
		if !s.Enabled {
			en = "·"
		}
		rows = append(rows, table.Row{s.Slug, strconv.Itoa(s.Port), app.Dash(s.Launcher), en, stat.String(), app.Dash(up)})
		slugs = append(slugs, s.Slug)
	}
	return statusesMsg{reg: reg, st: st, rows: rows, slugs: slugs, proxyRunning: running}
}

// refreshCmd reloads registry+state and computes statuses in a goroutine.
func refreshCmd() tea.Cmd {
	return func() tea.Msg {
		reg, err := config.LoadRegistry()
		if err != nil {
			return statusesMsg{}
		}
		st, err := state.Load()
		if err != nil {
			return statusesMsg{}
		}
		return buildStatuses(reg, st)
	}
}

// applyStatuses swaps in a fresh snapshot, preserving the cursor position.
func (m *model) applyStatuses(msg statusesMsg) {
	if msg.reg == nil {
		return
	}
	m.reg, m.st = msg.reg, msg.st
	m.proxyRunning = msg.proxyRunning
	cur := m.table.Cursor()
	m.table.SetRows(msg.rows)
	m.slugs = msg.slugs
	if cur >= len(msg.rows) {
		cur = len(msg.rows) - 1
	}
	if cur < 0 {
		cur = 0
	}
	m.table.SetCursor(cur)
}

func (m *model) selectedSite() *config.Site {
	if len(m.slugs) == 0 {
		return nil
	}
	idx := m.table.Cursor()
	if idx < 0 || idx >= len(m.slugs) {
		return nil
	}
	return m.reg.Find(m.slugs[idx])
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.table.SetHeight(max(5, msg.Height-7))
		m.viewport.Width = msg.Width
		m.viewport.Height = max(5, msg.Height-5)
		return m, nil

	case tickMsg:
		if m.mode == modeLogs {
			m.loadLog()
		}
		return m, tea.Batch(refreshCmd(), tick())

	case statusesMsg:
		m.applyStatuses(msg)
		return m, nil

	case actionDoneMsg:
		m.busy = false
		switch {
		case msg.err != nil:
			m.status = "ERROR: " + firstLine(msg.err.Error())
		case msg.bulk:
			m.status = fmt.Sprintf("%s %d site(s)", msg.verb, msg.n)
		case msg.slug != "":
			m.status = msg.verb + " " + msg.slug
		default:
			m.status = msg.verb
		}
		return m, refreshCmd()

	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Delegate to the active widget.
	var cmd tea.Cmd
	if m.mode == modeLogs {
		m.viewport, cmd = m.viewport.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeLogs {
		switch msg.String() {
		case "q", "esc", "l":
			m.mode = modeTable
			return m, nil
		}
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}

	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "s":
		if s := m.selectedSite(); s != nil && !m.busy {
			site := *s
			m.busy = true
			m.status = "starting " + site.Slug + "…"
			settings := m.settings
			return m, func() tea.Msg {
				err := supervisor.Start(site, settings)
				return actionDoneMsg{verb: "started", slug: site.Slug, err: err}
			}
		}
	case "x":
		if s := m.selectedSite(); s != nil && !m.busy {
			slug := s.Slug
			m.busy = true
			m.status = "stopping " + slug + "…"
			return m, func() tea.Msg {
				err := supervisor.Stop(slug)
				return actionDoneMsg{verb: "stopped", slug: slug, err: err}
			}
		}
	case "r":
		if s := m.selectedSite(); s != nil && !m.busy {
			site := *s
			m.busy = true
			m.status = "restarting " + site.Slug + "…"
			settings := m.settings
			return m, func() tea.Msg {
				err := supervisor.Restart(site, settings)
				return actionDoneMsg{verb: "restarted", slug: site.Slug, err: err}
			}
		}
	case "a":
		if !m.busy {
			sites := append([]config.Site(nil), m.reg.Sites...)
			settings := m.settings
			m.busy = true
			m.status = "starting enabled sites…"
			return m, func() tea.Msg {
				n := 0
				var firstErr error
				for _, s := range sites {
					if !s.Enabled {
						continue
					}
					if err := supervisor.Start(s, settings); err != nil {
						if firstErr == nil {
							firstErr = err
						}
						continue
					}
					n++
				}
				return actionDoneMsg{verb: "started", n: n, bulk: true, err: firstErr}
			}
		}
	case "X":
		if !m.busy {
			sites := append([]config.Site(nil), m.reg.Sites...)
			m.busy = true
			m.status = "stopping all…"
			return m, func() tea.Msg {
				n := 0
				var firstErr error
				for _, s := range sites {
					if err := supervisor.Stop(s.Slug); err != nil {
						if firstErr == nil {
							firstErr = err
						}
						continue
					}
					n++
				}
				return actionDoneMsg{verb: "stopped", n: n, bulk: true, err: firstErr}
			}
		}
	case "e":
		if s := m.selectedSite(); s != nil {
			slug := s.Slug
			var enabled bool
			err := config.MutateRegistry(func(reg *config.Registry) error {
				site := reg.Find(slug)
				if site == nil {
					return fmt.Errorf("unknown site %q", slug)
				}
				site.Enabled = !site.Enabled
				enabled = site.Enabled
				return nil
			})
			if err != nil {
				m.status = "ERROR: " + firstLine(err.Error())
				return m, nil
			}
			if enabled {
				m.status = "enabled " + slug
			} else {
				m.status = "disabled " + slug
			}
			return m, refreshCmd()
		}
	case "o":
		if s := m.selectedSite(); s != nil {
			_ = app.OpenBrowser(m.settings.SiteURL(*s))
			m.status = "opened " + s.Slug
		}
	case "S":
		var added []scan.Result
		err := config.MutateRegistry(func(reg *config.Registry) error {
			var err error
			added, err = scan.Scan(m.settings.ProjectsDir, reg, m.settings)
			return err
		})
		if err != nil {
			m.status = "ERROR: " + firstLine(err.Error())
			return m, nil
		}
		m.status = fmt.Sprintf("scan: +%d site(s)", len(added))
		return m, refreshCmd()
	case "p":
		if m.busy {
			break
		}
		running := m.proxyRunning
		settings := m.settings
		m.busy = true
		if running {
			m.status = "stopping proxy…"
		} else {
			m.status = "starting proxy…"
		}
		return m, func() tea.Msg {
			var err error
			verb := "proxy started"
			if running {
				verb = "proxy stopped"
				err = proxy.StopBackground()
			} else {
				err = proxy.StartBackground(settings)
			}
			return actionDoneMsg{verb: verb, err: err}
		}
	case "l":
		if s := m.selectedSite(); s != nil {
			m.mode = modeLogs
			m.logSlug = s.Slug
			m.loadLog()
		}
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *model) loadLog() {
	data, err := os.ReadFile(supervisor.LogPath(m.logSlug))
	if err != nil {
		m.viewport.SetContent("(no logs yet for " + m.logSlug + ")")
		return
	}
	m.viewport.SetContent(string(data))
	m.viewport.GotoBottom()
}

func (m *model) View() string {
	if m.mode == modeLogs {
		head := titleStyle.Render("logs: "+m.logSlug) + "  " + helpStyle.Render("↑/↓ scroll · q/l back")
		return head + "\n" + m.viewport.View()
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render("servd") + dimStyle.Render(" — local dev servers") + "\n\n")

	if len(m.reg.Sites) == 0 {
		b.WriteString(dimStyle.Render("No sites registered. Press ") + "S" + dimStyle.Render(" to scan ") + m.settings.ProjectsDir + "\n\n")
	} else {
		b.WriteString(m.table.View() + "\n")
	}

	// Selected site URL.
	if s := m.selectedSite(); s != nil {
		b.WriteString(dimStyle.Render("→ ") + m.settings.SiteURL(*s) + "\n")
	}

	// Proxy line.
	if m.proxyRunning {
		b.WriteString(okStyle.Render("● proxy on") + dimStyle.Render(fmt.Sprintf(" :%d  *.%s", m.settings.ProxyPort, m.settings.DomainSuffix)))
	} else {
		b.WriteString(offStyle.Render("○ proxy off") + dimStyle.Render("  press p to start"))
	}
	if m.status != "" {
		style := statusStyle
		if strings.HasPrefix(m.status, "ERROR:") {
			style = errStyle
		}
		b.WriteString("   " + style.Render(m.status))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("s start · x stop · r restart · a all · X stop-all · e en/disable · l logs · o open · p proxy · S scan · q quit"))
	return b.String()
}

// firstLine truncates a (possibly multi-line) error message for the status bar.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

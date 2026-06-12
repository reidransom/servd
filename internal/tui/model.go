// Package tui implements servd's interactive dashboard (Bubble Tea).
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
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
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	warnStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	offStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
)

func newModel() (*model, error) {
	settings, reg, st, err := loadAll()
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
	m.refresh()
	return m, nil
}

func loadAll() (config.Settings, *config.Registry, *state.State, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		return settings, nil, nil, err
	}
	reg, err := config.LoadRegistry()
	if err != nil {
		return settings, nil, nil, err
	}
	st, err := state.Load()
	if err != nil {
		return settings, reg, nil, err
	}
	return settings, reg, st, nil
}

// refresh reloads registry+state and rebuilds the table rows, preserving the
// cursor position.
func (m *model) refresh() {
	if reg, err := config.LoadRegistry(); err == nil {
		m.reg = reg
	}
	if st, err := state.Load(); err == nil {
		m.st = st
	}
	running, _ := proxy.Running(m.st)
	m.proxyRunning = running

	rows := make([]table.Row, 0, len(m.reg.Sites))
	slugs := make([]string, 0, len(m.reg.Sites))
	for _, s := range m.reg.Sites {
		st := supervisor.StatusOf(s, m.st)
		up := ""
		if d := supervisor.Uptime(s.Slug, m.st); d > 0 {
			up = fmtDur(d)
		}
		en := "✓"
		if !s.Enabled {
			en = "·"
		}
		rows = append(rows, table.Row{s.Slug, strconv.Itoa(s.Port), dash(s.Launcher), en, st.String(), dash(up)})
		slugs = append(slugs, s.Slug)
	}
	cur := m.table.Cursor()
	m.table.SetRows(rows)
	m.slugs = slugs
	if cur >= len(rows) {
		cur = len(rows) - 1
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
		m.table.SetHeight(maxInt(5, msg.Height-7))
		m.viewport.Width = msg.Width
		m.viewport.Height = maxInt(5, msg.Height-5)
		return m, nil

	case tickMsg:
		m.refresh()
		if m.mode == modeLogs {
			m.loadLog()
		}
		return m, tick()

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
		if s := m.selectedSite(); s != nil {
			_ = supervisor.Start(*s, m.settings, m.st)
			m.status = "started " + s.Slug
			m.refresh()
		}
	case "x":
		if s := m.selectedSite(); s != nil {
			m.status = "stopping " + s.Slug + "…"
			_ = supervisor.Stop(s.Slug, m.st)
			m.status = "stopped " + s.Slug
			m.refresh()
		}
	case "r":
		if s := m.selectedSite(); s != nil {
			_ = supervisor.Restart(*s, m.settings, m.st)
			m.status = "restarted " + s.Slug
			m.refresh()
		}
	case "a":
		n := 0
		for _, s := range m.reg.Sites {
			if !s.Enabled {
				continue
			}
			_ = supervisor.Start(s, m.settings, m.st)
			n++
		}
		m.status = fmt.Sprintf("started %d enabled site(s)", n)
		m.refresh()
	case "e":
		if s := m.selectedSite(); s != nil {
			s.Enabled = !s.Enabled
			_ = m.reg.Save()
			if s.Enabled {
				m.status = "enabled " + s.Slug
			} else {
				m.status = "disabled " + s.Slug
			}
			m.refresh()
		}
	case "X":
		for _, s := range m.reg.Sites {
			_ = supervisor.Stop(s.Slug, m.st)
		}
		m.status = "stopped all"
		m.refresh()
	case "o":
		if s := m.selectedSite(); s != nil {
			_ = openBrowser(m.siteURL(*s))
			m.status = "opened " + s.Slug
		}
	case "S":
		added, _ := scan.Scan(m.settings.ProjectsDir, m.reg, m.settings)
		for i := range m.reg.Sites {
			if m.reg.Sites[i].Launcher == "" {
				if res, err := launcher.Resolve(m.reg.Sites[i], m.settings); err == nil {
					m.reg.Sites[i].Launcher = res.Kind
				}
			}
		}
		_ = m.reg.Save()
		m.status = fmt.Sprintf("scan: +%d site(s)", len(added))
		m.refresh()
	case "p":
		if m.proxyRunning {
			_ = proxy.StopBackground()
			m.status = "proxy stopped"
		} else {
			_ = proxy.StartBackground(m.settings)
			m.status = "proxy started"
		}
		m.refresh()
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
		b.WriteString(dimStyle.Render("→ ") + m.siteURL(*s) + "\n")
	}

	// Proxy line.
	if m.proxyRunning {
		b.WriteString(okStyle.Render("● proxy on") + dimStyle.Render(fmt.Sprintf(" :%d  *.%s", m.settings.ProxyPort, m.settings.DomainSuffix)))
	} else {
		b.WriteString(offStyle.Render("○ proxy off") + dimStyle.Render("  press p to start"))
	}
	if m.status != "" {
		b.WriteString("   " + statusStyle.Render(m.status))
	}
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("s start · x stop · r restart · a all · X stop-all · e en/disable · l logs · o open · p proxy · S scan · q quit"))
	return b.String()
}

func (m *model) siteURL(s config.Site) string {
	return fmt.Sprintf("http://%s.%s:%d/", s.Slug, m.settings.DomainSuffix, m.settings.ProxyPort)
}

func dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func fmtDur(d time.Duration) string {
	d = d.Round(time.Second)
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	default:
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return exec.Command("xdg-open", url).Start()
	}
}

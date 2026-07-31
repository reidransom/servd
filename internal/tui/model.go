// Package tui implements servd's interactive dashboard (Bubble Tea).
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/registration"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// Run starts the TUI event loop.
func Run() error {
	m, err := newModel()
	if err != nil {
		return err
	}
	_, err = tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
	return err
}

type focus int

const (
	focusList focus = iota
	focusLog
)

type mode int

const (
	modeNormal mode = iota
	modeAdd
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
	focus    focus
	logSlug  string            // site the log panel is currently showing
	cmdCache map[string]string // slug -> resolved launch command (for stopped sites)
	viewport viewport.Model

	proxyRunning bool
	width        int
	height       int
	status       string // transient status line
	busy         bool   // an async action is in flight
	showHelp     bool   // help bar visible (toggled with h)

	mode       mode            // normal dashboard vs. the add-site modal
	addInput   textinput.Model // path entry for the add-site modal
	addMatches []string        // last tab-completion candidates, shown under the field
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	okStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	offStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	followStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	pausedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	boxStyle      = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	boxFocusStyle = boxStyle.BorderForeground(lipgloss.Color("#7aa2f7"))
)

// box returns the bordered-box style for a pane, highlighted when focused.
func box(focused bool) lipgloss.Style {
	if focused {
		return boxFocusStyle
	}
	return boxStyle
}

func newModel() (*model, error) {
	settings, reg, st, err := app.Load()
	if err != nil {
		return nil, err
	}
	cols := []table.Column{
		{Title: "SLUG", Width: 16},
		{Title: "PORT", Width: 5},
		{Title: "", Width: 2}, // status glyph
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

	m := &model{settings: settings, reg: reg, st: st, table: t, cmdCache: map[string]string{}, viewport: viewport.New(80, 20), showHelp: true}
	m.applyStatuses(buildStatuses(reg, st))
	m.syncLogSelection()
	return m, nil
}

// buildStatuses computes the table rows for a registry+state snapshot. It
// dials ports (StatusOf), so it must run off the Update goroutine.
func buildStatuses(reg *config.Registry, st *state.State) statusesMsg {
	running, _ := proxy.Running(st)
	rows := make([]table.Row, 0, len(reg.Sites))
	slugs := make([]string, 0, len(reg.Sites))
	for _, s := range reg.Sites {
		glyph := "○"
		switch supervisor.StatusOf(s, st) {
		case supervisor.Running:
			glyph = "●"
		case supervisor.Starting:
			glyph = "◐"
		}
		rows = append(rows, table.Row{s.Slug, strconv.Itoa(s.Port), glyph})
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

// resize recomputes the pane dimensions from the terminal size and which
// footer rows are visible. Called on WindowSizeMsg and when the help bar is
// toggled (hiding it gives its row back to the panes).
func (m *model) resize() {
	if m.width == 0 && m.height == 0 {
		return // no WindowSizeMsg yet; keep the constructor defaults
	}
	// Rows rendered outside the panes: title (1), footer detail (1), and the
	// help bar (1) when shown. Each bordered box also eats 2 rows (top+bottom
	// border), so the panes' inner content gets height minus all of that.
	chrome := 4
	if m.showHelp {
		chrome++
	}
	inner := max(5, m.height-chrome)
	m.table.SetHeight(inner)
	// The sidebar box hugs the table's rendered width; both boxes add 2 cols
	// of border, so the log viewport gets whatever's left.
	m.viewport.Width = max(20, m.width-m.sidebarWidth()-4)
	// One row inside the log box is the "$ command" header, so the viewport
	// gets inner-1 and both boxes still render `inner` content rows.
	m.viewport.Height = max(4, inner-1)
}

// sidebarWidth is the rendered width of the site-list table (fixed columns +
// lipgloss cell padding), used to size the log viewport so the two bordered
// boxes tile exactly across the terminal.
func (m *model) sidebarWidth() int {
	return lipgloss.Width(m.table.View())
}

// logCmd returns the shell command that launched (or would launch) the site
// shown in the log panel: the live command when it's running, otherwise the
// resolved launch command. Resolution hits the filesystem, so it's cached.
func (m *model) logCmd() string {
	if m.logSlug == "" {
		return ""
	}
	if e, ok := m.st.Get(m.logSlug); ok && e.Cmd != "" {
		return e.Cmd
	}
	if c, ok := m.cmdCache[m.logSlug]; ok {
		return c
	}
	c := ""
	if s := m.reg.Find(m.logSlug); s != nil {
		if res, err := launcher.Resolve(*s, m.settings); err == nil {
			c = res.Cmd
		}
	}
	m.cmdCache[m.logSlug] = c
	return c
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
		m.resize()
		return m, nil

	case tickMsg:
		m.syncLogSelection() // swap the panel if the highlighted slug changed
		m.loadLog()          // tail the selected site's log
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

	case tea.MouseMsg:
		return m.handleMouse(msg)
	}

	// Delegate to the focused widget.
	var cmd tea.Cmd
	if m.focus == focusLog {
		m.viewport, cmd = m.viewport.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}
	return m, cmd
}

// action marks the model busy with a status line and runs fn asynchronously,
// delivering its actionDoneMsg back into Update.
func (m *model) action(status string, fn func() actionDoneMsg) (tea.Model, tea.Cmd) {
	m.busy = true
	m.status = status
	return m, func() tea.Msg { return fn() }
}

// bulkAction applies do to each site, reporting the success count and the
// first error encountered.
func bulkAction(verb string, sites []config.Site, do func(config.Site) error) actionDoneMsg {
	n := 0
	var firstErr error
	for _, s := range sites {
		if err := do(s); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		n++
	}
	return actionDoneMsg{verb: verb, n: n, bulk: true, err: firstErr}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// The add-site modal captures all keys while it's open.
	if m.mode == modeAdd {
		return m.handleAddKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "A":
		m.mode = modeAdd
		ti := textinput.New()
		ti.Placeholder = "~/clients/newthing"
		ti.Prompt = ""
		ti.Focus()
		m.addInput = ti
		m.addMatches = nil
		m.status = ""
		return m, nil
	case "tab":
		if m.focus == focusList {
			m.focus = focusLog
		} else {
			m.focus = focusList
		}
		return m, nil
	case "h":
		m.showHelp = !m.showHelp
		m.resize()
		return m, nil
	case "s":
		if s := m.selectedSite(); s != nil && !m.busy {
			site, settings := *s, m.settings
			return m.action("starting "+site.Slug+"…", func() actionDoneMsg {
				return actionDoneMsg{verb: "started", slug: site.Slug, err: supervisor.Start(site, settings)}
			})
		}
	case "x":
		if s := m.selectedSite(); s != nil && !m.busy {
			slug := s.Slug
			return m.action("stopping "+slug+"…", func() actionDoneMsg {
				return actionDoneMsg{verb: "stopped", slug: slug, err: supervisor.Stop(slug)}
			})
		}
	case "r":
		if s := m.selectedSite(); s != nil && !m.busy {
			site, settings := *s, m.settings
			return m.action("restarting "+site.Slug+"…", func() actionDoneMsg {
				return actionDoneMsg{verb: "restarted", slug: site.Slug, err: supervisor.Restart(site, settings)}
			})
		}
	case "a":
		if !m.busy {
			var sites []config.Site
			for _, s := range m.reg.Sites {
				if s.Enabled {
					sites = append(sites, s)
				}
			}
			settings := m.settings
			return m.action("starting enabled sites…", func() actionDoneMsg {
				return bulkAction("started", sites, func(s config.Site) error { return supervisor.Start(s, settings) })
			})
		}
	case "X":
		if !m.busy {
			sites := append([]config.Site(nil), m.reg.Sites...)
			return m.action("stopping all…", func() actionDoneMsg {
				return bulkAction("stopped", sites, func(s config.Site) error { return supervisor.Stop(s.Slug) })
			})
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
	case "p":
		if m.busy {
			break
		}
		running, settings := m.proxyRunning, m.settings
		status, verb := "starting proxy…", "proxy started"
		if running {
			status, verb = "stopping proxy…", "proxy stopped"
		}
		return m.action(status, func() actionDoneMsg {
			var err error
			if running {
				err = proxy.StopBackground()
			} else {
				err = proxy.StartBackground(settings)
			}
			return actionDoneMsg{verb: verb, err: err}
		})
	}

	// Unhandled keys go to the focused widget.
	if m.focus == focusLog {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	m.syncLogSelection() // re-point the panel if the cursor moved
	return m, cmd
}

// handleAddKey drives the add-site modal: esc cancels, enter submits the typed
// path (deriving slug/port/command like `servd add <path>`), and every other
// key edits the path field.
func (m *model) handleAddKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeNormal
		m.status = ""
		return m, nil
	case "tab":
		completed, matches := completePath(m.addInput.Value())
		m.addInput.SetValue(completed)
		m.addInput.CursorEnd()
		// Only worth listing when the completion is ambiguous.
		if len(matches) > 1 {
			m.addMatches = matches
		} else {
			m.addMatches = nil
		}
		return m, nil
	case "enter":
		path := strings.TrimSpace(m.addInput.Value())
		if path == "" {
			m.status = "ERROR: a path is required"
			return m, nil
		}
		var site config.Site
		err := config.MutateRegistry(func(reg *config.Registry) error {
			var err error
			site, err = registration.AddSite(reg, m.settings, registration.AddParams{Path: expandHome(path)})
			return err
		})
		if err != nil {
			m.status = "ERROR: " + firstLine(err.Error())
			return m, nil // keep the modal open so the user can fix the path
		}
		m.mode = modeNormal
		m.cmdCache = map[string]string{}
		m.status = "added " + site.Slug
		return m, refreshCmd()
	}
	m.addMatches = nil // any edit invalidates the last completion list
	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

// completePath does shell-style filesystem completion of a partially-typed
// directory path (sites are directories, so files are ignored). It returns the
// path extended to the longest common prefix of the matching entries — with a
// trailing slash when the match is a single directory, so the user can keep
// descending — along with the list of matching entry names.
func completePath(input string) (string, []string) {
	p := expandHome(input)

	// Determine the directory to list and the prefix to match within it.
	dir, prefix := filepath.Dir(p), filepath.Base(p)
	if input == "" || strings.HasSuffix(input, "/") {
		dir, prefix = strings.TrimRight(p, "/"), ""
		if dir == "" {
			dir = "/"
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return input, nil
	}
	var matches []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		// Hide dotfiles unless the user has started typing one.
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}
		if strings.HasPrefix(name, prefix) {
			matches = append(matches, name)
		}
	}
	if len(matches) == 0 {
		return input, nil
	}

	lcp := matches[0]
	for _, m := range matches[1:] {
		lcp = commonPrefix(lcp, m)
	}
	completed := filepath.Join(dir, lcp)
	if len(matches) == 1 {
		completed += string(filepath.Separator) // unique dir: allow descending
	}
	return completed, matches
}

// commonPrefix returns the longest shared leading substring of a and b.
func commonPrefix(a, b string) string {
	n := min(len(a), len(b))
	i := 0
	for i < n && a[i] == b[i] {
		i++
	}
	return a[:i]
}

// expandHome replaces a leading ~ or ~/ with the user's home directory. The CLI
// relies on the shell for this; the TUI has no shell, so it expands here.
func expandHome(path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[1:])
		}
	}
	return path
}

// firstRowY is the terminal row of the first site row in the sidebar: the
// title (1) + the box's top border (1) + the table header (1) sit above it.
const firstRowY = 3

// handleMouse routes clicks and wheel events to the pane under the pointer:
// clicking a site row selects it (and shows its log), clicking either pane
// focuses it, and the wheel scrolls whichever pane it's over.
func (m *model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode == modeAdd {
		return m, nil // the modal owns the screen; ignore clicks underneath
	}
	overLog := msg.X >= m.sidebarWidth()

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if overLog {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		// The table itself ignores mouse events, so drive its cursor directly;
		// moving it re-points the log panel via syncLogSelection.
		if msg.Button == tea.MouseButtonWheelUp {
			m.table.MoveUp(1)
		} else {
			m.table.MoveDown(1)
		}
		m.syncLogSelection()
		return m, nil
	}

	if msg.Button != tea.MouseButtonLeft || msg.Action != tea.MouseActionPress {
		return m, nil
	}
	if overLog {
		m.focus = focusLog
		return m, nil
	}
	m.focus = focusList
	if idx := msg.Y - firstRowY; idx >= 0 && idx < len(m.slugs) {
		m.table.SetCursor(idx)
		m.syncLogSelection()
	}
	return m, nil
}

// syncLogSelection points the log panel at the highlighted site, resetting
// scroll to the bottom when the selection actually changed.
func (m *model) syncLogSelection() {
	slug := ""
	if s := m.selectedSite(); s != nil {
		slug = s.Slug
	}
	if slug == m.logSlug {
		return
	}
	m.logSlug = slug
	m.loadLog()
	m.viewport.GotoBottom()
}

// loadLog reads the selected site's logfile into the viewport. It preserves the
// user's scroll position unless they were already at the bottom, in which case
// it stays pinned there (tail/follow).
func (m *model) loadLog() {
	if m.logSlug == "" {
		m.viewport.SetContent(dimStyle.Render("(no site selected)"))
		return
	}
	follow := m.viewport.AtBottom()
	data, err := os.ReadFile(supervisor.LogPath(m.logSlug))
	if err != nil {
		m.viewport.SetContent(dimStyle.Render("(no logs yet for " + m.logSlug + ")"))
		return
	}
	m.viewport.SetContent(string(data))
	if follow {
		m.viewport.GotoBottom()
	}
}

func (m *model) View() string {
	if m.mode == modeAdd {
		return m.addView()
	}

	var b strings.Builder

	// Title row: name on the left, proxy status on the right.
	left := titleStyle.Render("servd") + dimStyle.Render(" — local dev servers")
	var right string
	if m.proxyRunning {
		right = okStyle.Render("● proxy on") + dimStyle.Render(fmt.Sprintf(" :%d %s", m.settings.Hostnames.HTTPPort, m.settings.PrimaryURLPattern()))
		if _, enabled := m.settings.FallbackURLPattern(); enabled {
			right += dimStyle.Render(" + nip.io")
		}
	} else {
		right = offStyle.Render("○ proxy off") + dimStyle.Render("  press p")
	}
	b.WriteString(rowLR(left, right, m.width) + "\n")

	// Panes: site list on the left, live log tail on the right.
	var sidebar string
	if len(m.reg.Sites) == 0 {
		hint := dimStyle.Render("No sites.\nPress A to add a site.")
		sidebar = box(m.focus == focusList).Width(m.sidebarWidth()).Height(m.viewport.Height).Render(hint)
	} else {
		sidebar = box(m.focus == focusList).Render(m.table.View())
	}
	// The log pane leads with the command that started the site, then its tail.
	cmd := m.logCmd()
	if cmd == "" {
		cmd = "(unknown)"
	}
	// Tail indicator: green LIVE when pinned to the bottom (new lines stream in),
	// amber scroll-percent when the user has scrolled back into history.
	var badge string
	if m.viewport.AtBottom() {
		badge = followStyle.Render("▼ LIVE")
	} else {
		badge = pausedStyle.Render(fmt.Sprintf("↑ %d%%", int(m.viewport.ScrollPercent()*100)))
	}
	cmdCell := lipgloss.NewStyle().MaxWidth(max(1, m.viewport.Width-lipgloss.Width(badge)-1)).
		Render(dimStyle.Render("$ ") + cmd)
	header := rowLR(cmdCell, badge, m.viewport.Width)
	logPane := box(m.focus == focusLog).Render(header + "\n" + m.viewport.View())
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, sidebar, logPane) + "\n")

	// Footer detail: selected-site URL + the launcher/uptime/enabled facts the
	// sidebar no longer shows, then any transient status message.
	if s := m.selectedSite(); s != nil {
		b.WriteString(dimStyle.Render("→ ") + m.settings.SiteURL(*s))
		var meta []string
		if s.Launcher != "" {
			meta = append(meta, s.Launcher)
		}
		if d := supervisor.Uptime(s.Slug, m.st); d > 0 {
			meta = append(meta, "up "+app.FmtDuration(d))
		}
		if !s.Enabled {
			meta = append(meta, "disabled")
		}
		if len(meta) > 0 {
			b.WriteString(dimStyle.Render("  (" + strings.Join(meta, " · ") + ")"))
		}
	}
	if m.status != "" {
		style := statusStyle
		if strings.HasPrefix(m.status, "ERROR:") {
			style = errStyle
		}
		b.WriteString("   " + style.Render(m.status))
	}
	if m.showHelp {
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("s start · x stop · r restart · a all · X stop-all · e en/dis · o open · p proxy · A add · tab focus · h help · q quit"))
	}
	return b.String()
}

// addView renders the centered add-site modal: a titled box with the path
// field, a hint line, and any error status.
func (m *model) addView() string {
	title := titleStyle.Render("add site")
	field := dimStyle.Render("path  ") + m.addInput.View()
	hint := helpStyle.Render("tab complete · enter add · esc cancel")
	body := title + "\n\n" + field + "\n\n" + hint
	// Show ambiguous tab-completion candidates (folder names only), truncated.
	if len(m.addMatches) > 0 {
		const maxShown = 8
		shown := m.addMatches
		suffix := ""
		if len(shown) > maxShown {
			shown = shown[:maxShown]
			suffix = fmt.Sprintf(" … +%d", len(m.addMatches)-maxShown)
		}
		body += "\n" + dimStyle.Render(strings.Join(shown, "  ")+suffix)
	}
	if strings.HasPrefix(m.status, "ERROR:") {
		body += "\n" + errStyle.Render(m.status)
	}
	modal := boxStyle.Width(max(40, m.width/2)).Render(body)
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

// rowLR lays out left- and right-justified segments across width, accounting
// for ANSI styling when measuring.
func rowLR(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}
	return left + strings.Repeat(" ", gap) + right
}

// firstLine truncates a (possibly multi-line) error message for the status bar.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

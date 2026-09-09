// Package tui implements servd's interactive dashboard (Bubble Tea).
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostnames"
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
	_, err = tea.NewProgram(m, tea.WithOutput(m.output), tea.WithAltScreen(), tea.WithMouseCellMotion()).Run()
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
	modeRename
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return tickMsg{} })
}

// actionDoneMsg reports the result of an async supervisor/proxy action.
type actionDoneMsg struct {
	verb    string // "started", "stopped", ...
	slug    string // empty for bulk actions
	n       int    // succeeded count, for bulk actions
	failed  int    // failed count, for bulk actions
	bulk    bool   // true when the action covered multiple sites
	err     error
	renamed bool // registry rename succeeded, even if restart failed
}

type model struct {
	settings config.Settings
	reg      *config.Registry
	st       *state.State

	statuses         map[string]supervisor.SiteStatus
	table            table.Model
	rows             []table.Row
	slugs            []string // keys parallel to rows; the table renders only the visible window
	rowOffset        int      // first visible row in the full snapshot
	focus            focus
	logSlug          string            // site or proxy key shown in the log panel
	cmdCache         map[string]string // slug -> resolved next launch command
	pendingSelection string            // rename target to follow when a snapshot observes it
	cmdErrors        map[string]error  // slug -> next launch resolution error
	viewport         viewport.Model
	selection        *textSelection
	output           *terminalOutput

	proxyRunning bool
	width        int
	height       int
	status       string // transient status line
	busy         bool   // an async action is in flight
	showHelp     bool   // help bar visible (toggled with h)

	mode        mode            // normal dashboard vs. the add-site modal
	addInput    textinput.Model // path entry for the add-site modal
	addMatches  []string        // last tab-completion candidates, shown under the field
	renameInput textinput.Model
	renameFrom  string
}

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7aa2f7"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	helpStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	followStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("42"))
	pausedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#e0af68"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bb9af7"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))

	boxStyle                = lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).BorderForeground(lipgloss.Color("240"))
	boxFocusStyle           = boxStyle.BorderForeground(lipgloss.Color("#7aa2f7"))
	sidebarSelectedStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("0")).Background(lipgloss.Color("#7aa2f7"))
	sidebarSelectedErrStyle = sidebarSelectedStyle.Foreground(lipgloss.Color("203"))
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
		{Title: "", Width: 1}, // status glyph
		{Title: "", Width: 19},
	}
	t := table.New(
		table.WithColumns(cols),
		table.WithFocused(true),
		table.WithHeight(13), // includes the header row omitted by sidebarTableView
	)
	s := table.DefaultStyles()
	s.Header = s.Header.Bold(true).Foreground(lipgloss.Color("#7aa2f7")).BorderBottom(true).PaddingRight(0)
	s.Cell = s.Cell.PaddingRight(0)
	s.Selected = sidebarSelectedStyle
	t.SetStyles(s)

	m := &model{settings: settings, reg: reg, st: st, table: t, cmdCache: map[string]string{}, cmdErrors: map[string]error{}, viewport: viewport.New(80, 20), showHelp: true}
	m.output = &terminalOutput{File: os.Stdout}
	m.applyStatuses(buildStatuses(settings, reg, st))
	return m, nil
}

// refreshCmd reloads settings, registry, and state before computing statuses.
func refreshCmd(_ config.Settings) tea.Cmd {
	return func() tea.Msg {
		settings, source, err := config.LoadSettingsWithSource()
		if err != nil {
			return statusesMsg{}
		}
		if !source.ConfigPresent {
			settings.Hostnames.HTTPPort = 80
		}
		reg, err := config.LoadRegistry()
		if err != nil {
			return statusesMsg{}
		}
		st, err := state.Load()
		if err != nil {
			return statusesMsg{}
		}
		return buildStatuses(settings, reg, st)
	}
}

// applyStatuses swaps in a fresh snapshot, preserving the selected row by key.
func (m *model) applyStatuses(msg statusesMsg) {
	if msg.reg == nil {
		return
	}
	selected, cur := m.selectedSlug(), m.rowOffset+m.table.Cursor()
	m.settings, m.reg, m.st = msg.settings, msg.reg, msg.st
	m.proxyRunning = msg.proxyRunning
	m.cmdCache = map[string]string{}
	m.cmdErrors = map[string]error{}
	m.statuses = msg.statuses
	m.rows, m.slugs = msg.rows, msg.slugs
	if idx := slices.Index(m.slugs, selected); idx >= 0 {
		cur = idx
	}
	if m.pendingSelection != "" {
		if idx := slices.Index(m.slugs, m.pendingSelection); idx >= 0 {
			cur = idx
			m.pendingSelection = ""
		}
	}
	m.selectRow(cur)
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
	cur := m.rowOffset + m.table.Cursor()
	m.table.SetHeight(inner + 1) // table height includes its omitted header row
	// The sidebar box hugs the table's rendered width; both boxes add 2 cols
	// of border, so the log viewport gets whatever's left.
	m.viewport.Width = max(20, m.width-m.sidebarWidth()-4)
	// One row inside the log box is the "$ command" header, so the viewport
	// gets inner-1 and both boxes still render `inner` content rows.
	m.viewport.Height = max(4, inner-1)
	m.selectRow(cur)
}

// sidebarTableView removes the table component's mandatory header row and
// colors error glyphs after the table has truncated its plain-text cells.
// Bubbles' table uses an ANSI-unaware truncator, so styled cell values can
// become malformed escape sequences in narrow columns.
func (m *model) sidebarTableView() string {
	view := m.table.View()
	_, rows, ok := strings.Cut(view, "\n")
	if !ok {
		rows = view
	}
	lines := strings.Split(rows, "\n")
	if len(m.slugs) == 1 && m.slugs[0] == proxy.Slug {
		for i, hint := range []string{"No sites.", "Press a to add a site."} {
			if i+1 < len(lines) {
				lines[i+1] = dimStyle.Width(lipgloss.Width(lines[0])).Render(hint)
			}
		}
	}
	for i, line := range lines {
		plain := ansi.Strip(line)
		glyph := strings.Index(plain, "✕")
		if glyph < 0 {
			continue
		}
		before, after := plain[:glyph], plain[glyph+len("✕"):]
		if line == plain {
			lines[i] = before + errStyle.Render("✕") + after
			continue
		}
		lines[i] = sidebarSelectedStyle.Render(before) +
			sidebarSelectedErrStyle.Render("✕") +
			sidebarSelectedStyle.Render(after)
	}
	return strings.Join(lines, "\n")
}

// sidebarWidth is the rendered width of the site-list table (fixed columns +
// lipgloss cell padding), used to size the log viewport so the two bordered
// boxes tile exactly across the terminal.
func (m *model) sidebarWidth() int {
	return lipgloss.Width(m.sidebarTableView())
}

// logCmd returns the next resolved launch command for the selected site.
// Resolution hits the filesystem, so it is cached until the next refresh.
func (m *model) logCmd() (string, error) {
	if m.logSlug == "" || m.logSlug == proxy.Slug {
		return "", nil
	}
	if err, ok := m.cmdErrors[m.logSlug]; ok {
		return "", err
	}
	if c, ok := m.cmdCache[m.logSlug]; ok {
		return c, nil
	}
	if m.cmdErrors == nil {
		m.cmdErrors = map[string]error{}
	}
	if m.cmdCache == nil {
		m.cmdCache = map[string]string{}
	}
	if s := m.reg.Find(m.logSlug); s != nil {
		res, err := launcher.Resolve(*s, m.settings)
		if err != nil {
			m.cmdErrors[m.logSlug] = err
			return "", err
		}
		m.cmdCache[m.logSlug] = res.Cmd
		return res.Cmd, nil
	}
	return "", nil
}

func (m *model) selectedSlug() string {
	idx := m.rowOffset + m.table.Cursor()
	if idx < 0 || idx >= len(m.slugs) {
		return ""
	}
	return m.slugs[idx]
}

func (m *model) selectedSite() *config.Site {
	slug := m.selectedSlug()
	if slug == "" || slug == proxy.Slug {
		return nil
	}
	return m.reg.Find(slug)
}

// selectRow owns scrolling so rendering and mouse hit-testing share the same
// offset. Bubbles' table does not expose its internal viewport offset.
func (m *model) selectRow(index int) {
	index = max(0, min(index, len(m.rows)-1))
	height := max(1, m.table.Height())
	m.rowOffset = max(0, min(m.rowOffset, len(m.rows)-height))
	if index < m.rowOffset {
		m.rowOffset = index
	} else if index >= m.rowOffset+height {
		m.rowOffset = index - height + 1
	}
	m.table.SetRows(m.rows[m.rowOffset:min(len(m.rows), m.rowOffset+height)])
	m.table.SetCursor(index - m.rowOffset)
	m.syncLogSelection()
}

// allSitesRunning reports whether every registered site has a live process.
func (m *model) allSitesRunning() bool {
	if len(m.reg.Sites) == 0 {
		return false
	}
	for _, site := range m.reg.Sites {
		entry, ok := m.st.Get(site.Slug)
		if !ok || !state.EntryAlive(entry) {
			return false
		}
	}
	return true
}

func (m *model) Init() tea.Cmd { return tick() }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.selection = nil
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil

	case tickMsg:
		m.syncLogSelection() // swap the panel if the highlighted slug changed
		m.loadLog()          // tail the selected site's log
		return m, tea.Batch(refreshCmd(m.settings), tick())

	case statusesMsg:
		m.applyStatuses(msg)
		return m, nil

	case actionDoneMsg:
		m.busy = false
		if msg.err != nil && !msg.renamed {
			m.pendingSelection = ""
		}
		switch {
		case msg.bulk:
			m.status = fmt.Sprintf("%s %d sites; %d failed", strings.ToUpper(msg.verb[:1])+msg.verb[1:], msg.n, msg.failed)
		case msg.err != nil:
			m.status = "ERROR: " + firstLine(msg.err.Error())
		case msg.slug != "":
			m.status = msg.verb + " " + msg.slug
		default:
			m.status = msg.verb
		}
		return m, refreshCmd(m.settings)

	case clipboardDoneMsg:
		if msg.err != nil {
			m.selection = nil
			m.status = "ERROR: copying to clipboard: " + firstLine(msg.err.Error())
		}
		return m, nil

	case tea.KeyMsg:
		m.selection = nil
		return m.handleKey(msg)

	case tea.MouseMsg:
		return m.handleSelectionMouse(msg)
	}

	// Delegate to the focused widget.
	var cmd tea.Cmd
	if m.focus == focusLog {
		m.viewport, cmd = m.viewport.Update(msg)
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

// bulkAction applies do to each site, reporting both outcome counts.
func bulkAction(verb string, sites []config.Site, do func(config.Site) error) actionDoneMsg {
	n := 0
	failed := 0
	for _, s := range sites {
		if err := do(s); err != nil {
			failed++
			continue
		}
		n++
	}
	return actionDoneMsg{verb: verb, n: n, failed: failed, bulk: true}
}

func (m *model) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Modals capture all keys while open.
	switch m.mode {
	case modeAdd:
		return m.handleAddKey(msg)
	case modeRename:
		return m.handleRenameKey(msg)
	}
	switch msg.String() {
	case "q", "ctrl+c":
		return m, tea.Quit
	case "S":
		if !m.busy {
			sites := append([]config.Site(nil), m.reg.Sites...)
			if m.allSitesRunning() {
				return m.action("stopping all sites…", func() actionDoneMsg {
					return bulkAction("stopped", sites, func(s config.Site) error { return supervisor.Stop(s.Slug) })
				})
			}
			settings := m.settings
			return m.action("starting all sites…", func() actionDoneMsg {
				return bulkAction("started", sites, func(s config.Site) error { return supervisor.Start(s, settings) })
			})
		}
	case "a":
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
		if m.selectedSlug() == proxy.Slug && !m.busy {
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
					_, err = proxy.StartBackground(settings)
				}
				return actionDoneMsg{verb: verb, err: err}
			})
		}
		if s := m.selectedSite(); s != nil && !m.busy {
			site, settings := *s, m.settings
			if entry, ok := m.st.Get(site.Slug); ok && state.EntryAlive(entry) {
				return m.action("stopping "+site.Slug+"…", func() actionDoneMsg {
					return actionDoneMsg{verb: "stopped", slug: site.Slug, err: supervisor.Stop(site.Slug)}
				})
			}
			return m.action("starting "+site.Slug+"…", func() actionDoneMsg {
				return actionDoneMsg{verb: "started", slug: site.Slug, err: supervisor.Start(site, settings)}
			})
		}
	case "r":
		if s := m.selectedSite(); s != nil && !m.busy {
			m.mode = modeRename
			m.renameFrom = s.Slug
			ti := textinput.New()
			ti.Prompt = ""
			ti.SetValue(s.Slug)
			ti.CursorEnd()
			ti.Focus()
			m.renameInput = ti
			m.status = ""
			return m, nil
		}
	case "R":
		if s := m.selectedSite(); s != nil && !m.busy {
			site, settings := *s, m.settings
			return m.action("restarting "+site.Slug+"…", func() actionDoneMsg {
				return actionDoneMsg{verb: "restarted", slug: site.Slug, err: supervisor.Restart(site, settings)}
			})
		}
	case "d":
		if s := m.selectedSite(); s != nil && !m.busy {
			slug := s.Slug
			return m.action("removing "+slug+"…", func() actionDoneMsg {
				// Keep the CLI's removal semantics: remove registry state even if
				// an already-unhealthy process cannot be stopped.
				_ = supervisor.Stop(slug)
				return actionDoneMsg{verb: "removed", slug: slug, err: config.MutateRegistry(func(reg *config.Registry) error {
					return registration.RemoveSite(reg, slug)
				})}
			})
		}
		return m, nil
	case "o", "c":
		url, label := "", ""
		if m.selectedSlug() == proxy.Slug {
			url, label = m.proxyURL(), "proxy"
		} else if s := m.selectedSite(); s != nil {
			url, label = m.settings.SiteURL(*s), s.Slug
		}
		if url != "" {
			if msg.String() == "c" {
				return m, m.output.copy(url)
			}
			if err := app.OpenBrowser(url); err != nil {
				m.status = "ERROR: " + firstLine(err.Error())
			} else {
				m.status = "opened " + label
			}
		}
	}

	// Unhandled keys go to the focused widget.
	if m.focus == focusLog {
		var cmd tea.Cmd
		m.viewport, cmd = m.viewport.Update(msg)
		return m, cmd
	}
	cur := m.rowOffset + m.table.Cursor()
	switch {
	case key.Matches(msg, m.table.KeyMap.LineUp):
		cur--
	case key.Matches(msg, m.table.KeyMap.LineDown):
		cur++
	case key.Matches(msg, m.table.KeyMap.PageUp):
		cur -= m.table.Height()
	case key.Matches(msg, m.table.KeyMap.PageDown):
		cur += m.table.Height()
	case key.Matches(msg, m.table.KeyMap.HalfPageUp):
		cur -= m.table.Height() / 2
	case key.Matches(msg, m.table.KeyMap.HalfPageDown):
		cur += m.table.Height() / 2
	case key.Matches(msg, m.table.KeyMap.GotoTop):
		cur = 0
	case key.Matches(msg, m.table.KeyMap.GotoBottom):
		cur = len(m.rows) - 1
	default:
		return m, nil
	}
	m.selectRow(cur)
	return m, nil
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
		m.cmdErrors = map[string]error{}
		m.status = "added " + site.Slug
		return m, refreshCmd(m.settings)
	}
	m.addMatches = nil // any edit invalidates the last completion list
	var cmd tea.Cmd
	m.addInput, cmd = m.addInput.Update(msg)
	return m, cmd
}

// completePath does shell-style filesystem completion of a partially-typed
// directory path (sites are directories, so files are ignored). It returns the
// path extended to the longest common prefix of the matching entries — with a
// trailing path separator when the match is a single directory, so the user can
// keep descending — along with the list of matching entry names.
func completePath(input string) (string, []string) {
	p := expandHome(input)

	// Determine the directory to list and the prefix to match within it.
	dir, prefix := filepath.Dir(p), filepath.Base(p)
	if input == "" {
		dir, prefix = string(filepath.Separator), ""
	} else if strings.HasSuffix(input, "/") || strings.HasSuffix(input, string(filepath.Separator)) {
		dir, prefix = filepath.Clean(p), ""
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

// firstRowY is zero-based: the title and top border precede sidebar content.
const firstRowY = 2

// handleMouse routes clicks and wheel events to the pane under the pointer:
// clicking a server row selects it (and shows its log), clicking either pane
// focuses it, and the wheel scrolls whichever pane it's over.
func (m *model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if m.mode != modeNormal {
		return m, nil // the modal owns the screen; ignore clicks underneath
	}
	sidebarWidth := m.sidebarWidth()
	insideRows := msg.Y >= firstRowY && msg.Y < firstRowY+m.table.Height()
	overList := insideRows && msg.X >= 1 && msg.X < sidebarWidth+1
	overLog := insideRows && msg.X >= sidebarWidth+3 && msg.X < sidebarWidth+3+m.viewport.Width
	if !overList && !overLog {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseButtonWheelUp, tea.MouseButtonWheelDown:
		if overLog {
			var cmd tea.Cmd
			m.viewport, cmd = m.viewport.Update(msg)
			return m, cmd
		}
		cur := m.rowOffset + m.table.Cursor()
		if msg.Button == tea.MouseButtonWheelUp {
			m.selectRow(cur - 1)
		} else {
			m.selectRow(cur + 1)
		}
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
	if idx := m.rowOffset + msg.Y - firstRowY; idx < len(m.rows) {
		m.selectRow(idx)
	}
	return m, nil
}

// syncLogSelection points the log panel at the highlighted row, resetting
// scroll to the bottom when the selection actually changed.
func (m *model) syncLogSelection() {
	slug := m.selectedSlug()
	if slug == m.logSlug {
		return
	}
	m.pendingSelection = ""
	m.logSlug = slug
	m.loadLog()
	m.viewport.GotoBottom()
}

// loadLog reads the selected row's logfile into the viewport. It preserves the
// user's scroll position unless they were already at the bottom, in which case
// it stays pinned there (tail/follow).
func (m *model) loadLog() {
	if m.logSlug == "" {
		m.viewport.SetContent(dimStyle.Render("(no server selected)"))
		return
	}
	follow := m.viewport.AtBottom()
	data, err := os.ReadFile(supervisor.LogPath(m.logSlug))
	if err != nil {
		label := m.logSlug
		if label == proxy.Slug {
			label = "proxy"
		}
		m.viewport.SetContent(dimStyle.Render("(no logs yet for " + label + ")"))
		return
	}
	m.viewport.SetContent(string(data))
	if follow {
		m.viewport.GotoBottom()
	}
}

func (m *model) proxyURL() string {
	return hostnames.FormatURL(m.settings.BindHost, m.settings.Hostnames.HTTPPort, false) + "/"
}

func (m *model) View() string {
	if m.selection != nil {
		return m.selection.view()
	}
	switch m.mode {
	case modeAdd:
		return m.addView()
	case modeRename:
		return m.renameView()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("servd") + dimStyle.Render(" — local dev servers") + "\n")

	sidebar := box(m.focus == focusList).Render(m.sidebarTableView())
	// Sites show their next command; the proxy has no site launch command.
	logTitle := dimStyle.Render("proxy log")
	if m.logSlug != proxy.Slug {
		cmd, cmdErr := m.logCmd()
		if cmdErr != nil {
			cmd = errStyle.Render("ERROR: " + firstLine(cmdErr.Error()))
		} else if cmd == "" {
			cmd = "(unknown)"
		}
		logTitle = dimStyle.Render("$ ") + cmd
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
		Render(logTitle)
	header := rowLR(cmdCell, badge, m.viewport.Width)
	logPane := box(m.focus == focusLog).Render(header + "\n" + m.viewport.View())
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, sidebar, logPane) + "\n")

	// Footer detail: selected-server URL and any transient status message.
	if m.selectedSlug() == proxy.Slug {
		b.WriteString(dimStyle.Render("→ ") + m.proxyURL())
	} else if s := m.selectedSite(); s != nil {
		b.WriteString(dimStyle.Render("→ ") + m.settings.SiteURL(*s))
		if health, ok := m.statuses[s.Slug]; ok && health.Kind == supervisor.Error {
			b.WriteString("   " + errStyle.Render("ERROR: "+health.Reason))
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
		help := "s start/stop"
		if m.selectedSite() != nil {
			help += " · r rename · R restart · d remove"
		}
		help += " · S start/stop-all · a add · o open · c copy URL · tab focus · h help · q quit"
		b.WriteString(helpStyle.Render(help))
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

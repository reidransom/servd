package tui

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// TestAddModalRegistersSite drives the add-site modal end to end: typing a path
// and pressing enter should register the site and close the modal.
func TestAddModalRegistersSite(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	proj := filepath.Join(t.TempDir(), "widget")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, ".servd.toml"), []byte(`cmd = "serve"`), 0o644); err != nil {
		t.Fatal(err)
	}

	ti := textinput.New()
	ti.SetValue(proj)
	m := &model{
		settings: config.Settings{PortRangeStart: 42201, BindHost: "127.0.0.1", Hostnames: config.HostnameSettings{TLD: "localhost", HTTPPort: 42200}},
		reg:      &config.Registry{},
		mode:     modeAdd,
		addInput: ti,
		cmdCache: map[string]string{},
	}

	if _, cmd := m.handleAddKey(tea.KeyMsg{Type: tea.KeyEnter}); cmd == nil {
		t.Fatal("expected a refresh command after add")
	}
	if m.mode != modeNormal {
		t.Errorf("mode = %v, want modeNormal (modal should close)", m.mode)
	}

	reg, err := config.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if s := reg.Find("widget"); s == nil {
		t.Fatalf("site not registered; registry = %+v", reg.Sites)
	}
	data, err := os.ReadFile(config.RegistryPath())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "enabled") {
		t.Fatalf("registered site persists removed enabled field:\n%s", data)
	}
}

// TestRemoveKeyRemovesSelectedSite drives the dashboard shortcut through its
// asynchronous completion and verifies the registry no longer contains it.
func TestRemoveKeyRemovesSelectedSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	if err := (&config.Registry{Sites: []config.Site{{Slug: "widget", Port: 4242}}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	selectDashboardSite(t, m, "widget")

	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}); cmd == nil {
		t.Fatal("expected a remove command")
	} else {
		m.Update(cmd())
	}

	reg, err := config.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Find("widget") != nil {
		t.Fatalf("site still registered: %+v", reg.Sites)
	}
}

func TestStartStopKeyTogglesSelectedSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	site := config.Site{Slug: "widget", Path: t.TempDir(), Port: 4242, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{site}}).Save(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = supervisor.Stop(site.Slug) })

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	selectDashboardSite(t, m, "widget")

	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Fatal("expected a start command")
	} else {
		m.Update(cmd())
	}
	assertSiteProcesses(t, true, site.Slug)

	m.Update(refreshCmd(m.settings)())
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Fatal("expected a stop command")
	} else {
		m.Update(cmd())
	}
	assertSiteProcesses(t, false, site.Slug)
}

func TestAllKeyTogglesSites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	sites := []config.Site{
		{Slug: "widget", Path: t.TempDir(), Port: 4242, Cmd: "sleep 30"},
		{Slug: "gadget", Path: t.TempDir(), Port: 4243, Cmd: "sleep 30"},
	}
	if err := (&config.Registry{Sites: sites}).Save(); err != nil {
		t.Fatal(err)
	}
	for _, site := range sites {
		t.Cleanup(func() { _ = supervisor.Stop(site.Slug) })
	}

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}

	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")}); cmd == nil {
		t.Fatal("expected a start-all command")
	} else {
		m.Update(cmd())
	}
	assertSiteProcesses(t, true, "widget", "gadget")

	m.Update(refreshCmd(m.settings)())
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")}); cmd == nil {
		t.Fatal("expected a stop-all command")
	} else {
		m.Update(cmd())
	}
	assertSiteProcesses(t, false, "widget", "gadget")
}

func TestAddKeyOpensModal(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	if m.mode != modeAdd {
		t.Errorf("mode = %v, want modeAdd", m.mode)
	}
}

// TestHelpToggle checks that h hides the help bar and gives its row to the
// panes, and that a second press restores both.
func TestHelpToggle(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	if !strings.Contains(m.View(), "s start/stop") {
		t.Fatal("help bar missing from initial view")
	}
	shown := m.table.Height()

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if strings.Contains(m.View(), "s start/stop") {
		t.Error("help bar still visible after h")
	}
	if got := m.table.Height(); got != shown+1 {
		t.Errorf("table height = %d after hiding help, want %d", got, shown+1)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if !strings.Contains(m.View(), "s start/stop") {
		t.Error("help bar not restored by second h")
	}
	if got := m.table.Height(); got != shown {
		t.Errorf("table height = %d after restoring help, want %d", got, shown)
	}
}

func TestViewOmitsWebsiteEnablement(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	registry := &config.Registry{Sites: []config.Site{{Slug: "widget", Path: t.TempDir(), Port: 1}}}
	m.applyStatuses(buildStatuses(m.settings, registry, m.st))
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})

	view := m.View()
	for _, text := range []string{"en/dis", "disabled"} {
		if strings.Contains(view, text) {
			t.Errorf("TUI view exposes removed enablement %q:\n%s", text, view)
		}
	}
}

func TestCompletePath(t *testing.T) {
	root := t.TempDir()
	for _, d := range []string{"alpha", "alpine", "beta", ".hidden"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "afile"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	// Unique directory prefix → completed to the full name with a trailing sep.
	if got, m := completePath(filepath.Join(root, "be")); got != filepath.Join(root, "beta")+string(os.PathSeparator) || len(m) != 1 {
		t.Errorf("unique: got %q matches %v", got, m)
	}
	// Ambiguous prefix → longest common prefix, both candidates returned. Note
	// "afile" is a file and must be ignored, so "a" matches only alpha/alpine.
	got, m := completePath(filepath.Join(root, "a"))
	if got != filepath.Join(root, "alp") {
		t.Errorf("ambiguous: got %q, want .../alp", got)
	}
	if len(m) != 2 {
		t.Errorf("ambiguous: got matches %v, want alpha+alpine", m)
	}
	// Trailing slash → list every visible subdir (dotfiles hidden).
	if _, m := completePath(root + string(os.PathSeparator)); len(m) != 3 {
		t.Errorf("list: got %d matches %v, want 3 (dotfile hidden)", len(m), m)
	}
	// A leading dot in the prefix reveals dotfiles.
	if got, m := completePath(filepath.Join(root, ".h")); len(m) != 1 || got != filepath.Join(root, ".hidden")+string(os.PathSeparator) {
		t.Errorf("dotfile: got %q matches %v", got, m)
	}
}

func TestAddModalTabCompletes(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "widget"), 0o755); err != nil {
		t.Fatal(err)
	}

	ti := textinput.New()
	ti.SetValue(filepath.Join(root, "wid"))
	m := &model{mode: modeAdd, addInput: ti, cmdCache: map[string]string{}}

	m.handleAddKey(tea.KeyMsg{Type: tea.KeyTab})
	if got, want := m.addInput.Value(), filepath.Join(root, "widget")+string(os.PathSeparator); got != want {
		t.Errorf("after tab: value = %q, want %q", got, want)
	}
}

// TestAddModalKeepsOpenOnError leaves the modal open when the path can't be
// added (here: empty input).
func TestAddModalKeepsOpenOnError(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	ti := textinput.New()
	m := &model{
		settings: config.Settings{PortRangeStart: 42201, BindHost: "127.0.0.1", Hostnames: config.HostnameSettings{TLD: "localhost", HTTPPort: 42200}},
		reg:      &config.Registry{},
		mode:     modeAdd,
		addInput: ti,
		cmdCache: map[string]string{},
	}

	m.handleAddKey(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeAdd {
		t.Errorf("mode = %v, want modeAdd (modal should stay open on error)", m.mode)
	}
	if m.status == "" {
		t.Error("want an error status, got empty")
	}
}

// TestSiteListOmitsPorts keeps backend ports out of the default TUI display.
func TestSiteListOmitsPorts(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	reg := &config.Registry{Sites: []config.Site{{Slug: "widget", Port: 4242}}}
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	view := m.View()
	if !strings.Contains(view, "widget") {
		t.Fatal("site slug missing from TUI")
	}
	if strings.Contains(view, "PORT") || strings.Contains(view, "4242") {
		t.Errorf("TUI shows a port:\n%s", view)
	}
}

func TestLogHeaderRefreshesNextCommandAndResolutionErrors(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	project := t.TempDir()
	configPath := filepath.Join(project, ".servd.toml")
	if err := os.WriteFile(configPath, []byte(`cmd = "echo next"`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&config.Registry{Sites: []config.Site{{Slug: "site", Path: project, Port: 4011}}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	selectDashboardSite(t, m, "site")
	if view := ansi.Strip(m.View()); !strings.Contains(view, "$ echo next") {
		t.Fatalf("next command missing from log header:\n%s", view)
	}
	if err := os.WriteFile(configPath, []byte("cmd ="), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(refreshCmd(m.settings)())
	if view := ansi.Strip(m.View()); !strings.Contains(view, "ERROR:") || strings.Contains(view, "$ echo next") {
		t.Fatalf("invalid next command did not replace cached header:\n%s", view)
	}
	if err := os.WriteFile(configPath, []byte(`cmd = "echo repaired"`), 0o644); err != nil {
		t.Fatal(err)
	}
	m.Update(refreshCmd(m.settings)())
	if view := ansi.Strip(m.View()); !strings.Contains(view, "$ echo repaired") || strings.Contains(view, "ERROR:") {
		t.Fatalf("repaired next command did not clear cached error:\n%s", view)
	}
}

func TestRenameAndRestartKeys(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	site := config.Site{Slug: "widget", Path: t.TempDir(), Port: 4242, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{site}}).Save(); err != nil {
		t.Fatal(err)
	}
	if err := supervisor.Start(site, config.DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = supervisor.Stop("widget")
		_ = supervisor.Stop("gadget")
	})

	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	selectDashboardSite(t, m, "widget")

	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}); cmd != nil {
		t.Fatal("rename key unexpectedly returned a command before input")
	}
	if m.mode != modeRename || m.renameInput.Value() != "widget" {
		t.Fatalf("rename modal = mode %v value %q", m.mode, m.renameInput.Value())
	}
	m.renameInput.SetValue("gadget")
	_, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("rename submission returned no command")
	}
	m.Update(cmd())

	reg, err := config.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if reg.Find("widget") != nil || reg.Find("gadget") == nil {
		t.Fatalf("registry after rename = %+v", reg.Sites)
	}
	runtime, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	before, ok := runtime.Get("gadget")
	if !ok || !state.EntryAlive(before) {
		t.Fatalf("renamed site is not running: %+v", runtime.Entries)
	}
	if _, ok := runtime.Get("widget"); ok {
		t.Fatalf("old runtime entry remains: %+v", runtime.Entries)
	}

	m.Update(refreshCmd(m.settings)())
	_, cmd = m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("R")})
	if cmd == nil {
		t.Fatal("restart key returned no command")
	}
	m.Update(cmd())
	runtime, err = state.Load()
	if err != nil {
		t.Fatal(err)
	}
	after, ok := runtime.Get("gadget")
	if !ok || !state.EntryAlive(after) || !after.StartedAt.After(before.StartedAt) {
		t.Fatalf("restart did not replace runtime entry: before=%+v after=%+v", before, after)
	}
}

func TestProxySelectionWithoutSites(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	logPath := supervisor.LogPath(proxy.Slug)
	if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(logPath, []byte("proxy listener stopped\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	view := ansi.Strip(m.View())
	for _, want := range []string{"○", "Press a to add a site.", "proxy log", "proxy listener stopped", "→ http://servd.localhost/"} {
		if !strings.Contains(view, want) {
			t.Errorf("empty-registry dashboard missing %q:\n%s", want, view)
		}
	}
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	if got := ansi.Strip(m.View()); got != view {
		t.Errorf("moving down selected the nonselectable add-site hint:\n%s", got)
	}
}

func TestRefreshKeepsSelectionAndLogsTogether(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	alpha := config.Site{Slug: "alpha", Path: root, Port: 4011, Cmd: "sleep 30"}
	bravo := config.Site{Slug: "bravo", Path: root, Port: 4012, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{alpha, bravo}}).Save(); err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{proxy.Slug, alpha.Slug, bravo.Slug} {
		path := supervisor.LogPath(slug)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("log from "+slug+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	assertSelected := func(slug, url string) {
		t.Helper()
		view := ansi.Strip(m.View())
		if !strings.Contains(view, "log from "+slug) || !strings.Contains(view, "→ "+url) {
			t.Fatalf("selection and logs do not show %s:\n%s", slug, view)
		}
	}
	assertSelected("bravo", "http://bravo.localhost/")
	m.Update(buildStatuses(m.settings, &config.Registry{Sites: []config.Site{bravo, alpha}}, m.st))
	assertSelected("bravo", "http://bravo.localhost/")
	m.Update(buildStatuses(m.settings, &config.Registry{Sites: []config.Site{alpha}}, m.st))
	assertSelected("alpha", "http://alpha.localhost/")
	m.Update(buildStatuses(m.settings, &config.Registry{}, m.st))
	assertSelected(proxy.Slug, "http://servd.localhost/")
}

func selectDashboardSite(t *testing.T, m *model, slug string) {
	t.Helper()
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	for range len(m.reg.Sites) + 1 {
		if site := m.selectedSite(); site != nil && site.Slug == slug {
			return
		}
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
	}
	t.Fatalf("site %q is not selectable", slug)
}

func TestRenamePreservesSelectionAfterSorting(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	if err := (&config.Registry{Sites: []config.Site{
		{Slug: "middle", Path: root, Port: 4011, Cmd: "sleep 30"},
		{Slug: "widget", Path: root, Port: 4012, Cmd: "sleep 30"},
	}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	selectDashboardSite(t, m, "widget")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m.Update(tea.KeyMsg{Type: tea.KeyCtrlU})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("gadget")})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("rename did not submit")
	}
	result := cmd()
	// A periodic snapshot can observe the registry rename before the action
	// completion arrives, especially while a running site is restarting.
	m.Update(refreshCmd(m.settings)())
	_, refresh := m.Update(result)
	if refresh == nil {
		t.Fatal("rename did not refresh")
	}
	m.Update(refresh())
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "→ http://gadget.localhost/") || !strings.Contains(view, "no logs yet for gadget") {
		t.Fatalf("renamed site lost selection or log panel:\n%s", view)
	}
}

func TestProxyStartReportsBindFailure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	settings := config.DefaultSettings()
	settings.Hostnames.HTTPPort = listener.Addr().(*net.TCPAddr).Port
	if err := config.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if cmd == nil {
		t.Fatal("s did not attempt to start the selected proxy")
	}
	if _, duplicate := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); duplicate != nil {
		t.Fatal("s scheduled a second action while busy")
	}
	_, refresh := m.Update(cmd())
	if refresh == nil {
		t.Fatal("failed start did not refresh status")
	}
	m.Update(refresh())
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "ERROR:") || !strings.Contains(view, "could not bind configured proxy port") || !strings.Contains(view, "○") {
		t.Fatalf("failed proxy start did not stay stopped and report its error:\n%s", view)
	}
}

func TestProxySelectionRejectsSiteActionsAndGlobalProxyKey(t *testing.T) {
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
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	for _, key := range []string{"r", "R", "d", "p"} {
		before := m.View()
		if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}); cmd != nil {
			t.Fatalf("%s scheduled an action on the proxy", key)
		}
		if m.View() != before {
			t.Fatalf("%s changed the dashboard while the proxy was selected", key)
		}
	}
	for range 2 {
		for range 2 {
			before := m.View()
			if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}); cmd != nil || m.View() != before {
				t.Fatal("p still acts as a global proxy shortcut")
			}
			m.Update(tea.KeyMsg{Type: tea.KeyTab})
		}
		selectDashboardSite(t, m, "widget")
	}
}

func TestOpenReportsBrowserLaunchFailure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv("PATH", t.TempDir())
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 24})
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")})
	view := ansi.Strip(m.View())
	if !strings.Contains(view, "ERROR:") || strings.Contains(view, "opened proxy") {
		t.Fatalf("browser launch failure was not reported:\n%s", view)
	}
}

func TestSidebarClicksFollowVisibleRowsAfterScrolling(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	root := t.TempDir()
	sites := make([]config.Site, 24)
	for i := range sites {
		sites[i] = config.Site{Slug: fmt.Sprintf("site-%02d", i), Path: root, Port: 4100 + i, Cmd: "sleep 30"}
	}
	if err := (&config.Registry{Sites: sites}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 120, Height: 12})
	m.Update(tea.KeyMsg{Type: tea.KeyEnd})
	lines := strings.Split(ansi.Strip(m.View()), "\n")
	y := slices.IndexFunc(lines, func(line string) bool { return strings.Contains(line, "site-22") })
	if y < 0 {
		t.Fatalf("penultimate site is not visible at end:\n%s", m.View())
	}
	m.Update(tea.MouseMsg{X: 3, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m.Update(tea.MouseMsg{X: 3, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "→ http://site-22.localhost/") {
		t.Fatalf("click selected a different site than the visible row:\n%s", view)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyHome})
	m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m.Update(tea.MouseMsg{X: 3, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m.Update(tea.MouseMsg{X: 3, Y: 2, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	if view := ansi.Strip(m.View()); !strings.Contains(view, "→ http://servd.localhost/") {
		t.Fatalf("clicking the first sidebar row did not select the proxy:\n%s", view)
	}
	before := m.View()
	m.Update(tea.MouseMsg{X: 0, Y: 4, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress})
	m.Update(tea.MouseMsg{X: 3, Y: 0, Button: tea.MouseButtonWheelDown})
	if m.View() != before {
		t.Fatal("border click or title wheel event changed selection")
	}
}

func assertSiteProcesses(t *testing.T, running bool, slugs ...string) {
	t.Helper()
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range slugs {
		entry, exists := st.Get(slug)
		if running {
			if !exists || !state.EntryAlive(entry) {
				t.Errorf("%s has no live process", slug)
			}
		} else if exists {
			t.Errorf("%s still has runtime state after stopping: %+v", slug, entry)
		}
	}
}

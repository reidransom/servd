package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/reidransom/servd/internal/config"
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
	if err := os.WriteFile(filepath.Join(proj, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	ti := textinput.New()
	ti.SetValue(proj)
	m := &model{
		settings: config.Settings{PortRangeStart: 42201, BindHost: "127.0.0.1", Hostnames: config.HostnameSettings{HTTPPort: 42200}},
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
	if m.status != "removed widget" {
		t.Errorf("status = %q, want %q", m.status, "removed widget")
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

	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Fatal("expected a start command")
	} else {
		m.Update(cmd())
	}
	if got := m.status; got != "started widget" {
		t.Errorf("status = %q, want %q", got, "started widget")
	}

	m.Update(refreshCmd(m.settings)())
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")}); cmd == nil {
		t.Fatal("expected a stop command")
	} else {
		m.Update(cmd())
	}
	if got := m.status; got != "stopped widget" {
		t.Errorf("status = %q, want %q", got, "stopped widget")
	}
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
	if got := m.status; got != "started 2 site(s)" {
		t.Errorf("status = %q, want %q", got, "started 2 site(s)")
	}

	m.Update(refreshCmd(m.settings)())
	if _, cmd := m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("S")}); cmd == nil {
		t.Fatal("expected a stop-all command")
	} else {
		m.Update(cmd())
	}
	if got := m.status; got != "stopped 2 site(s)" {
		t.Errorf("status = %q, want %q", got, "stopped 2 site(s)")
	}
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
		settings: config.Settings{PortRangeStart: 42201, BindHost: "127.0.0.1", Hostnames: config.HostnameSettings{HTTPPort: 42200}},
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
	if m.status != "renamed widget to gadget" {
		t.Fatalf("rename status = %q", m.status)
	}

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
	if m.status != "restarted gadget" {
		t.Fatalf("restart status = %q", m.status)
	}
	runtime, err = state.Load()
	if err != nil {
		t.Fatal(err)
	}
	after, ok := runtime.Get("gadget")
	if !ok || !state.EntryAlive(after) || !after.StartedAt.After(before.StartedAt) {
		t.Fatalf("restart did not replace runtime entry: before=%+v after=%+v", before, after)
	}
}

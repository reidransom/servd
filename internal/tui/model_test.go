package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/reidransom/servd/internal/config"
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

	if !strings.Contains(m.View(), "s start") {
		t.Fatal("help bar missing from initial view")
	}
	shown := m.table.Height()

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if strings.Contains(m.View(), "s start") {
		t.Error("help bar still visible after h")
	}
	if got := m.table.Height(); got != shown+1 {
		t.Errorf("table height = %d after hiding help, want %d", got, shown+1)
	}

	m.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h")})
	if !strings.Contains(m.View(), "s start") {
		t.Error("help bar not restored by second h")
	}
	if got := m.table.Height(); got != shown {
		t.Errorf("table height = %d after restoring help, want %d", got, shown)
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
	if got, m := completePath(filepath.Join(root, "be")); got != filepath.Join(root, "beta")+"/" || len(m) != 1 {
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
	if _, m := completePath(root + "/"); len(m) != 3 {
		t.Errorf("list: got %d matches %v, want 3 (dotfile hidden)", len(m), m)
	}
	// A leading dot in the prefix reveals dotfiles.
	if got, m := completePath(filepath.Join(root, ".h")); len(m) != 1 || got != filepath.Join(root, ".hidden")+"/" {
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
	if got, want := m.addInput.Value(), filepath.Join(root, "widget")+"/"; got != want {
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

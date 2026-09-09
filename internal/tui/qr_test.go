package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/reidransom/servd/internal/config"
)

func TestQRModalCapturesInputAndKeepsURLSnapshot(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	site := config.Site{Slug: "alpha", Path: t.TempDir(), Port: 4011, Cmd: "sleep 30"}
	if err := (&config.Registry{Sites: []config.Site{site}}).Save(); err != nil {
		t.Fatal(err)
	}
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	selectDashboardSite(t, m, site.Slug)
	m.settings.Hostnames.HTTPPort = 8088
	m.viewport.SetContent("retained log line")
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")})
	snapshot := m.View()
	if !strings.Contains(ansi.Strip(snapshot), "http://alpha.localhost:8088/") {
		t.Fatalf("QR modal does not show the selected site's exact URL:\n%s", snapshot)
	}
	settings := m.settings
	settings.Hostnames.HTTPPort = 9090
	m.Update(buildStatuses(settings, m.reg, m.st))
	for _, msg := range []tea.Msg{
		tea.KeyMsg{Type: tea.KeyUp},
		tea.KeyMsg{Type: tea.KeyTab},
		tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")},
		tea.MouseMsg{X: 2, Y: firstRowY, Button: tea.MouseButtonLeft, Action: tea.MouseActionPress},
		tea.MouseMsg{X: 10, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionMotion},
		tea.MouseMsg{X: 10, Y: firstRowY + 1, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease},
		tea.MouseMsg{X: 2, Y: firstRowY, Button: tea.MouseButtonWheelUp},
	} {
		if _, cmd := m.Update(msg); cmd != nil {
			t.Fatalf("modal input %v triggered an action", msg)
		}
	}
	if got := m.View(); got != snapshot {
		t.Fatalf("input or refresh changed the QR snapshot:\n%s", got)
	}
	if _, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}); cmd != nil {
		t.Fatal("q quit instead of dismissing the QR modal")
	}
	for _, want := range []string{"http://alpha.localhost:9090/", "retained log line"} {
		if got := ansi.Strip(m.View()); !strings.Contains(got, want) {
			t.Fatalf("dismissal lost the selection or logs; missing %q:\n%s", want, got)
		}
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	if cmd == nil {
		t.Fatal("normal q no longer quits")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("normal q did not request quit")
	}
}

func TestQRProxyModalHidesMatrixUntilItFits(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m.settings.Hostnames.HTTPPort = 8088
	dashboard := m.View()
	m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")})
	full := m.View()
	if !strings.Contains(ansi.Strip(full), "http://127.0.0.1:8088/") || !strings.ContainsAny(full, "▀▄█") {
		t.Fatalf("proxy-only dashboard did not show a QR code:\n%s", full)
	}
	for _, size := range []tea.WindowSizeMsg{{Width: 20, Height: 50}, {Width: 100, Height: 8}} {
		m.Update(size)
		small := m.View()
		if strings.ContainsAny(small, "▀▄█") || !strings.Contains(small, "Need ") {
			t.Fatalf("small terminal displayed a partial matrix instead of dimensions:\n%s", small)
		}
		if lipgloss.Width(small) > size.Width || lipgloss.Height(small) > size.Height {
			t.Fatalf("resize instructions exceed terminal size: %dx%d", lipgloss.Width(small), lipgloss.Height(small))
		}
	}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if got := m.View(); got != full {
		t.Fatal("growing terminal did not restore the complete QR modal")
	}
	for _, dismiss := range []tea.KeyMsg{{Type: tea.KeyEsc}, {Type: tea.KeyRunes, Runes: []rune("Q")}} {
		m.Update(dismiss)
		if got := m.View(); got != dashboard {
			t.Fatal("dismissal did not restore the dashboard")
		}
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("Q")})
	}
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("Ctrl-C did not quit from the QR modal")
	}
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("Ctrl-C did not request quit from the QR modal")
	}
}

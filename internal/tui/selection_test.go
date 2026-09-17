package tui

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
)

func selectionModel(t *testing.T) (*model, *os.File) {
	t.Helper()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	m, err := newModel()
	if err != nil {
		t.Fatal(err)
	}
	out, err := os.CreateTemp(t.TempDir(), "terminal")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := out.Close(); err != nil {
			t.Error(err)
		}
	})
	m.output = &terminalOutput{File: out}
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 24})
	return m, out
}

func selectionMouse(m *model, action tea.MouseAction, x, y int) tea.Cmd {
	_, cmd := m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonLeft, Action: action})
	return cmd
}

func copiedText(t *testing.T, out *os.File) string {
	t.Helper()
	data, err := os.ReadFile(out.Name())
	if err != nil {
		t.Fatal(err)
	}
	sequence := string(data)
	if !strings.HasPrefix(sequence, "\x1b]52;c;") || !strings.HasSuffix(sequence, "\x07") {
		t.Fatalf("not a system clipboard write: %q", sequence)
	}
	text, err := base64.StdEncoding.DecodeString(strings.TrimSuffix(strings.TrimPrefix(sequence, "\x1b]52;c;"), "\x07"))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}

func TestDragSelectionHighlightsAndCopies(t *testing.T) {
	m, out := selectionModel(t)
	before := m.View()
	selectionMouse(m, tea.MouseActionPress, 0, 0)
	selectionMouse(m, tea.MouseActionMotion, 4, 0)
	if m.View() == before || ansi.Strip(m.View()) != ansi.Strip(before) {
		t.Fatal("drag must highlight text without changing displayed content")
	}
	cmd := selectionMouse(m, tea.MouseActionRelease, 4, 0)
	if cmd == nil {
		t.Fatal("releasing a text selection did not copy it")
	}
	m.Update(cmd())
	if got := copiedText(t, out); got != "servd" {
		t.Fatalf("clipboard = %q, want servd", got)
	}
	if cmd := selectionMouse(m, tea.MouseActionRelease, 4, 0); cmd != nil {
		t.Fatal("duplicate release copied again")
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if m.View() != before {
		t.Fatal("Escape did not clear the retained highlight")
	}
}

func TestLogSelectionCopiesSnapshotWithinPane(t *testing.T) {
	m, out := selectionModel(t)
	m.syncLogSelection()
	m.viewport.SetContent("\x1b[31malpha\x1b[0m\nbravo\ncharlie")
	x, y := m.sidebarWidth()+3, firstRowY+1
	selectionMouse(m, tea.MouseActionPress, x+2, y)
	selectionMouse(m, tea.MouseActionMotion, x+3, y+1)
	selected := m.View()
	// Refreshes must not move the visible selection or change the copied bytes.
	m.Update(tickMsg{})
	m.viewport.SetContent("new log output")
	if m.View() != selected {
		t.Fatal("live refresh changed the selected frame")
	}
	cmd := selectionMouse(m, tea.MouseActionRelease, x+3, y+1)
	if cmd == nil {
		t.Fatal("missing clipboard write")
	}
	m.Update(cmd())
	if got := copiedText(t, out); got != "pha\nbrav" {
		t.Fatalf("clipboard = %q, want only selected log cells", got)
	}
	m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !strings.Contains(ansi.Strip(m.View()), "new log output") {
		t.Fatal("clearing selection did not reveal refreshed logs")
	}
}

func TestReverseSelectionIncludesWholeGraphemes(t *testing.T) {
	m, out := selectionModel(t)
	m.viewport.SetContent("a界e\u0301👩‍💻z")
	x, y := m.sidebarWidth()+3, firstRowY+1
	// Both endpoints fall on the second cell of a wide grapheme.
	selectionMouse(m, tea.MouseActionPress, x+5, y)
	selectionMouse(m, tea.MouseActionMotion, x+2, y)
	_, cmd := m.Update(tea.MouseMsg{X: x + 2, Y: y, Button: tea.MouseButtonNone, Action: tea.MouseActionRelease})
	if cmd == nil {
		t.Fatal("legacy release did not finalize selection")
	}
	m.Update(cmd())
	if got := copiedText(t, out); got != "界e\u0301👩‍💻" {
		t.Fatalf("clipboard split a grapheme: %q", got)
	}
}

func TestSelectionClampsReleaseOutsidePane(t *testing.T) {
	m, out := selectionModel(t)
	m.viewport.SetContent("alpha\nbravo")
	x, y := m.sidebarWidth()+3, firstRowY+1
	selectionMouse(m, tea.MouseActionPress, x+2, y)
	// A final position is honored even if no intermediate motion arrived.
	cmd := selectionMouse(m, tea.MouseActionRelease, m.width+20, y+1)
	if cmd == nil {
		t.Fatal("release outside pane lost selection")
	}
	m.Update(cmd())
	if got := copiedText(t, out); got != "pha\nbravo" {
		t.Fatalf("clipboard includes border or padding: %q", got)
	}
}

func TestClickAndCancelledSelectionLeaveClipboardAlone(t *testing.T) {
	m, out := selectionModel(t)
	selectionMouse(m, tea.MouseActionPress, 0, 0)
	if cmd := selectionMouse(m, tea.MouseActionRelease, 0, 0); cmd != nil {
		t.Fatal("plain click copied text")
	}
	selectionMouse(m, tea.MouseActionPress, 0, 0)
	selectionMouse(m, tea.MouseActionMotion, 4, 0)
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	if cmd := selectionMouse(m, tea.MouseActionRelease, 4, 0); cmd != nil {
		t.Fatal("resize did not cancel selection")
	}
	x, y := m.sidebarWidth()+3, firstRowY+1
	m.viewport.SetContent(strings.Repeat("log line\n", 40))
	m.viewport.GotoBottom()
	selectionMouse(m, tea.MouseActionPress, x, y)
	selectionMouse(m, tea.MouseActionMotion, x+3, y)
	m.Update(tea.MouseMsg{X: x, Y: y, Button: tea.MouseButtonWheelUp})
	if m.viewport.AtBottom() {
		t.Fatal("wheel no longer scrolls the log pane")
	}
	if cmd := selectionMouse(m, tea.MouseActionRelease, x+3, y); cmd != nil {
		t.Fatal("scroll did not cancel stale selection")
	}
	info, err := out.Stat()
	if err != nil || info.Size() != 0 {
		t.Fatalf("click or cancellation wrote to clipboard: %v", err)
	}
}

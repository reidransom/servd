package tui

import (
	"image"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/reidransom/servd/internal/app"
)

// terminalOutput serializes clipboard escapes with Bubble Tea's renderer.
// Embedding File preserves the terminal descriptor used for resize detection.
type terminalOutput struct {
	*os.File
	mu sync.Mutex
}

func (o *terminalOutput) Write(p []byte) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.File.Write(p)
}

func (o *terminalOutput) WriteString(s string) (int, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.File.WriteString(s)
}

type clipboardDoneMsg struct{ err error }

func (o *terminalOutput) copy(text string) tea.Cmd {
	return func() tea.Msg {
		err := app.WriteClipboard(o, text)
		return clipboardDoneMsg{err}
	}
}

// A selection owns a snapshot of the displayed cells. Live log/status updates
// continue underneath, but cannot move the text being highlighted or copied.
type textSelection struct {
	lines             []string
	bounds            image.Rectangle
	anchor, cursor    image.Point
	dragged, released bool
}

func (s *textSelection) move(x, y int) {
	s.cursor = image.Pt(
		min(max(x, s.bounds.Min.X), s.bounds.Max.X-1),
		min(max(y, s.bounds.Min.Y), s.bounds.Max.Y-1),
	)
	s.dragged = s.dragged || s.cursor != s.anchor
}

func (s *textSelection) span(y int) (int, int) {
	start, end := s.anchor, s.cursor
	if start.Y > end.Y || (start.Y == end.Y && start.X > end.X) {
		start, end = end, start
	}
	if !s.dragged || y < start.Y || y > end.Y {
		return 0, 0
	}
	left, right := s.bounds.Min.X, s.bounds.Max.X
	if y == start.Y {
		left = start.X
	}
	if y == end.Y {
		right = end.X + 1
	}
	// Include complete graphemes when either endpoint hits a wide cell.
	plain := ansi.Strip(s.lines[y])
	for x := 0; len(plain) > 0; {
		cluster, width := ansi.FirstGraphemeCluster(plain, ansi.GraphemeWidth)
		if x < left && left < x+width {
			left = x
		}
		if x < right && right < x+width {
			right = x + width
		}
		x += width
		plain = plain[len(cluster):]
	}
	return left, right
}

func (s *textSelection) text() string {
	var lines []string
	for y, line := range s.lines {
		left, right := s.span(y)
		if left < right {
			lines = append(lines, strings.TrimRight(ansi.Strip(ansi.Cut(line, left, right)), " "))
		}
	}
	return strings.Join(lines, "\n")
}

func (s *textSelection) view() string {
	lines := make([]string, len(s.lines))
	for y, line := range s.lines {
		left, right := s.span(y)
		if left >= right {
			lines[y] = line
			continue
		}
		lines[y] = ansi.Cut(line, 0, left) + "\x1b[0;30;104m" +
			ansi.Strip(ansi.Cut(line, left, right)) + "\x1b[0m" +
			ansi.Cut(line, right, ansi.StringWidth(line))
	}
	return strings.Join(lines, "\n")
}

func (m *model) beginSelection(x, y int) {
	m.selection = nil
	lines := strings.Split(m.View(), "\n")
	bounds := image.Rect(0, 0, m.width, min(m.height, len(lines)))
	point := image.Pt(x, y)
	if !point.In(bounds) {
		return
	}
	if m.mode == modeNormal {
		sidebarWidth := m.sidebarWidth()
		panes := []image.Rectangle{
			image.Rect(1, firstRowY, sidebarWidth+1, firstRowY+m.table.Height()),
			image.Rect(sidebarWidth+3, firstRowY, sidebarWidth+3+m.viewport.Width, firstRowY+1+m.viewport.Height),
		}
		for _, pane := range panes {
			if point.In(pane) {
				bounds = bounds.Intersect(pane)
				break
			}
		}
	}
	m.selection = &textSelection{lines: lines, bounds: bounds, anchor: point, cursor: point}
}

func (m *model) handleSelectionMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		m.selection = nil
		return m.handleMouse(msg)
	}
	if msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress {
		m.beginSelection(msg.X, msg.Y)
		return m, nil
	}
	s := m.selection
	if s == nil || s.released {
		return m, nil
	}
	if msg.Action == tea.MouseActionMotion && msg.Button == tea.MouseButtonLeft {
		s.move(msg.X, msg.Y)
	}
	// Legacy mouse reporting does not identify which button was released.
	if msg.Action == tea.MouseActionRelease && (msg.Button == tea.MouseButtonLeft || msg.Button == tea.MouseButtonNone) {
		s.move(msg.X, msg.Y)
		s.released = true
		if s.dragged {
			if text := s.text(); strings.TrimSpace(text) != "" {
				return m, m.output.copy(text)
			}
			return m, nil
		}
		m.selection = nil
		msg.Action = tea.MouseActionPress
		msg.Button = tea.MouseButtonLeft
		return m.handleMouse(msg)
	}
	return m, nil
}

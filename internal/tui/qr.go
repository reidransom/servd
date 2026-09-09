package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
	qrcode "github.com/skip2/go-qrcode"

	"github.com/reidransom/servd/internal/proxy"
)

// qrModal owns the URL snapshot and matrix until dismissed. Refreshes may
// update the dashboard's URLs without changing what a device is scanning.
type qrModal struct {
	url    string
	matrix string
	width  int
	err    error
}

func (m *model) openQR() {
	url := ""
	if m.selectedSlug() == proxy.Slug {
		url = m.proxyURL()
	} else if site := m.selectedSite(); site != nil {
		url = m.settings.SiteURL(*site)
	}
	if url == "" {
		return
	}
	m.qr = qrModal{url: url}
	m.mode = modeQR
	code, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		m.qr.err = err
		return
	}
	// Bitmap includes the encoder's four-module quiet zone on every side.
	bitmap := code.Bitmap()
	m.qr.width = len(bitmap)
	m.qr.matrix = renderQRBitmap(bitmap)
}

// Each terminal cell represents two vertically adjacent modules. Explicit
// truecolor escapes bypass terminal-theme colors and automatic color detection.
func renderQRBitmap(bitmap [][]bool) string {
	var b strings.Builder
	b.Grow((len(bitmap) + 1) / 2 * (len(bitmap)*3 + 45))
	for y := 0; y < len(bitmap); y += 2 {
		if y > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("\x1b[0;38;2;0;0;0;48;2;255;255;255m")
		for x, upper := range bitmap[y] {
			lower := y+1 < len(bitmap) && bitmap[y+1][x]
			switch {
			case upper && lower:
				b.WriteRune('█')
			case upper:
				b.WriteRune('▀')
			case lower:
				b.WriteRune('▄')
			default:
				b.WriteByte(' ')
			}
		}
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

func (m *model) handleQRKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case "esc", "Q", "q":
		m.mode = modeNormal
		m.qr = qrModal{}
	}
	return m, nil
}

func (m *model) qrView() string {
	const notice = "Scanning device needs network/DNS access. Localhost/loopback is local to the scanning device."
	const hint = "Esc/Q/q close · Ctrl-C quit"
	width := max(m.qr.width, min(64, max(32, m.width-2)))
	body := titleStyle.Render("QR code") + "\n" + ansi.Hardwrap(m.qr.url, width, true) + "\n\n"
	if m.qr.err != nil {
		body += errStyle.Render(ansi.Hardwrap("Unable to generate QR code: "+m.qr.err.Error(), width, true))
	} else {
		body += m.qr.matrix
	}
	body += "\n\n" + dimStyle.Render(ansi.Hardwrap(notice, width, true)) + "\n" + helpStyle.Render(hint)
	modal := boxStyle.Width(width).Render(body)
	requiredWidth, requiredHeight := lipgloss.Width(modal), lipgloss.Height(modal)
	if m.width < requiredWidth || m.height < requiredHeight {
		// Never send a partially visible matrix to the terminal. Resizing simply
		// lays out the cached matrix again; it does not re-encode the URL.
		message := fmt.Sprintf("Need %dx%d; resize terminal.\n%s\nQR code\n%s", requiredWidth, requiredHeight, hint, m.qr.url)
		if m.qr.err != nil {
			message = "Unable to generate QR code: " + m.qr.err.Error() + "\n" + message
		}
		if m.width <= 0 || m.height <= 0 {
			return ""
		}
		lines := strings.Split(ansi.Hardwrap(message, m.width, true), "\n")
		return strings.Join(lines[:min(m.height, len(lines))], "\n")
	}
	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
}

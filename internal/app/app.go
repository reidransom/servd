// Package app holds small helpers shared by the CLI commands and the TUI
// (which can't import each other).
package app

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

// Load reads settings, registry and reconciled runtime state.
func Load() (config.Settings, *config.Registry, *state.State, error) {
	settings, err := config.LoadSettings()
	if err != nil {
		return settings, nil, nil, fmt.Errorf("loading settings: %w", err)
	}
	reg, err := config.LoadRegistry()
	if err != nil {
		return settings, nil, nil, fmt.Errorf("loading registry: %w", err)
	}
	st, err := state.Load()
	if err != nil {
		return settings, reg, nil, fmt.Errorf("loading state: %w", err)
	}
	return settings, reg, st, nil
}

// FmtDuration renders an uptime compactly: 45s, 1m30s, 3h5m.
func FmtDuration(d time.Duration) string {
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

// Dash substitutes "-" for an empty table cell.
func Dash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// OpenBrowser opens a URL in the default browser (best effort).
func OpenBrowser(url string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin, args = "open", []string{url}
	case "windows":
		bin, args = "cmd", []string{"/c", "start", url}
	default:
		bin, args = "xdg-open", []string{url}
	}
	return exec.Command(bin, args...).Start()
}

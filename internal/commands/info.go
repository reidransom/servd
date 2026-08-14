package commands

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
)

// siteInfo is the machine-readable record for one site, emitted by the
// --json flags so agents and scripts can discover running servers without
// parsing tables or reading state.json (whose format is not a public API).
type siteInfo struct {
	Slug          string     `json:"slug"`
	Path          string     `json:"path"`
	Port          int        `json:"port"`
	URL           string     `json:"url"` // via the reverse proxy (needs proxy.accepting)
	FallbackURL   string     `json:"fallback_url,omitempty"`
	DirectURL     string     `json:"direct_url"` // straight to the dev server's port
	Launcher      string     `json:"launcher,omitempty"`
	Status        string     `json:"status"` // stopped | starting | running
	PID           int        `json:"pid,omitempty"`
	Cmd           string     `json:"cmd,omitempty"` // command the live process was started with
	Log           string     `json:"log"`
	StartedAt     *time.Time `json:"started_at,omitempty"`
	UptimeSeconds int        `json:"uptime_seconds,omitempty"`
}

// proxyInfo describes the background reverse proxy in --json output. running
// is "pid alive"; accepting is the stronger "port answers connections".
type proxyInfo struct {
	Running            bool     `json:"running"`
	Accepting          bool     `json:"accepting"`
	PID                int      `json:"pid,omitempty"`
	Port               int      `json:"port"`
	PrimaryURLPattern  string   `json:"primary_url_pattern"`
	FallbackURLPattern string   `json:"fallback_url_pattern,omitempty"`
	TLDs               []string `json:"tlds"`
	NipIO              bool     `json:"nip_io"`
}

func newSiteInfo(settings config.Settings, s config.Site, st *state.State) siteInfo {
	info := siteInfo{
		Slug:      s.Slug,
		Path:      s.Path,
		Port:      s.Port,
		URL:       settings.SiteURL(s),
		DirectURL: fmt.Sprintf("http://127.0.0.1:%d/", s.Port),
		Launcher:  s.Launcher,
		Status:    supervisor.StatusOf(s, st).String(),
		Log:       supervisor.LogPath(s.Slug),
	}
	if fallback, ok := settings.FallbackURL(s); ok {
		info.FallbackURL = fallback
	}
	if e, ok := st.Get(s.Slug); ok && state.EntryAlive(e) {
		info.PID = e.PID
		info.Cmd = e.Cmd
		started := e.StartedAt
		info.StartedAt = &started
		info.UptimeSeconds = int(time.Since(e.StartedAt).Seconds())
	}
	return info
}

func newProxyInfo(settings config.Settings, st *state.State) proxyInfo {
	running, pid := proxy.Running(st)
	info := proxyInfo{
		Running:           running,
		Accepting:         proxy.Accepting(settings),
		PID:               pid,
		Port:              settings.Hostnames.HTTPPort,
		PrimaryURLPattern: settings.PrimaryURLPattern(),
		TLDs:              append([]string(nil), settings.Hostnames.TLDs...),
		NipIO:             settings.Hostnames.NipIO,
	}
	if fallback, ok := settings.FallbackURLPattern(); ok {
		info.FallbackURLPattern = fallback
	}
	return info
}

// printJSON writes v to stdout, indented, with a trailing newline.
func printJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

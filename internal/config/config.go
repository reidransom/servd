// Package config loads and persists servd's settings and the site registry.
//
// Two files live under the XDG config dir (~/.config/servd):
//   - config.toml : global Settings (projects dir, port range, proxy port, ...)
//   - sites.toml  : the registry of known sites ([[site]] entries)
//
// Runtime state (pids, logs) is handled separately in the state package.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/reidransom/servd/internal/flock"
	"github.com/reidransom/servd/internal/hostnames"
)

// Settings holds global servd configuration.
type Settings struct {
	PortRangeStart int              `toml:"port_range_start"`
	BindHost       string           `toml:"bind_host"`
	Hostnames      HostnameSettings `toml:"hostnames"`
	// DefaultEnabled controls whether newly added sites start out enabled.
	// Defaults to false: registration leaves sites disabled until you `servd
	// enable` the ones you want `up --all` to run.
	DefaultEnabled bool `toml:"default_enabled"`
}

// HostsMode controls future hosts-file synchronization behavior. It is
// configured now so legacy sync_hosts migrations retain their intent.
type HostsMode string

const (
	HostsAuto   HostsMode = "auto"
	HostsAlways HostsMode = "always"
	HostsNever  HostsMode = "never"
)

// HostnameSettings holds public hostname and listener settings separately from
// backend process binding and port allocation.
type HostnameSettings struct {
	TLDs        []string  `toml:"tlds"`
	HTTPS       bool      `toml:"https"`
	HTTPPort    int       `toml:"http_port"`
	HostsMode   HostsMode `toml:"hosts_mode"`
	LAN         bool      `toml:"lan"`
	LANIP       string    `toml:"lan_ip"`
	NipIO       bool      `toml:"nip_io"`
	NipIOSuffix string    `toml:"nip_io_suffix"`
}

// Site is one registered project in the registry.
type Site struct {
	Slug       string `toml:"slug"`
	HostPrefix string `toml:"host_prefix,omitempty"`
	Path       string `toml:"path"`
	Port       int    `toml:"port"`
	Enabled    bool   `toml:"enabled"`
	Cmd        string `toml:"cmd,omitempty"`      // manual launch override (highest precedence)
	Launcher   string `toml:"launcher,omitempty"` // recorded resolver kind, e.g. "jigyll", "procfile"
	// PreserveHost forwards the original routed Host header to the backend
	// instead of rewriting it to the backend's own address. Only needed by
	// servers that build absolute URLs from Host; such servers must then
	// allowlist their servd hostname themselves.
	PreserveHost bool `toml:"preserve_host,omitempty"`
}

// Registry is the on-disk list of sites (sites.toml).
type Registry struct {
	Sites []Site `toml:"site"`
}

// DefaultSettings returns settings with sane defaults filled in.
func DefaultSettings() Settings {
	return Settings{
		PortRangeStart: 4001,
		BindHost:       "127.0.0.1",
		Hostnames: HostnameSettings{
			TLDs:        []string{"localhost"},
			HTTPS:       false,
			HTTPPort:    8080,
			HostsMode:   HostsAuto,
			NipIO:       true,
			NipIOSuffix: "127.0.0.1.nip.io",
		},
	}
}

// ConfigDir is ~/.config/servd (honoring XDG_CONFIG_HOME).
func ConfigDir() string {
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "servd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "servd")
}

// StateDir is ~/.local/state/servd (honoring XDG_STATE_HOME).
func StateDir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "servd")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "servd")
}

// LogDir is where per-site logfiles are written.
func LogDir() string { return filepath.Join(StateDir(), "logs") }

// HostnameBase returns the route base for site, including its stored worktree
// prefix when present.
func (s Settings) HostnameBase(site Site) (string, error) {
	if err := hostnames.ValidateLabel(site.Slug); err != nil {
		return "", fmt.Errorf("site slug: %w", err)
	}
	if site.HostPrefix != "" {
		if err := hostnames.ValidateLabel(site.HostPrefix); err != nil {
			return "", fmt.Errorf("site host prefix: %w", err)
		}
	}
	return hostnames.ApplyWorktreePrefix(site.Slug, site.HostPrefix), nil
}

// PrimaryHostnames returns the configured hostname for every primary TLD in
// declaration order.
func (s Settings) PrimaryHostnames(site Site) ([]string, error) {
	base, err := s.HostnameBase(site)
	if err != nil {
		return nil, err
	}
	return hostnames.ParseHostnames(base, s.Hostnames.TLDs)
}

// PrimaryHostname returns the preferred, first configured hostname.
func (s Settings) PrimaryHostname(site Site) (string, error) {
	hosts, err := s.PrimaryHostnames(site)
	if err != nil {
		return "", err
	}
	if len(hosts) == 0 {
		return "", errors.New("at least one primary hostname is required")
	}
	return hosts[0], nil
}

// NipIOHostname returns the compatibility hostname when fallback routing is
// enabled.
func (s Settings) NipIOHostname(site Site) (string, error) {
	if !s.Hostnames.NipIO {
		return "", errors.New("nip.io fallback routing is disabled")
	}
	base, err := s.HostnameBase(site)
	if err != nil {
		return "", err
	}
	return hostnames.ParseHostname(base, s.Hostnames.NipIOSuffix)
}

// RouteHostnames returns every exact hostname served for site.
func (s Settings) RouteHostnames(site Site) ([]string, error) {
	hosts, err := s.PrimaryHostnames(site)
	if err != nil {
		return nil, err
	}
	if s.Hostnames.NipIO {
		nip, err := s.NipIOHostname(site)
		if err != nil {
			return nil, err
		}
		hosts = append(hosts, nip)
	}
	return hosts, nil
}

// FallbackURL returns the nip.io compatibility URL, if configured.
func (s Settings) FallbackURL(site Site) (string, bool) {
	host, err := s.NipIOHostname(site)
	if err != nil {
		return "", false
	}
	return hostnames.FormatURL(host, s.Hostnames.HTTPPort, false) + "/", true
}

// PrimaryURLPattern returns the preferred route shape for command output.
func (s Settings) PrimaryURLPattern() string {
	tld := "<invalid-tld>"
	if len(s.Hostnames.TLDs) > 0 {
		tld = s.Hostnames.TLDs[0]
	}
	return hostnames.FormatURL("<slug>."+tld, s.Hostnames.HTTPPort, false) + "/"
}

// FallbackURLPattern returns the optional nip.io route shape.
func (s Settings) FallbackURLPattern() (string, bool) {
	if !s.Hostnames.NipIO || s.Hostnames.NipIOSuffix == "" {
		return "", false
	}
	return hostnames.FormatURL("<slug>."+s.Hostnames.NipIOSuffix, s.Hostnames.HTTPPort, false) + "/", true
}

// SiteURL is the preferred HTTP URL a site is reachable at through the proxy.
// app.Load validates Settings before callers can use this helper.
func (s Settings) SiteURL(site Site) string {
	host, err := s.PrimaryHostname(site)
	if err != nil {
		return ""
	}
	return hostnames.FormatURL(host, s.Hostnames.HTTPPort, false) + "/"
}

func settingsPath() string { return filepath.Join(ConfigDir(), "config.toml") }
func registryPath() string { return filepath.Join(ConfigDir(), "sites.toml") }

// RegistryPath exposes the sites.toml path (used by the proxy to watch mtime).
func RegistryPath() string { return registryPath() }

// rawSettings keeps migration-only fields presence-aware so explicitly supplied
// hostname settings always win over their legacy counterparts.
type rawSettings struct {
	PortRangeStart *int                 `toml:"port_range_start"`
	BindHost       *string              `toml:"bind_host"`
	DefaultEnabled *bool                `toml:"default_enabled"`
	ProxyPort      *int                 `toml:"proxy_port"`
	DomainSuffix   *string              `toml:"domain_suffix"`
	Hostnames      *rawHostnameSettings `toml:"hostnames"`
}

type rawHostnameSettings struct {
	TLDs        *[]string  `toml:"tlds"`
	HTTPS       *bool      `toml:"https"`
	HTTPPort    *int       `toml:"http_port"`
	HostsMode   *HostsMode `toml:"hosts_mode"`
	SyncHosts   *bool      `toml:"sync_hosts"`
	LAN         *bool      `toml:"lan"`
	LANIP       *string    `toml:"lan_ip"`
	NipIO       *bool      `toml:"nip_io"`
	NipIOSuffix *string    `toml:"nip_io_suffix"`
}

// LoadSettings reads config.toml and migrates legacy keys into the hostname
// model without retaining them as runtime settings.
func LoadSettings() (Settings, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return s, s.Validate()
	}
	if err != nil {
		return s, err
	}

	var raw rawSettings
	if err := toml.Unmarshal(data, &raw); err != nil {
		return s, err
	}
	if raw.PortRangeStart != nil {
		s.PortRangeStart = *raw.PortRangeStart
	}
	if raw.BindHost != nil {
		s.BindHost = *raw.BindHost
	}
	if raw.DefaultEnabled != nil {
		s.DefaultEnabled = *raw.DefaultEnabled
	}
	if h := raw.Hostnames; h != nil {
		if h.TLDs != nil {
			s.Hostnames.TLDs = *h.TLDs
		}
		if h.HTTPS != nil {
			s.Hostnames.HTTPS = *h.HTTPS
		}
		if h.HTTPPort != nil {
			s.Hostnames.HTTPPort = *h.HTTPPort
		}
		if h.HostsMode != nil {
			s.Hostnames.HostsMode = *h.HostsMode
		} else if h.SyncHosts != nil {
			if *h.SyncHosts {
				s.Hostnames.HostsMode = HostsAlways
			} else {
				s.Hostnames.HostsMode = HostsNever
			}
		}
		if h.LAN != nil {
			s.Hostnames.LAN = *h.LAN
		}
		if h.LANIP != nil {
			s.Hostnames.LANIP = *h.LANIP
		}
		if h.NipIO != nil {
			s.Hostnames.NipIO = *h.NipIO
		}
		if h.NipIOSuffix != nil {
			s.Hostnames.NipIOSuffix = *h.NipIOSuffix
		}
	}
	if raw.ProxyPort != nil && (raw.Hostnames == nil || raw.Hostnames.HTTPPort == nil) {
		s.Hostnames.HTTPPort = *raw.ProxyPort
	}
	if raw.DomainSuffix != nil &&
		(raw.Hostnames == nil || raw.Hostnames.TLDs == nil) &&
		*raw.DomainSuffix != "127.0.0.1.nip.io" {
		s.Hostnames.TLDs = []string{*raw.DomainSuffix}
	}
	return s, s.Validate()
}

// Validate verifies the active HTTP hostname contract.
func (s Settings) Validate() error {
	if s.Hostnames.HTTPPort < 1 || s.Hostnames.HTTPPort > 65535 {
		return fmt.Errorf("hostnames.http_port must be between 1 and 65535")
	}
	if len(s.Hostnames.TLDs) == 0 {
		return errors.New("hostnames.tlds must contain at least one TLD")
	}
	for _, tld := range s.Hostnames.TLDs {
		if err := hostnames.ValidateTLD(tld); err != nil {
			return fmt.Errorf("hostnames.tlds: %w", err)
		}
	}
	if s.Hostnames.NipIO {
		if err := hostnames.ValidateTLD(s.Hostnames.NipIOSuffix); err != nil {
			return fmt.Errorf("hostnames.nip_io_suffix: %w", err)
		}
	}
	if s.Hostnames.HTTPS {
		return errors.New("hostnames.https is unsupported: HTTPS listener support has not landed")
	}
	switch s.Hostnames.HostsMode {
	case HostsAuto, HostsAlways, HostsNever:
	default:
		return fmt.Errorf("hostnames.hosts_mode must be auto, always, or never")
	}
	return nil
}

// SaveSettings writes config.toml atomically using only the active hostname
// model; legacy migration keys are intentionally omitted.
func SaveSettings(s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(s)
	if err != nil {
		return err
	}
	return WriteAtomic(settingsPath(), data)
}

// LoadRegistry reads sites.toml; a missing file yields an empty registry.
func LoadRegistry() (*Registry, error) {
	r := &Registry{}
	data, err := os.ReadFile(registryPath())
	if errors.Is(err, os.ErrNotExist) {
		return r, nil
	}
	if err != nil {
		return r, err
	}
	if err := toml.Unmarshal(data, r); err != nil {
		return r, err
	}
	return r, nil
}

// MutateRegistry loads sites.toml, applies fn and saves the result, all
// under an exclusive file lock. All registry writers must go through here so
// concurrent servd processes (CLI, TUI) can't lose each other's updates.
// Keep fn fast — don't stop/start servers while holding the lock.
func MutateRegistry(fn func(*Registry) error) error {
	return flock.WithLock(registryPath(), func() error {
		r, err := LoadRegistry()
		if err != nil {
			return err
		}
		if err := fn(r); err != nil {
			return err
		}
		return r.Save()
	})
}

// Save writes the registry to sites.toml atomically (sorted by slug).
func (r *Registry) Save() error {
	sort.Slice(r.Sites, func(i, j int) bool { return r.Sites[i].Slug < r.Sites[j].Slug })
	data, err := toml.Marshal(r)
	if err != nil {
		return err
	}
	return WriteAtomic(registryPath(), data)
}

// Find returns a pointer to the site with the given slug, or nil.
func (r *Registry) Find(slug string) *Site {
	for i := range r.Sites {
		if r.Sites[i].Slug == slug {
			return &r.Sites[i]
		}
	}
	return nil
}

// FindByPath returns the site registered at the given absolute path, or nil.
func (r *Registry) FindByPath(path string) *Site {
	for i := range r.Sites {
		if r.Sites[i].Path == path {
			return &r.Sites[i]
		}
	}
	return nil
}

// HasPort reports whether any site already uses the given port.
func (r *Registry) HasPort(port int) bool {
	for i := range r.Sites {
		if r.Sites[i].Port == port {
			return true
		}
	}
	return false
}

// WriteAtomic writes data to path via a temp file + rename, creating parents.
// Atomic, not durable: there is no fsync before the rename.
func WriteAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

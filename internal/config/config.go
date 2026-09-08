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
}

// SettingsSource records whether settings came from the user's config file.
// Defaults are still valid settings, but callers that choose a listener need
// to distinguish them from an explicit configuration choice.
type SettingsSource struct {
	ConfigPresent bool
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
	TLD          string    `toml:"tlds"`
	TLDsFallback []string  `toml:"tlds_fallback"`
	HTTPS        bool      `toml:"https"`
	HTTPPort     int       `toml:"http_port"`
	HostsMode    HostsMode `toml:"hosts_mode"`
	LAN          bool      `toml:"lan"`
	LANIP        string    `toml:"lan_ip"`
}

// Site is one registered project in the registry.
type Site struct {
	Slug       string `toml:"slug"`
	HostPrefix string `toml:"host_prefix,omitempty"`
	Path       string `toml:"path"`
	Port       int    `toml:"port"`
	Cmd        string `toml:"cmd,omitempty"`
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
			TLD:          "localhost",
			TLDsFallback: []string{},
			HTTPS:        false,
			HTTPPort:     8080,
			HostsMode:    HostsAuto,
		},
	}
}

// EnableLAN selects the .local hostname family required for mDNS publishing.
func (s *Settings) EnableLAN() {
	s.Hostnames.LAN = true
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

func (s Settings) primaryTLD() string {
	if s.Hostnames.LAN {
		return "local"
	}
	return s.Hostnames.TLD
}

// fallbackTLDs preserves declaration order without repeating the primary or
// another fallback. It never modifies the declared settings.
func (s Settings) fallbackTLDs() []string {
	primary := s.primaryTLD()
	seen := make(map[string]struct{}, len(s.Hostnames.TLDsFallback))
	var fallbacks []string
	for _, tld := range s.Hostnames.TLDsFallback {
		if tld == primary {
			continue
		}
		if _, ok := seen[tld]; ok {
			continue
		}
		seen[tld] = struct{}{}
		fallbacks = append(fallbacks, tld)
	}
	return fallbacks
}

// PrimaryHostname returns the site's single primary hostname.
func (s Settings) PrimaryHostname(site Site) (string, error) {
	base, err := s.HostnameBase(site)
	if err != nil {
		return "", err
	}
	tld := s.primaryTLD()
	return hostnames.ParseHostname(base+"."+tld, tld)
}

// RouteHostnames returns the primary followed by every distinct explicit
// fallback. Any invalid hostname fails the entire route list.
func (s Settings) RouteHostnames(site Site) ([]string, error) {
	base, err := s.HostnameBase(site)
	if err != nil {
		return nil, err
	}
	tld := s.primaryTLD()
	primary, err := hostnames.ParseHostname(base+"."+tld, tld)
	if err != nil {
		return nil, fmt.Errorf("primary hostname: %w", err)
	}
	fallbacks := s.fallbackTLDs()
	hosts := make([]string, 1, 1+len(fallbacks))
	hosts[0] = primary
	for _, suffix := range fallbacks {
		host, err := hostnames.ParseHostname(base+"."+suffix, suffix)
		if err != nil {
			return nil, fmt.Errorf("fallback hostname for %q: %w", suffix, err)
		}
		hosts = append(hosts, host)
	}
	return hosts, nil
}

// FallbackURLs returns the distinct explicit alternative URLs.
// app.Load validates settings and registry routes before URL helpers are used.
func (s Settings) FallbackURLs(site Site) []string {
	hosts, err := s.RouteHostnames(site)
	if err != nil {
		return nil
	}
	var urls []string
	for _, host := range hosts[1:] {
		urls = append(urls, hostnames.FormatURL(host, s.Hostnames.HTTPPort, false)+"/")
	}
	return urls
}

// PrimaryURLPattern returns the primary route shape for command output.
func (s Settings) PrimaryURLPattern() string {
	return hostnames.FormatURL("<slug>."+s.primaryTLD(), s.Hostnames.HTTPPort, false) + "/"
}

// FallbackURLPatterns returns the distinct explicit alternative route shapes.
func (s Settings) FallbackURLPatterns() []string {
	var patterns []string
	for _, tld := range s.fallbackTLDs() {
		patterns = append(patterns, hostnames.FormatURL("<slug>."+tld, s.Hostnames.HTTPPort, false)+"/")
	}
	return patterns
}

// SiteURL is the preferred HTTP URL a site is reachable at through the proxy.
// app.Load validates settings and registry routes before URL helpers are used.
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
	ProxyPort      *int                 `toml:"proxy_port"`
	DomainSuffix   *string              `toml:"domain_suffix"`
	Hostnames      *rawHostnameSettings `toml:"hostnames"`
}

type rawHostnameSettings struct {
	TLD          *string    `toml:"tlds"`
	TLDsFallback *[]string  `toml:"tlds_fallback"`
	HTTPS        *bool      `toml:"https"`
	HTTPPort     *int       `toml:"http_port"`
	HostsMode    *HostsMode `toml:"hosts_mode"`
	SyncHosts    *bool      `toml:"sync_hosts"`
	LAN          *bool      `toml:"lan"`
	LANIP        *string    `toml:"lan_ip"`
}

// LoadSettings reads config.toml and migrates legacy keys into the hostname
// model without retaining them as runtime settings.
func LoadSettings() (Settings, error) {
	settings, _, err := LoadSettingsWithSource()
	return settings, err
}

// LoadSettingsWithSource also reports whether config.toml existed. A malformed
// config remains present so callers never treat it as permission to use
// first-run defaults.
func LoadSettingsWithSource() (Settings, SettingsSource, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return s, SettingsSource{}, s.Validate()
	}
	if err != nil {
		return s, SettingsSource{}, err
	}
	source := SettingsSource{ConfigPresent: true}

	// Check removed keys before typed decoding so even a mixed old/new schema
	// reports the required migration instead of silently losing a route.
	var keys struct {
		Hostnames map[string]any `toml:"hostnames"`
	}
	if err := toml.Unmarshal(data, &keys); err != nil {
		return s, source, err
	}
	for _, key := range []string{"nip_io", "nip_io_suffix"} {
		if _, present := keys.Hostnames[key]; present {
			return s, source, fmt.Errorf("hostnames.%s has been removed: delete the key and explicitly choose any wanted suffix with hostnames.tlds (a string) or hostnames.tlds_fallback (a list of strings)", key)
		}
	}
	if value, present := keys.Hostnames["tlds"]; present {
		if _, ok := value.(string); !ok {
			return s, source, errors.New("hostnames.tlds must be one nonempty string: replace the old array with its chosen primary suffix and move any wanted alternatives to hostnames.tlds_fallback")
		}
	}

	var raw rawSettings
	if err := toml.Unmarshal(data, &raw); err != nil {
		return s, source, err
	}
	if raw.PortRangeStart != nil {
		s.PortRangeStart = *raw.PortRangeStart
	}
	if raw.BindHost != nil {
		s.BindHost = *raw.BindHost
	}
	if h := raw.Hostnames; h != nil {
		if h.TLD != nil {
			s.Hostnames.TLD = *h.TLD
		}
		if h.TLDsFallback != nil {
			s.Hostnames.TLDsFallback = *h.TLDsFallback
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
	}
	if raw.ProxyPort != nil && (raw.Hostnames == nil || raw.Hostnames.HTTPPort == nil) {
		s.Hostnames.HTTPPort = *raw.ProxyPort
	}
	if raw.DomainSuffix != nil && (raw.Hostnames == nil || raw.Hostnames.TLD == nil) {
		s.Hostnames.TLD = *raw.DomainSuffix
	}
	return s, source, s.Validate()
}

// Validate verifies the active HTTP hostname contract.
func (s Settings) Validate() error {
	if s.Hostnames.HTTPPort < 1 || s.Hostnames.HTTPPort > 65535 {
		return fmt.Errorf("hostnames.http_port must be between 1 and 65535")
	}
	if err := hostnames.ValidateTLD(s.Hostnames.TLD); err != nil {
		return fmt.Errorf("hostnames.tlds: %w", err)
	}
	for i, tld := range s.Hostnames.TLDsFallback {
		if err := hostnames.ValidateTLD(tld); err != nil {
			return fmt.Errorf("hostnames.tlds_fallback[%d]: %w", i, err)
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

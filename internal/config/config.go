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
)

// Settings holds global servd configuration.
type Settings struct {
	ProjectsDir    string `toml:"projects_dir"`
	PortRangeStart int    `toml:"port_range_start"`
	ProxyPort      int    `toml:"proxy_port"`
	DomainSuffix   string `toml:"domain_suffix"`
	BindHost       string `toml:"bind_host"`
	// DefaultEnabled controls whether newly scanned/added sites start out
	// enabled. Defaults to false: discovery registers sites but leaves them
	// disabled until you `servd enable` the ones you want `up --all` to run.
	DefaultEnabled bool `toml:"default_enabled"`
}

// Site is one registered project in the registry.
type Site struct {
	Slug     string `toml:"slug"`
	Path     string `toml:"path"`
	Port     int    `toml:"port"`
	Enabled  bool   `toml:"enabled"`
	Cmd      string `toml:"cmd,omitempty"`      // manual launch override (highest precedence)
	Launcher string `toml:"launcher,omitempty"` // recorded resolver kind, e.g. "jigyll", "procfile"
	// PreserveHost forwards the original (nip.io) Host header to the backend
	// instead of rewriting it to the backend's own address. Only needed by
	// servers that build absolute URLs from Host; such servers must then
	// allowlist the nip.io hostname themselves.
	PreserveHost bool `toml:"preserve_host,omitempty"`
}

// Registry is the on-disk list of sites (sites.toml).
type Registry struct {
	Sites []Site `toml:"site"`
}

// DefaultSettings returns settings with sane defaults filled in.
func DefaultSettings() Settings {
	home, _ := os.UserHomeDir()
	return Settings{
		ProjectsDir:    filepath.Join(home, "clients"),
		PortRangeStart: 4001,
		ProxyPort:      8080,
		DomainSuffix:   "127.0.0.1.nip.io",
		BindHost:       "127.0.0.1",
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

// SiteURL is the nip.io URL a site is reachable at through the proxy.
func (s Settings) SiteURL(site Site) string {
	return fmt.Sprintf("http://%s.%s:%d/", site.Slug, s.DomainSuffix, s.ProxyPort)
}

func settingsPath() string { return filepath.Join(ConfigDir(), "config.toml") }
func registryPath() string { return filepath.Join(ConfigDir(), "sites.toml") }

// RegistryPath exposes the sites.toml path (used by the proxy to watch mtime).
func RegistryPath() string { return registryPath() }

// LoadSettings reads config.toml, falling back to defaults for missing fields.
func LoadSettings() (Settings, error) {
	s := DefaultSettings()
	data, err := os.ReadFile(settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := toml.Unmarshal(data, &s); err != nil {
		return s, err
	}
	// Re-fill any zero values from defaults.
	d := DefaultSettings()
	if s.ProjectsDir == "" {
		s.ProjectsDir = d.ProjectsDir
	}
	if s.PortRangeStart == 0 {
		s.PortRangeStart = d.PortRangeStart
	}
	if s.ProxyPort == 0 {
		s.ProxyPort = d.ProxyPort
	}
	if s.DomainSuffix == "" {
		s.DomainSuffix = d.DomainSuffix
	}
	if s.BindHost == "" {
		s.BindHost = d.BindHost
	}
	return s, nil
}

// SaveSettings writes config.toml atomically.
func SaveSettings(s Settings) error {
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
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

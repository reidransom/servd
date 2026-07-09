// Package scan discovers servable projects under a directory and merges them
// into the registry, assigning a stable slug and port to each new one.
package scan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
)

// maxDepth bounds how deep below the root we look for project dirs.
const maxDepth = 2

// Result describes one newly-added site from a scan.
type Result struct {
	Slug string
	Path string
	Port int
	Kind string // resolved launcher kind, e.g. "jigyll", "static"
}

// Scan walks root (depth-limited), finds directories that resolve to a launch
// command, and adds any not already in the registry. Returns the new sites.
// The caller is responsible for saving the registry.
func Scan(root string, reg *config.Registry, settings config.Settings) ([]Result, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var added []Result
	candidates, err := walk(root)
	if err != nil {
		return nil, err
	}
	for _, dir := range candidates {
		if reg.FindByPath(dir) != nil {
			continue // already registered
		}
		// walk only returns servable dirs; resolve again to record the kind.
		res, err := launcher.Resolve(config.Site{Path: dir}, settings)
		if err != nil {
			continue
		}
		slug := uniqueSlug(Slugify(filepath.Base(dir)), reg)
		port := NextFreePort(reg, settings)
		site := config.Site{Slug: slug, Path: dir, Port: port, Enabled: settings.DefaultEnabled, Launcher: res.Kind}
		reg.Sites = append(reg.Sites, site)
		added = append(added, Result{Slug: slug, Path: dir, Port: port, Kind: res.Kind})
	}
	return added, nil
}

// AddParams are the inputs to AddSite. A zero-value Slug, Port or Cmd means
// "derive it" (slug from the folder name, port from NextFreePort, command from
// launcher auto-detection).
type AddParams struct {
	Path   string
	Slug   string
	Cmd    string
	Port   int
	Enable bool
}

// AddSite registers one project in reg, defaulting the slug and port and
// resolving the launcher, applying the same dup/validation checks as the
// `servd add` CLI. The caller supplies the lock via config.MutateRegistry and
// is responsible for saving the registry. Returns the created site.
func AddSite(reg *config.Registry, settings config.Settings, in AddParams) (config.Site, error) {
	abs, err := filepath.Abs(in.Path)
	if err != nil {
		return config.Site{}, err
	}
	if reg.FindByPath(abs) != nil {
		return config.Site{}, fmt.Errorf("%s is already registered", abs)
	}
	slug := in.Slug
	if slug == "" {
		slug = Slugify(filepath.Base(abs))
	}
	if reg.Find(slug) != nil {
		return config.Site{}, fmt.Errorf("slug %q already in use", slug)
	}
	port := in.Port
	if port == 0 {
		port = NextFreePort(reg, settings)
	} else if reg.HasPort(port) {
		return config.Site{}, fmt.Errorf("port %d already assigned", port)
	}
	site := config.Site{Slug: slug, Path: abs, Port: port, Enabled: settings.DefaultEnabled || in.Enable, Cmd: in.Cmd}
	if res, err := launcher.Resolve(site, settings); err == nil {
		site.Launcher = res.Kind
	} else if in.Cmd == "" {
		return config.Site{}, fmt.Errorf("cannot determine how to serve %s; pass a command: %w", abs, err)
	}
	reg.Sites = append(reg.Sites, site)
	return site, nil
}

// walk returns directories at depth 0..maxDepth under root, where each level is
// considered a potential project root. A directory that is itself servable is
// not descended into (its children aren't separate projects).
func walk(root string) ([]string, error) {
	var dirs []string
	var rec func(dir string, depth int)
	rec = func(dir string, depth int) {
		if launcher.Servable(dir) {
			dirs = append(dirs, dir)
			return // don't descend into a project
		}
		if depth >= maxDepth {
			return
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return
		}
		for _, e := range entries {
			if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
				continue
			}
			if e.Name() == "node_modules" || e.Name() == "vendor" {
				continue
			}
			rec(filepath.Join(dir, e.Name()), depth+1)
		}
	}
	// The root itself may be a single project, or a parent of projects.
	rec(root, 0)
	return dirs, nil
}

// NextFreePort returns the lowest port at or above settings.PortRangeStart
// that is unused in the registry, isn't the proxy port, and isn't currently
// bound by some other process on the host.
func NextFreePort(reg *config.Registry, settings config.Settings) int {
	p := settings.PortRangeStart
	for {
		if !reg.HasPort(p) && p != settings.ProxyPort && netcheck.PortFree(settings.BindHost, p) {
			return p
		}
		p++
	}
}

var nonSlug = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify lowercases and replaces runs of non-alphanumerics with a hyphen.
func Slugify(name string) string {
	s := nonSlug.ReplaceAllString(strings.ToLower(name), "-")
	s = strings.Trim(s, "-")
	if s == "" {
		s = "site"
	}
	return s
}

// uniqueSlug appends -2, -3, ... until the slug is unused in the registry.
func uniqueSlug(base string, reg *config.Registry) string {
	if reg.Find(base) == nil {
		return base
	}
	for i := 2; ; i++ {
		cand := base + "-" + strconv.Itoa(i)
		if reg.Find(cand) == nil {
			return cand
		}
	}
}

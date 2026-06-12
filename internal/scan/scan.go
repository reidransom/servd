// Package scan discovers servable projects under a directory and merges them
// into the registry, assigning a stable slug and port to each new one.
package scan

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
)

// maxDepth bounds how deep below the root we look for project dirs.
const maxDepth = 2

// Result describes one newly-added site from a scan.
type Result struct {
	Slug string
	Path string
	Port int
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
		if !launcher.Servable(dir) {
			continue
		}
		slug := uniqueSlug(Slugify(filepath.Base(dir)), reg)
		port := reg.NextFreePort(settings.PortRangeStart)
		site := config.Site{Slug: slug, Path: dir, Port: port, Enabled: settings.DefaultEnabled}
		reg.Sites = append(reg.Sites, site)
		added = append(added, Result{Slug: slug, Path: dir, Port: port})
	}
	return added, nil
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
		cand := base + "-" + itoa(i)
		if reg.Find(cand) == nil {
			return cand
		}
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}

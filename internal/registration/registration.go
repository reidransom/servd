// Package registration registers individual projects with stable slugs and ports.
package registration

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
)

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

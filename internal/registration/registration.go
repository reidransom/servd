// Package registration registers individual projects with stable slugs and ports.
package registration

import (
	"fmt"
	"path/filepath"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostnames"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
)

// AddParams are the inputs to AddSite. Zero-value Slug, HostPrefix, Port or
// Cmd means "derive it". NoWorktreePrefix suppresses automatic prefix
// detection and cannot be combined with HostPrefix.
type AddParams struct {
	Path             string
	Slug             string
	HostPrefix       string
	NoWorktreePrefix bool
	Cmd              string
	Port             int
	Enable           bool
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
	if in.HostPrefix != "" && in.NoWorktreePrefix {
		return config.Site{}, fmt.Errorf("cannot use both host prefix and no-worktree-prefix")
	}
	slug := in.Slug
	if slug != "" {
		if err := hostnames.ValidateLabel(slug); err != nil {
			return config.Site{}, fmt.Errorf("invalid slug %q: %w (try %q)", slug, err, hostnames.SanitizeLabel(slug))
		}
	} else {
		if inferred, err := hostnames.InferProjectName(abs); err == nil {
			slug = hostnames.SanitizeLabel(inferred.Name)
		}
		if slug == "" {
			slug = hostnames.SanitizeLabel(filepath.Base(abs))
		}
		if err := hostnames.ValidateLabel(slug); err != nil {
			return config.Site{}, fmt.Errorf("cannot derive a valid slug for %s: %w", abs, err)
		}
	}
	if reg.Find(slug) != nil {
		return config.Site{}, fmt.Errorf("slug %q already in use", slug)
	}
	prefix := in.HostPrefix
	if prefix != "" {
		if err := hostnames.ValidateLabel(prefix); err != nil {
			return config.Site{}, fmt.Errorf("invalid host prefix %q: %w", prefix, err)
		}
	} else if !in.NoWorktreePrefix {
		if detected, err := hostnames.DetectWorktreePrefix(abs); err == nil {
			prefix = detected.Prefix
		}
	}
	if prefix != "" {
		if err := hostnames.ValidateLabel(prefix); err != nil {
			return config.Site{}, fmt.Errorf("invalid detected host prefix %q: %w", prefix, err)
		}
	}
	port := in.Port
	if port == 0 {
		port = NextFreePort(reg, settings)
	} else if reg.HasPort(port) {
		return config.Site{}, fmt.Errorf("port %d already assigned", port)
	}
	site := config.Site{Slug: slug, HostPrefix: prefix, Path: abs, Port: port, Enabled: settings.DefaultEnabled || in.Enable, Cmd: in.Cmd}
	if res, err := launcher.Resolve(site, settings); err == nil {
		site.Launcher = res.Kind
	} else if in.Cmd == "" {
		return config.Site{}, fmt.Errorf("cannot determine how to serve %s; pass a command: %w", abs, err)
	}
	reg.Sites = append(reg.Sites, site)
	return site, nil
}

// NextFreePort returns the lowest port at or above settings.PortRangeStart
// that is unused in the registry, isn't the proxy listener, and isn't
// currently bound by some other process on the host.
func NextFreePort(reg *config.Registry, settings config.Settings) int {
	p := settings.PortRangeStart
	for {
		if !reg.HasPort(p) && p != settings.Hostnames.HTTPPort && netcheck.PortFree(settings.BindHost, p) {
			return p
		}
		p++
	}
}

// Package launcher resolves a project directory into a concrete command for
// starting its dev server.
//
// Resolution order (first match wins):
//
//  1. Manual override — a non-empty Cmd on the site (set via `servd add --cmd`).
//  2. Procfile / Procfile.dev — the "web" process (or first), run via $PORT/$HOST.
//  3. Auto-detected convention — jigyll, hugo, node, just, make, static.
//
// Port/host injection is unified across all kinds: the supervisor always
// exports PORT and HOST into the child environment, and this package also
// substitutes {port}/{host} placeholders in template commands. Procfiles use
// $PORT; detectors use {port} flags; both work.
package launcher

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/reidransom/servd/internal/config"
)

// Resolved is a fully-resolved launch specification.
type Resolved struct {
	Cmd  string // shell command line with {port}/{host} substituted
	Kind string // launcher kind, e.g. "jigyll", "procfile", "manual"
	Dir  string // working directory (the project path)
}

// detectors are tried in order; the first that claims the dir wins.
var detectors = []func(dir string) (cmd, kind string, ok bool){
	detectJigyll,
	detectHugo,
	detectNode,
	detectJust,
	detectMake,
	detectStatic,
}

// resolveDir resolves a directory without a manual override: Procfile first,
// then the auto-detectors. The returned cmd still contains {port}/{host}
// placeholders.
func resolveDir(dir string) (cmd, kind string, ok bool) {
	if entries := readProcfile(dir); len(entries) > 0 {
		if e, found := webProcess(entries); found {
			return e.Cmd, "procfile", true
		}
	}
	for _, detect := range detectors {
		if cmd, kind, ok := detect(dir); ok {
			return cmd, kind, true
		}
	}
	return "", "", false
}

// Servable reports whether a directory can be resolved to a launch command
// without a manual override (used by scan to decide what to register).
func Servable(dir string) bool {
	_, _, ok := resolveDir(dir)
	return ok
}

// Resolve produces the launch spec for a site, applying the precedence order.
func Resolve(site config.Site, settings config.Settings) (Resolved, error) {
	dir := site.Path
	host := settings.BindHost
	port := site.Port

	// 1. Manual override.
	if strings.TrimSpace(site.Cmd) != "" {
		return Resolved{Cmd: subst(site.Cmd, host, port), Kind: "manual", Dir: dir}, nil
	}

	// 2. Procfile, then auto-detect.
	if cmd, kind, ok := resolveDir(dir); ok {
		return Resolved{Cmd: subst(cmd, host, port), Kind: kind, Dir: dir}, nil
	}

	return Resolved{}, fmt.Errorf("no launcher could serve %s (add a Procfile or use --cmd)", dir)
}

// subst replaces {port} and {host} placeholders. Procfile $PORT/$HOST are left
// untouched here and resolved from the environment by the shell at run time.
func subst(cmd, host string, port int) string {
	cmd = strings.ReplaceAll(cmd, "{port}", strconv.Itoa(port))
	cmd = strings.ReplaceAll(cmd, "{host}", host)
	return cmd
}

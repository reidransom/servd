// Package launcher resolves a project directory into a concrete command for
// starting its dev server.
//
// Resolution order (first match wins):
//
//  1. Manual override — a non-empty Cmd on the site (set via `servd add --cmd`).
//  2. .servd.toml — a `cmd = "..."` declared in the project directory itself.
//  3. Procfile / Procfile.dev — the "web" process (or first), run via $PORT/$HOST.
//  4. Launcher rules — declarative match-and-run rules, tried in order: user
//     rules from ~/.config/servd/launchers.toml first, then the built-in
//     defaults embedded from defaults.toml (jigyll, hugo, node, just, make,
//     static). See the Rule type; `servd launchers` prints the effective set.
//
// Port/host injection is unified across all kinds: the supervisor always
// exports PORT and HOST into the child environment, and this package also
// substitutes {port}/{host} placeholders in template commands. Procfiles use
// $PORT; rules use {port} flags; both work.
package launcher

import (
	"fmt"
	"github.com/reidransom/servd/internal/config"
	"os"
	"strconv"
	"strings"
)

// Resolved is a fully-resolved launch specification.
type Resolved struct {
	Cmd  string // shell command line with {port}/{host} substituted
	Kind string // launcher kind, e.g. "jigyll", "procfile", "manual"
	Dir  string // working directory (the project path)
}

// resolveDir resolves a directory without a manual override: .servd.toml,
// then Procfile, then the launcher rules. The returned cmd still contains
// {port}/{host} placeholders.
func resolveDir(dir string) (cmd, kind string, ok bool) {
	if cmd, ok := readProjectCmd(dir); ok {
		return cmd, "project", true
	}
	if entries := readProcfile(dir); len(entries) > 0 {
		if e, found := webProcess(entries); found {
			return e.Cmd, "procfile", true
		}
	}
	return matchRules(dir, EffectiveRules())
}

// Resolve produces the launch spec for a site, applying the precedence order.
func Resolve(site config.Site, settings config.Settings) (Resolved, error) {
	dir := site.Path
	host := settings.BindHost
	port := site.Port

	info, err := os.Stat(dir)
	if err != nil {
		return Resolved{}, fmt.Errorf("project path %s is unavailable: %w", dir, err)
	}
	if !info.IsDir() {
		return Resolved{}, fmt.Errorf("project path %s is not a directory", dir)
	}

	// 1. Manual override.
	if strings.TrimSpace(site.Cmd) != "" {
		return Resolved{Cmd: subst(site.Cmd, host, port), Kind: "manual", Dir: dir}, nil
	}

	// 2. .servd.toml, Procfile, then the launcher rules.
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

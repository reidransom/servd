package launcher

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// fileExists reports whether a regular file exists at dir/name.
func fileExists(dir, name string) bool {
	fi, err := os.Stat(filepath.Join(dir, name))
	return err == nil && !fi.IsDir()
}

// anyExists reports whether any of the named files exist in dir.
func anyExists(dir string, names ...string) bool {
	for _, n := range names {
		if fileExists(dir, n) {
			return true
		}
	}
	return false
}

// lookPath is exec.LookPath, swappable in tests.
var lookPath = exec.LookPath

// onPath reports whether a binary is resolvable on PATH.
func onPath(bin string) bool {
	_, err := lookPath(bin)
	return err == nil
}

// detectJigyll handles Jekyll-style sites (_config.yml). Prefers jigyll, falls
// back to jekyll. Binds explicitly so the proxy can reach it.
func detectJigyll(dir string) (cmd, kind string, ok bool) {
	if !anyExists(dir, "_config.yml", "_config.yaml") {
		return "", "", false
	}
	if onPath("jigyll") {
		return "jigyll serve -s . -H {host} -P {port} -w", "jigyll", true
	}
	if onPath("jekyll") {
		return "jekyll serve --host {host} --port {port} --watch", "jekyll", true
	}
	// Config present but no tool: still claim it so the user sees a clear error.
	return "jigyll serve -s . -H {host} -P {port} -w", "jigyll", true
}

// detectHugo handles Hugo sites.
func detectHugo(dir string) (cmd, kind string, ok bool) {
	if !anyExists(dir, "hugo.toml", "hugo.yaml", "hugo.json", "config.toml", "config.yaml", "config.json") {
		return "", "", false
	}
	// Disambiguate from non-Hugo config.toml: require a hugo binary AND a
	// content/ or layouts/ dir, OR an unambiguous hugo.* config.
	hugoConfig := anyExists(dir, "hugo.toml", "hugo.yaml", "hugo.json")
	looksHugo := dirExists(dir, "content") || dirExists(dir, "layouts") || dirExists(dir, "archetypes")
	if !hugoConfig && !looksHugo {
		return "", "", false
	}
	if !onPath("hugo") {
		return "", "", false
	}
	return "hugo serve --bind {host} -p {port}", "hugo", true
}

func dirExists(dir, name string) bool {
	fi, err := os.Stat(filepath.Join(dir, name))
	return err == nil && fi.IsDir()
}

// packageJSON is the subset of package.json we read.
type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

// frameworksTakingPortFlag are deps whose dev server accepts `--port`.
var frameworksTakingPortFlag = []string{"vite", "next", "astro", "nuxt", "@11ty/eleventy", "@sveltejs/kit", "svelte"}

// detectNode handles package.json projects.
func detectNode(dir string) (cmd, kind string, ok bool) {
	if !fileExists(dir, "package.json") {
		return "", "", false
	}
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return "", "", false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return "", "", false
	}
	script := ""
	for _, s := range []string{"dev", "serve", "start"} {
		if _, has := pkg.Scripts[s]; has {
			script = s
			break
		}
	}
	if script == "" {
		return "", "", false
	}
	base := "npm run " + script
	// Append a --port flag only for frameworks known to accept it; everything
	// else relies on the injected $PORT env var.
	if hasAnyDep(pkg, frameworksTakingPortFlag) {
		base += " -- --port {port}"
	}
	return base, "node:" + script, true
}

func hasAnyDep(pkg packageJSON, names []string) bool {
	for _, n := range names {
		if _, ok := pkg.Dependencies[n]; ok {
			return true
		}
		if _, ok := pkg.DevDependencies[n]; ok {
			return true
		}
	}
	return false
}

// detectJust handles a justfile with a `serve` recipe.
func detectJust(dir string) (cmd, kind string, ok bool) {
	for _, name := range []string{"justfile", "Justfile", ".justfile"} {
		if hasRecipe(filepath.Join(dir, name), "serve") && onPath("just") {
			return "just serve", "just", true
		}
	}
	return "", "", false
}

// detectMake handles a Makefile with a `serve` target.
func detectMake(dir string) (cmd, kind string, ok bool) {
	for _, name := range []string{"Makefile", "makefile", "GNUmakefile"} {
		if hasRecipe(filepath.Join(dir, name), "serve") && onPath("make") {
			return "make serve PORT={port}", "make", true
		}
	}
	return "", "", false
}

// hasRecipe reports whether a make/just file declares a target/recipe named
// `target` — a column-0 line like "serve:", "serve: deps" or "serve arg:".
// Variable assignments ("serve = x", "serve := x", "serve ?= x") don't count.
// Limitation: just recipes whose default arguments contain '=' before the
// colon are not recognized.
func hasRecipe(path, target string) bool {
	data, err := os.ReadFile(path)
	if err != nil {
		return false
	}
	for _, raw := range strings.Split(string(data), "\n") {
		// Targets are declared at column 0; indented lines are recipe bodies.
		if raw == "" || raw[0] == ' ' || raw[0] == '\t' || raw[0] == '#' {
			continue
		}
		line := strings.TrimRight(raw, " \t\r")
		if !strings.HasPrefix(line, target+":") &&
			!strings.HasPrefix(line, target+" ") &&
			!strings.HasPrefix(line, target+"\t") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		// '=' before the colon ("serve ?= a:b") or right after the colon run
		// ("serve := x", "serve ::= x") marks a variable assignment.
		if eq := strings.IndexByte(line, '='); eq >= 0 && eq < colon {
			continue
		}
		if strings.HasPrefix(strings.TrimLeft(line[colon:], ":"), "=") {
			continue
		}
		return true
	}
	return false
}

// detectStatic is the last-resort fallback: a directory with an index.html is
// served by servd's own built-in file server (no external dependency).
func detectStatic(dir string) (cmd, kind string, ok bool) {
	if !fileExists(dir, "index.html") {
		return "", "", false
	}
	self, err := os.Executable()
	if err != nil {
		self = "servd"
	}
	return shellQuote(self) + " __static --host {host} --port {port} --dir .", "static", true
}

// shellQuote wraps s in single quotes, escaping embedded single quotes, so it
// survives `sh -c`.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

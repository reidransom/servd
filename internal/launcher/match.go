// Match primitives for launcher rules. Everything here is framework-agnostic;
// the knowledge of specific tools and conventions lives in defaults.toml and
// the user's launchers.toml.

package launcher

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// lookPath is exec.LookPath, swappable in tests.
var lookPath = exec.LookPath

// onPath reports whether a binary is resolvable on PATH.
func onPath(bin string) bool {
	_, err := lookPath(bin)
	return err == nil
}

// anyGlob reports whether any pattern (a plain name or glob) matches an entry
// of the wanted type (file or directory) in dir.
func anyGlob(dir string, patterns []string, wantDir bool) bool {
	for _, p := range patterns {
		hits, err := filepath.Glob(filepath.Join(dir, p))
		if err != nil {
			continue
		}
		for _, h := range hits {
			if fi, err := os.Stat(h); err == nil && fi.IsDir() == wantDir {
				return true
			}
		}
	}
	return false
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

// packageJSON is the subset of package.json the npm_script predicate reads.
type packageJSON struct {
	Scripts         map[string]string `json:"scripts"`
	Dependencies    map[string]string `json:"dependencies"`
	DevDependencies map[string]string `json:"devDependencies"`
}

func readPackageJSON(dir string) (packageJSON, error) {
	var pkg packageJSON
	data, err := os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return pkg, err
	}
	err = json.Unmarshal(data, &pkg)
	return pkg, err
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

// selfExe is the shell-quoted path to the running servd binary, substituted
// for {self} in rule commands (used by the built-in static file server).
func selfExe() string {
	self, err := os.Executable()
	if err != nil {
		self = "servd"
	}
	return ShellQuote(self)
}

// ShellQuote wraps s in single quotes, escaping embedded single quotes, so it
// survives `sh -c`.
func ShellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

package launcher

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// stubLookPath makes onPath report exactly the given binaries as installed.
func stubLookPath(t *testing.T, available ...string) {
	t.Helper()
	set := map[string]bool{}
	for _, b := range available {
		set[b] = true
	}
	orig := lookPath
	lookPath = func(bin string) (string, error) {
		if set[bin] {
			return "/stub/" + bin, nil
		}
		return "", exec.ErrNotFound
	}
	t.Cleanup(func() { lookPath = orig })
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, name)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHasRecipe(t *testing.T) {
	cases := []struct {
		line string
		want bool
	}{
		{"serve:", true},
		{"serve: deps", true},
		{"serve install:", true}, // make multi-target line
		{"serve arg1:", true},    // just recipe with argument
		{"serve:\t", true},
		{"serve = x", false},
		{"serve:=x", false},
		{"serve := x", false},
		{"serve ::= x", false},
		{"serve ?= x", false},
		{"serve += x", false},
		{"serve ?= foo:bar", false},
		{"serve-prod:", false},
		{"# serve:", false},
		{"\tserve:", false}, // indented: recipe body, not a declaration
		{"  serve:", false},
		{"unrelated:", false},
	}
	dir := t.TempDir()
	for _, c := range cases {
		path := filepath.Join(dir, "Makefile")
		if err := os.WriteFile(path, []byte("other:\n\techo hi\n"+c.line+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if got := hasRecipe(path, "serve"); got != c.want {
			t.Errorf("hasRecipe(%q) = %v, want %v", c.line, got, c.want)
		}
	}
	if hasRecipe(filepath.Join(dir, "missing"), "serve") {
		t.Error("missing file should have no recipe")
	}
}

func TestDetectJigyll(t *testing.T) {
	dir := t.TempDir()
	if _, _, ok := detectJigyll(dir); ok {
		t.Fatal("no _config.yml should not be claimed")
	}
	writeFile(t, dir, "_config.yml", "title: x")

	stubLookPath(t, "jigyll")
	cmd, kind, ok := detectJigyll(dir)
	if !ok || kind != "jigyll" || !strings.HasPrefix(cmd, "jigyll serve") {
		t.Errorf("jigyll on path: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t, "jekyll")
	cmd, kind, ok = detectJigyll(dir)
	if !ok || kind != "jekyll" || !strings.HasPrefix(cmd, "jekyll serve") {
		t.Errorf("jekyll fallback: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t) // neither installed: still claimed, so the user sees an error
	_, kind, ok = detectJigyll(dir)
	if !ok || kind != "jigyll" {
		t.Errorf("no tool: got %q %v, want claimed as jigyll", kind, ok)
	}
}

func TestDetectNode(t *testing.T) {
	dir := t.TempDir()
	if _, _, ok := detectNode(dir); ok {
		t.Fatal("no package.json should not be claimed")
	}

	writeFile(t, dir, "package.json", `{"scripts":{"start":"node .","serve":"x","dev":"vite"}}`)
	cmd, kind, ok := detectNode(dir)
	if !ok || kind != "node:dev" || cmd != "npm run dev" {
		t.Errorf("dev preferred: got %q %q %v", cmd, kind, ok)
	}

	writeFile(t, dir, "package.json", `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"^5"}}`)
	cmd, _, ok = detectNode(dir)
	if !ok || cmd != "npm run dev -- --port {port}" {
		t.Errorf("vite port flag: got %q %v", cmd, ok)
	}

	writeFile(t, dir, "package.json", `{"scripts":{"build":"tsc"}}`)
	if _, _, ok := detectNode(dir); ok {
		t.Error("no dev/serve/start script should not be claimed")
	}

	writeFile(t, dir, "package.json", `{not json`)
	if _, _, ok := detectNode(dir); ok {
		t.Error("invalid JSON should not be claimed")
	}
}

func TestDetectJustAndMake(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "justfile", "serve:\n\tpython -m http.server\n")
	stubLookPath(t, "just", "make")
	if cmd, kind, ok := detectJust(dir); !ok || kind != "just" || cmd != "just serve" {
		t.Errorf("just: got %q %q %v", cmd, kind, ok)
	}

	mdir := t.TempDir()
	writeFile(t, mdir, "Makefile", "serve:\n\tpython -m http.server $(PORT)\n")
	if cmd, kind, ok := detectMake(mdir); !ok || kind != "make" || cmd != "make serve PORT={port}" {
		t.Errorf("make: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t) // tools missing: not claimed
	if _, _, ok := detectJust(dir); ok {
		t.Error("just not installed should not be claimed")
	}
	if _, _, ok := detectMake(mdir); ok {
		t.Error("make not installed should not be claimed")
	}
}

func TestDetectStatic(t *testing.T) {
	dir := t.TempDir()
	if _, _, ok := detectStatic(dir); ok {
		t.Fatal("no index.html should not be claimed")
	}
	writeFile(t, dir, "index.html", "<h1>hi</h1>")
	cmd, kind, ok := detectStatic(dir)
	if !ok || kind != "static" || !strings.Contains(cmd, "__static") {
		t.Errorf("static: got %q %q %v", cmd, kind, ok)
	}
}

func TestShellQuote(t *testing.T) {
	if got := ShellQuote("/a b/it's"); got != `'/a b/it'\''s'` {
		t.Errorf("ShellQuote: got %q", got)
	}
}

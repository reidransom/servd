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

// isolateConfig points XDG_CONFIG_HOME at a temp dir so tests never read the
// developer's real launchers.toml. Returns the servd config dir within it.
func isolateConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	cfg := filepath.Join(dir, "servd")
	if err := os.MkdirAll(cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	return cfg
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

// matchBuiltins runs the embedded default rules against dir.
func matchBuiltins(dir string) (cmd, kind string, ok bool) {
	return matchRules(dir, builtinRules())
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

func TestBuiltinJigyll(t *testing.T) {
	dir := t.TempDir()
	stubLookPath(t, "jigyll")
	if _, _, ok := matchBuiltins(dir); ok {
		t.Fatal("no _config.yml should not be claimed")
	}
	writeFile(t, dir, "_config.yml", "title: x")

	cmd, kind, ok := matchBuiltins(dir)
	if !ok || kind != "jigyll" || !strings.HasPrefix(cmd, "jigyll serve") {
		t.Errorf("jigyll on path: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t, "jekyll")
	cmd, kind, ok = matchBuiltins(dir)
	if !ok || kind != "jekyll" || !strings.HasPrefix(cmd, "jekyll serve") {
		t.Errorf("jekyll fallback: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t) // neither installed: still claimed, so the user sees an error
	_, kind, ok = matchBuiltins(dir)
	if !ok || kind != "jigyll" {
		t.Errorf("no tool: got %q %v, want claimed as jigyll", kind, ok)
	}
}

func TestBuiltinHugo(t *testing.T) {
	dir := t.TempDir()
	stubLookPath(t, "hugo")
	writeFile(t, dir, "hugo.toml", "baseURL = '/'")
	cmd, kind, ok := matchBuiltins(dir)
	if !ok || kind != "hugo" || !strings.HasPrefix(cmd, "hugo serve") {
		t.Errorf("hugo.toml: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t) // hugo not installed: not claimed
	if _, _, ok := matchBuiltins(dir); ok {
		t.Error("hugo not installed should not be claimed")
	}

	// A bare config.toml is ambiguous: needs a telltale dir to count as Hugo.
	stubLookPath(t, "hugo")
	legacy := t.TempDir()
	writeFile(t, legacy, "config.toml", "baseURL = '/'")
	if _, _, ok := matchBuiltins(legacy); ok {
		t.Error("bare config.toml should not be claimed")
	}
	if err := os.MkdirAll(filepath.Join(legacy, "content"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, kind, ok = matchBuiltins(legacy)
	if !ok || kind != "hugo" {
		t.Errorf("config.toml + content/: got %q %v, want hugo", kind, ok)
	}
}

func TestBuiltinNode(t *testing.T) {
	dir := t.TempDir()
	stubLookPath(t)
	if _, _, ok := matchBuiltins(dir); ok {
		t.Fatal("no package.json should not be claimed")
	}

	writeFile(t, dir, "package.json", `{"scripts":{"start":"node .","serve":"x","dev":"vite"}}`)
	cmd, kind, ok := matchBuiltins(dir)
	if !ok || kind != "node:dev" || cmd != "npm run dev" {
		t.Errorf("dev preferred: got %q %q %v", cmd, kind, ok)
	}

	writeFile(t, dir, "package.json", `{"scripts":{"dev":"vite"},"devDependencies":{"vite":"^5"}}`)
	cmd, _, ok = matchBuiltins(dir)
	if !ok || cmd != "npm run dev -- --port {port}" {
		t.Errorf("vite port flag: got %q %v", cmd, ok)
	}

	writeFile(t, dir, "package.json", `{"scripts":{"build":"tsc"}}`)
	if _, _, ok := matchBuiltins(dir); ok {
		t.Error("no dev/serve/start script should not be claimed")
	}

	writeFile(t, dir, "package.json", `{not json`)
	if _, _, ok := matchBuiltins(dir); ok {
		t.Error("invalid JSON should not be claimed")
	}
}

func TestBuiltinJustAndMake(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "justfile", "serve:\n\tpython -m http.server\n")
	stubLookPath(t, "just", "make")
	if cmd, kind, ok := matchBuiltins(dir); !ok || kind != "just" || cmd != "just serve" {
		t.Errorf("just: got %q %q %v", cmd, kind, ok)
	}

	mdir := t.TempDir()
	writeFile(t, mdir, "Makefile", "serve:\n\tpython -m http.server $(PORT)\n")
	if cmd, kind, ok := matchBuiltins(mdir); !ok || kind != "make" || cmd != "make serve PORT={port}" {
		t.Errorf("make: got %q %q %v", cmd, kind, ok)
	}

	stubLookPath(t) // tools missing: not claimed
	if _, _, ok := matchBuiltins(dir); ok {
		t.Error("just not installed should not be claimed")
	}
	if _, _, ok := matchBuiltins(mdir); ok {
		t.Error("make not installed should not be claimed")
	}
}

func TestBuiltinStatic(t *testing.T) {
	dir := t.TempDir()
	stubLookPath(t)
	if _, _, ok := matchBuiltins(dir); ok {
		t.Fatal("no index.html should not be claimed")
	}
	writeFile(t, dir, "index.html", "<h1>hi</h1>")
	cmd, kind, ok := matchBuiltins(dir)
	if !ok || kind != "static" || !strings.Contains(cmd, "__static") {
		t.Errorf("static: got %q %q %v", cmd, kind, ok)
	}
}

func TestUserRulesOverrideAndDisable(t *testing.T) {
	cfg := isolateConfig(t)
	stubLookPath(t, "make")
	writeFile(t, cfg, "launchers.toml", `
[[launcher]]
name = "make"
recipe = { files = ["Makefile"], target = "serve" }
bin = "make"
cmd = "make dev-serve HOST={host} PORT={port}"

[[launcher]]
name = "static"
disabled = true
`)

	dir := t.TempDir()
	writeFile(t, dir, "Makefile", "serve:\n\techo hi\n")
	cmd, kind, ok := matchRules(dir, EffectiveRules())
	if !ok || kind != "make" || cmd != "make dev-serve HOST={host} PORT={port}" {
		t.Errorf("override: got %q %q %v", cmd, kind, ok)
	}

	sdir := t.TempDir()
	writeFile(t, sdir, "index.html", "<h1>hi</h1>")
	if _, _, ok := matchRules(sdir, EffectiveRules()); ok {
		t.Error("disabled static rule should not claim index.html")
	}
}

func TestUserRuleBeforeBuiltins(t *testing.T) {
	cfg := isolateConfig(t)
	stubLookPath(t, "middleman", "just")
	writeFile(t, cfg, "launchers.toml", `
[[launcher]]
name = "middleman"
matches = { file = "Gemfile", regex = "middleman" }
bin = "middleman"
cmd = "middleman serve -p {port}"
`)

	dir := t.TempDir()
	writeFile(t, dir, "Gemfile", `gem "middleman"`)
	writeFile(t, dir, "justfile", "serve:\n\techo hi\n") // built-in would claim this
	cmd, kind, ok := matchRules(dir, EffectiveRules())
	if !ok || kind != "middleman" || cmd != "middleman serve -p {port}" {
		t.Errorf("user rule should win: got %q %q %v", cmd, kind, ok)
	}
}

func TestInvalidRulesNeverMatch(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "index.html", "x")
	stubLookPath(t, "sh")
	rules := []Rule{
		{Name: "bin-only", Bin: "sh", Cmd: "sh run"},     // no dir predicate
		{Name: "no-cmd", Exists: []string{"index.html"}}, // no command
		{Cmd: "x", Exists: []string{"index.html"}},       // no name
		{Name: "off", Disabled: true, Exists: []string{"index.html"}, Cmd: "x"},
	}
	if cmd, kind, ok := matchRules(dir, rules); ok {
		t.Errorf("invalid rules matched: %q %q", cmd, kind)
	}
}

func TestMalformedUserRulesIgnored(t *testing.T) {
	cfg := isolateConfig(t)
	writeFile(t, cfg, "launchers.toml", "not valid toml [[[")
	stubLookPath(t, "just")
	dir := t.TempDir()
	writeFile(t, dir, "justfile", "serve:\n\techo hi\n")
	if _, kind, ok := matchRules(dir, EffectiveRules()); !ok || kind != "just" {
		t.Errorf("builtins should still apply: got %q %v", kind, ok)
	}
}

func TestEffectiveRulesMerge(t *testing.T) {
	cfg := isolateConfig(t)
	writeFile(t, cfg, "launchers.toml", `
[[launcher]]
name = "mine"
exists = ["x"]
cmd = "x"

[[launcher]]
name = "hugo"
exists = ["hugo.toml"]
cmd = "custom-hugo"
`)
	rules := EffectiveRules()
	if len(rules) != len(builtinRules())+1 {
		t.Fatalf("got %d rules, want builtins+1", len(rules))
	}
	if rules[0].Name != "mine" || rules[1].Name != "hugo" {
		t.Errorf("user rules should come first: got %q, %q", rules[0].Name, rules[1].Name)
	}
	for _, r := range rules[2:] {
		if r.Name == "hugo" {
			t.Error("overridden builtin hugo should be dropped")
		}
	}
}

func TestTools(t *testing.T) {
	got := Tools([]Rule{
		{Name: "a", Bin: "hugo", Cmd: "hugo serve"},
		{Name: "b", Cmd: "npm run dev"},
		{Name: "c", Cmd: "{self} __static"}, // placeholder: not a tool
		{Name: "d", Bin: "hugo", Cmd: "hugo serve"},
	})
	want := []string{"hugo", "npm"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("Tools = %v, want %v", got, want)
	}
}

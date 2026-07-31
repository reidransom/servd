package launcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestSubst(t *testing.T) {
	got := subst("serve -H {host} -P {port} --url http://{host}:{port}/ $PORT", "127.0.0.1", 4001)
	want := "serve -H 127.0.0.1 -P 4001 --url http://127.0.0.1:4001/ $PORT"
	if got != want {
		t.Errorf("subst:\n got %q\nwant %q", got, want)
	}
}

func TestShellJoin(t *testing.T) {
	cases := []struct {
		argv []string
		want string
	}{
		{[]string{"npm", "run", "dev"}, "npm run dev"},
		{[]string{"jigyll", "serve", "-P", "{port}"}, "jigyll serve -P {port}"},
		{[]string{"echo", "hello world"}, "echo 'hello world'"},
		{[]string{"sh", "-c", "a && b"}, `sh -c 'a && b'`},
		{[]string{"say", "it's"}, `say 'it'\''s'`},
	}
	for _, c := range cases {
		if got := ShellJoin(c.argv); got != c.want {
			t.Errorf("ShellJoin(%q):\n got %q\nwant %q", c.argv, got, c.want)
		}
	}
}

func testSettings() config.Settings {
	return config.Settings{BindHost: "127.0.0.1", PortRangeStart: 42101, Hostnames: config.HostnameSettings{HTTPPort: 42100}}
}

func TestResolveManualOverridesProcfile(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Procfile"), []byte("web: from-procfile"), 0o644); err != nil {
		t.Fatal(err)
	}
	site := config.Site{Slug: "x", Path: dir, Port: 4001, Cmd: "manual -p {port}"}
	res, err := Resolve(site, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "manual" || res.Cmd != "manual -p 4001" {
		t.Errorf("got kind=%q cmd=%q, want manual override", res.Kind, res.Cmd)
	}
}

func TestResolveProcfile(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	procfile := "worker: node worker.js\nweb: npm start\n"
	if err := os.WriteFile(filepath.Join(dir, "Procfile"), []byte(procfile), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "procfile" || res.Cmd != "npm start" {
		t.Errorf("got kind=%q cmd=%q, want procfile web entry", res.Kind, res.Cmd)
	}
}

func TestResolveNothingServable(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	_, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err == nil {
		t.Fatal("empty dir should not resolve")
	}
	if !strings.Contains(err.Error(), "--cmd") {
		t.Errorf("error should mention --cmd escape hatch: %v", err)
	}
}

func TestResolveProjectConfigBeatsProcfile(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	writeFile(t, dir, "Procfile", "web: from-procfile")
	writeFile(t, dir, ".servd.toml", `cmd = "from-project -p {port}"`)
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "project" || res.Cmd != "from-project -p 4001" {
		t.Errorf("got kind=%q cmd=%q, want .servd.toml override", res.Kind, res.Cmd)
	}
}

func TestResolveProjectConfigInvalidFallsThrough(t *testing.T) {
	isolateConfig(t)
	dir := t.TempDir()
	writeFile(t, dir, ".servd.toml", "not toml [[[")
	writeFile(t, dir, "Procfile", "web: from-procfile")
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kind != "procfile" || res.Cmd != "from-procfile" {
		t.Errorf("got kind=%q cmd=%q, want procfile fallback", res.Kind, res.Cmd)
	}
}

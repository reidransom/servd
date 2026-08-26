package launcher

import (
	"os"
	"path/filepath"
	"runtime"
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

func testSettings() config.Settings {
	return config.Settings{BindHost: "127.0.0.1", PortRangeStart: 42101, Hostnames: config.HostnameSettings{HTTPPort: 42100}}
}

func TestResolveExplicitCommandWinsWithoutReadingRepositoryConfig(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".servd.toml", "not toml [[[")
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001, Cmd: "manual -p {port}"}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != "explicit" || res.Cmd != "manual -p 4001" {
		t.Errorf("got source=%q cmd=%q, want explicit command", res.Source, res.Cmd)
	}
}

func TestResolveRepositoryCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".servd.toml", "extra = true\ncmd = \"from-project -H {host} -p {port}\"")
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != ".servd.toml" || res.Cmd != "from-project -H 127.0.0.1 -p 4001" {
		t.Errorf("got source=%q cmd=%q, want repository command", res.Source, res.Cmd)
	}
}

func TestResolveWhitespaceSiteCommandUsesRepositoryCommand(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, ".servd.toml", `cmd = "from-project"`)
	res, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001, Cmd: " \t\n "}, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if res.Source != ".servd.toml" || res.Cmd != "from-project" {
		t.Errorf("got source=%q cmd=%q, want repository command", res.Source, res.Cmd)
	}
}

func TestResolveIgnoresDiscoverySources(t *testing.T) {
	parent := t.TempDir()
	writeFile(t, parent, ".servd.toml", `cmd = "parent"`)
	dir := filepath.Join(parent, "child")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, content := range map[string]string{
		"Procfile":     "web: ignored",
		"Procfile.dev": "web: ignored",
		"_config.yml":  "ignored",
		"Gemfile":      "ignored",
		"hugo.toml":    "ignored",
		"index.html":   "ignored",
		"justfile":     "serve:\n\tignored",
		"Makefile":     "serve:\n\tignored",
		"package.json": `{"scripts":{"dev":"ignored"}}`,
		"recipe":       "ignored",
	} {
		writeFile(t, dir, name, content)
	}

	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	launcherConfig := filepath.Join(configHome, "servd", "launchers.toml")
	const launcherConfigContent = "[[launcher]]\nname = \"ignored\"\ncmd = \"ignored\""
	writeFile(t, filepath.Dir(launcherConfig), filepath.Base(launcherConfig), launcherConfigContent)

	binDir := t.TempDir()
	binary := "hugo"
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}
	writeFile(t, binDir, binary, "")
	t.Setenv("PATH", binDir)

	_, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err == nil || !strings.Contains(err.Error(), "no command configured") {
		t.Fatalf("resolution error = %v, want missing command", err)
	}

	data, readErr := os.ReadFile(launcherConfig)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != launcherConfigContent {
		t.Errorf("launchers.toml was modified: got %q, want %q", data, launcherConfigContent)
	}
}

func TestResolveRepositoryCommandErrors(t *testing.T) {
	cases := []struct {
		name     string
		setup    func(t *testing.T, dir string)
		contains string
	}{
		{"missing", func(t *testing.T, dir string) {}, "no command configured"},
		{"unreadable", func(t *testing.T, dir string) {
			if err := os.Mkdir(filepath.Join(dir, ".servd.toml"), 0o755); err != nil {
				t.Fatal(err)
			}
		}, "cannot read repository command file"},
		{"malformed", func(t *testing.T, dir string) { writeFile(t, dir, ".servd.toml", "not toml [[[") }, "invalid repository command file"},
		{"non-string", func(t *testing.T, dir string) { writeFile(t, dir, ".servd.toml", "cmd = 42") }, "non-string cmd"},
		{"missing cmd", func(t *testing.T, dir string) { writeFile(t, dir, ".servd.toml", "other = true") }, "has no cmd"},
		{"blank cmd", func(t *testing.T, dir string) { writeFile(t, dir, ".servd.toml", `cmd = " \t"`) }, "blank cmd"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.setup(t, dir)
			_, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
			if err == nil || !strings.Contains(err.Error(), tc.contains) {
				t.Fatalf("resolution error = %v, want %q", err, tc.contains)
			}
		})
	}
}

func TestResolveMissingCommandIncludesRemediation(t *testing.T) {
	dir := t.TempDir()
	_, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err == nil {
		t.Fatal("missing command resolved")
	}
	for _, want := range []string{dir, "create " + filepath.Join(dir, ".servd.toml") + " with a nonblank cmd", "servd rm x", "servd add " + dir + " -- <command>"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

func TestResolveRejectsMissingProjectPathWithExplicitCommand(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	_, err := Resolve(config.Site{Slug: "x", Path: path, Port: 4001, Cmd: "serve"}, testSettings())
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("resolution error = %v, want unavailable path", err)
	}
}

func TestResolveRejectsProjectPathThatIsNotDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project-file")
	if err := os.WriteFile(path, []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Resolve(config.Site{Slug: "x", Path: path, Port: 4001, Cmd: "serve"}, testSettings())
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Fatalf("resolution error = %v, want non-directory error", err)
	}
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

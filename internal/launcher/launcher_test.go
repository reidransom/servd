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

func testSettings() config.Settings {
	return config.Settings{BindHost: "127.0.0.1", PortRangeStart: 42101, ProxyPort: 42100}
}

func TestResolveManualOverridesProcfile(t *testing.T) {
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
	dir := t.TempDir()
	_, err := Resolve(config.Site{Slug: "x", Path: dir, Port: 4001}, testSettings())
	if err == nil {
		t.Fatal("empty dir should not resolve")
	}
	if !strings.Contains(err.Error(), "--cmd") {
		t.Errorf("error should mention --cmd escape hatch: %v", err)
	}
}

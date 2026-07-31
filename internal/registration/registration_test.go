package registration

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"My Site!", "my-site"},
		{"acme", "acme"},
		{"--x--", "x"},
		{"client_2 (new)", "client-2-new"},
		{"日本語", "site"},
		{"", "site"},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func testSettings() config.Settings {
	// High port range so host-port probes don't collide with real services.
	return config.Settings{PortRangeStart: 42101, ProxyPort: 42100, BindHost: "127.0.0.1"}
}

func mkSite(t *testing.T, root string, parts ...string) {
	t.Helper()
	dir := filepath.Join(append([]string{root}, parts...)...)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>hi</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAddSite(t *testing.T) {
	root := t.TempDir()
	mkSite(t, root, "gamma")
	dir := filepath.Join(root, "gamma")

	reg := &config.Registry{}
	site, err := AddSite(reg, testSettings(), AddParams{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if site.Slug != "gamma" || site.Port != 42101 || site.Launcher == "" {
		t.Errorf("got %+v, want slug=gamma port=42101 launcher!=\"\"", site)
	}
	if len(reg.Sites) != 1 {
		t.Fatalf("registry: got %d sites", len(reg.Sites))
	}

	// Duplicate path is rejected.
	if _, err := AddSite(reg, testSettings(), AddParams{Path: dir}); err == nil {
		t.Error("duplicate path: want error, got nil")
	}
	// Duplicate slug (different path) is rejected.
	mkSite(t, root, "other")
	if _, err := AddSite(reg, testSettings(), AddParams{Path: filepath.Join(root, "other"), Slug: "gamma"}); err == nil {
		t.Error("duplicate slug: want error, got nil")
	}
}

func TestAddSiteUndetectable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	reg := &config.Registry{}
	// No servable markers and no command → error.
	if _, err := AddSite(reg, testSettings(), AddParams{Path: dir}); err == nil {
		t.Error("undetectable dir without cmd: want error, got nil")
	}
	// An explicit command makes it registrable.
	site, err := AddSite(reg, testSettings(), AddParams{Path: dir, Cmd: "python -m http.server {port}"})
	if err != nil {
		t.Fatal(err)
	}
	if site.Cmd == "" {
		t.Errorf("got %+v, want Cmd preserved", site)
	}
}

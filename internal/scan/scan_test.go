package scan

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

func TestUniqueSlug(t *testing.T) {
	reg := &config.Registry{Sites: []config.Site{{Slug: "acme"}, {Slug: "acme-2"}}}
	if got := uniqueSlug("fresh", reg); got != "fresh" {
		t.Errorf("free base: got %q", got)
	}
	if got := uniqueSlug("acme", reg); got != "acme-3" {
		t.Errorf("taken base and -2: got %q, want acme-3", got)
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

func TestScan(t *testing.T) {
	root := t.TempDir()
	mkSite(t, root, "alpha")
	mkSite(t, root, "beta")
	mkSite(t, root, "node_modules")  // skipped name
	mkSite(t, root, ".hidden")       // dot dir skipped
	mkSite(t, root, "d1", "d2", "d3") // depth 3: beyond maxDepth

	reg := &config.Registry{}
	added, err := Scan(root, reg, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 2 {
		t.Fatalf("got %d added (%v), want 2", len(added), added)
	}
	if added[0].Slug != "alpha" || added[1].Slug != "beta" {
		t.Errorf("slugs: got %v", added)
	}
	if added[0].Port != 42101 || added[1].Port != 42102 {
		t.Errorf("ports: got %d, %d want 42101, 42102", added[0].Port, added[1].Port)
	}
	if len(reg.Sites) != 2 {
		t.Errorf("registry: got %d sites", len(reg.Sites))
	}

	// Re-scan finds nothing new.
	again, err := Scan(root, reg, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("re-scan: got %v, want none", again)
	}
}

func TestWalkDoesNotDescendIntoProjects(t *testing.T) {
	root := t.TempDir()
	mkSite(t, root, "proj")
	// A nested servable dir inside a project must not become its own site.
	mkSite(t, root, "proj", "docs")

	reg := &config.Registry{}
	added, err := Scan(root, reg, testSettings())
	if err != nil {
		t.Fatal(err)
	}
	if len(added) != 1 || added[0].Slug != "proj" {
		t.Errorf("got %v, want only proj", added)
	}
}

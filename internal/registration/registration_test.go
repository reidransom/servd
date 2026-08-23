package registration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func testSettings() config.Settings {
	settings := config.DefaultSettings()
	settings.PortRangeStart = 42101
	settings.Hostnames.HTTPPort = 42100
	return settings
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

	if _, err := AddSite(reg, testSettings(), AddParams{Path: dir}); err == nil {
		t.Error("duplicate path: want error, got nil")
	}
	mkSite(t, root, "other")
	if _, err := AddSite(reg, testSettings(), AddParams{Path: filepath.Join(root, "other"), Slug: "gamma"}); err == nil {
		t.Error("duplicate slug: want error, got nil")
	}
}

func TestAddSiteInfersPackageName(t *testing.T) {
	root := t.TempDir()
	mkSite(t, root, "directory-name")
	dir := filepath.Join(root, "directory-name")
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"@scope/Acme App"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	site, err := AddSite(&config.Registry{}, testSettings(), AddParams{Path: dir})
	if err != nil {
		t.Fatal(err)
	}
	if site.Slug != "acme-app" {
		t.Fatalf("slug = %q, want package-derived acme-app", site.Slug)
	}
}

func TestAddSiteValidatesSlugAndHostPrefix(t *testing.T) {
	root := t.TempDir()
	mkSite(t, root, "site")
	dir := filepath.Join(root, "site")
	settings := testSettings()

	site, err := AddSite(&config.Registry{}, settings, AddParams{Path: dir, Slug: "acme", HostPrefix: "auth"})
	if err != nil {
		t.Fatal(err)
	}
	if site.Slug != "acme" || site.HostPrefix != "auth" {
		t.Fatalf("site identity = %+v", site)
	}
	if _, err := AddSite(&config.Registry{}, settings, AddParams{Path: dir, Slug: "Acme App"}); err == nil || !strings.Contains(err.Error(), "try \"acme-app\"") {
		t.Fatalf("invalid explicit slug error = %v", err)
	}
	if _, err := AddSite(&config.Registry{}, settings, AddParams{Path: dir, HostPrefix: "auth", NoWorktreePrefix: true}); err == nil {
		t.Fatal("conflicting prefix options succeeded")
	}
	withoutPrefix, err := AddSite(&config.Registry{}, settings, AddParams{Path: dir, NoWorktreePrefix: true})
	if err != nil {
		t.Fatal(err)
	}
	if withoutPrefix.HostPrefix != "" {
		t.Fatalf("suppressed prefix = %q", withoutPrefix.HostPrefix)
	}
}

func TestAddSiteUndetectable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "empty")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}

	reg := &config.Registry{}
	if _, err := AddSite(reg, testSettings(), AddParams{Path: dir}); err == nil {
		t.Error("undetectable dir without cmd: want error, got nil")
	}
	site, err := AddSite(reg, testSettings(), AddParams{Path: dir, Cmd: "python -m http.server {port}"})
	if err != nil {
		t.Fatal(err)
	}
	if site.Cmd == "" {
		t.Errorf("got %+v, want Cmd preserved", site)
	}
}

func TestRenameSite(t *testing.T) {
	reg := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Path: "/alpha", Port: 42101, Cmd: "serve"},
		{Slug: "beta", Path: "/beta", Port: 42102},
	}}

	renamed, err := RenameSite(reg, "alpha", "gamma")
	if err != nil {
		t.Fatal(err)
	}
	if renamed.Slug != "gamma" || renamed.Path != "/alpha" || renamed.Port != 42101 || renamed.Cmd != "serve" {
		t.Fatalf("renamed site = %+v", renamed)
	}
	if reg.Find("alpha") != nil || reg.Find("gamma") == nil {
		t.Fatalf("registry after rename = %+v", reg.Sites)
	}
}

func TestRenameSiteRejectsInvalidOrDuplicateSlug(t *testing.T) {
	for _, newSlug := range []string{"Bad Slug", "beta"} {
		reg := &config.Registry{Sites: []config.Site{{Slug: "alpha"}, {Slug: "beta"}}}
		if _, err := RenameSite(reg, "alpha", newSlug); err == nil {
			t.Errorf("rename to %q succeeded", newSlug)
		}
		if reg.Find("alpha") == nil {
			t.Errorf("rename to %q mutated registry after error: %+v", newSlug, reg.Sites)
		}
	}
}

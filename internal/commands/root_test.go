package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/spf13/cobra"
)

func TestSelectSitesAllReturnsEveryRegisteredSite(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	sites, err := selectSites(registry, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].Slug != "alpha" || sites[1].Slug != "bravo" {
		t.Fatalf("selectSites --all = %#v, want alpha and bravo", sites)
	}
	sites[0].Slug = "changed"
	if registry.Sites[0].Slug != "alpha" {
		t.Fatalf("selectSites --all mutated registry: %#v", registry.Sites)
	}
}

func TestSelectSitesExplicitSlugsAndErrors(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	sites, err := selectSites(registry, []string{"bravo"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Slug != "bravo" {
		t.Fatalf("selectSites bravo = %#v, want only bravo", sites)
	}

	_, err = selectSites(registry, []string{"missing"}, false)
	if err == nil {
		t.Fatal("selectSites accepted an unknown target")
	}
}

func TestRemovedAndStaticCommandSurface(t *testing.T) {
	root := newRootCmd()
	var staticFound, legacyFound, launchersFound bool
	for _, command := range root.Commands() {
		switch command.Name() {
		case "static":
			staticFound = true
			if command.Hidden {
				t.Fatal("static command is hidden")
			}
		case "__static":
			legacyFound = true
		case "launchers":
			launchersFound = true
		}
	}
	if !staticFound {
		t.Fatal("static command is not registered")
	}
	if legacyFound {
		t.Fatal("legacy __static alias is still registered")
	}
	if launchersFound {
		t.Fatal("removed launchers command is still registered")
	}
}

func TestBulkUpAndRestartReportEveryFailure(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := config.MutateRegistry(func(registry *config.Registry) error {
		registry.Sites = []config.Site{
			{Slug: "first", Path: "/missing/first", Port: 4101, Cmd: "echo first"},
			{Slug: "second", Path: "/missing/second", Port: 4102, Cmd: "echo second"},
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	for _, command := range []*cobra.Command{newUpCmd(), newRestartCmd()} {
		command.SetArgs([]string{"--all"})
		err := command.Execute()
		if err == nil || !strings.Contains(err.Error(), "2 site(s) failed") {
			t.Fatalf("%s --all error = %v, want two failed sites", command.Name(), err)
		}
	}
}

func TestCurrentDirectoryTargetingBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name   string
		args   []string
		marker bool
		child  bool
		err    string
	}{
		{name: "up unregistered", args: []string{"up"}, marker: true, err: "not registered"},
		{name: "status unregistered", args: []string{"status"}, marker: true, err: "not registered"},
		{name: "all bypasses cwd", args: []string{"up", "--all"}, marker: true},
		{name: "explicit bypasses cwd", args: []string{"up", "missing"}, marker: true, err: "unknown site"},
		{name: "status without config", args: []string{"status"}},
		{name: "status ignores parent config", args: []string{"status"}, marker: true, child: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			t.Setenv("XDG_STATE_HOME", t.TempDir())
			dir := t.TempDir()
			if tc.marker {
				if err := os.WriteFile(filepath.Join(dir, ".servd.toml"), []byte(`cmd = "serve"`), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if tc.child {
				dir = filepath.Join(dir, "child")
				if err := os.Mkdir(dir, 0o755); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			cmd := newRootCmd()
			cmd.SetOut(&bytes.Buffer{})
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs(tc.args)
			err := cmd.Execute()
			if tc.err == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.err) {
				t.Fatalf("%v error = %v, want %q", tc.args, err, tc.err)
			}
		})
	}
}

func TestFindSiteByDirectoryPrefersCanonicalRegistration(t *testing.T) {
	directory := t.TempDir()
	canonical, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatal(err)
	}
	newAlias := func(name string) string {
		path := filepath.Join(t.TempDir(), name)
		if err := os.Symlink(directory, path); err != nil {
			if runtime.GOOS == "windows" {
				t.Skipf("directory symlinks unavailable: %v", err)
			}
			t.Fatal(err)
		}
		return path
	}
	first := newAlias("first")
	second := newAlias("second")
	target := newAlias("target")
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "first", Path: first},
		{Slug: "second", Path: second},
		{Slug: "direct", Path: canonical},
	}}

	site, err := findSiteByDirectory(registry, target)
	if err != nil {
		t.Fatal(err)
	}
	if site == nil || site.Slug != "direct" {
		t.Fatalf("findSiteByDirectory(%q) = %#v, want direct registration", target, site)
	}
}

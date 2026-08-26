package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
)

func TestStatusSites(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	all, err := statusSites(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("statusSites without slug returned %d sites, want 2", len(all))
	}

	sites, err := statusSites(registry, []string{"bravo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Slug != "bravo" {
		t.Fatalf("statusSites for bravo = %#v, want only bravo", sites)
	}

	_, err = statusSites(registry, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), `unknown site "missing"`) {
		t.Fatalf("statusSites missing error = %v, want unknown-site error", err)
	}
}

func TestStatusCommandAcceptsAtMostOneSlug(t *testing.T) {
	command := newStatusCmd()
	if err := command.Args(command, []string{"alpha", "bravo"}); err == nil {
		t.Fatal("status accepted multiple slugs")
	}
}

func TestAddCommandVectorReportsSourceAndWhich(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	project := filepath.Join(t.TempDir(), "project")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	vector := []string{"echo", "hello world", "&&"}
	wantCommand := launcher.ShellJoin(vector)

	var addOutput bytes.Buffer
	add := newAddCmd()
	add.SetOut(&addOutput)
	add.SetArgs(append([]string{"--slug", "example", project, "--"}, vector...))
	if err := add.Execute(); err != nil {
		t.Fatalf("add explicit command: %v", err)
	}
	if got := addOutput.String(); !strings.Contains(got, "source: explicit") || !strings.Contains(got, "command: "+wantCommand) {
		t.Fatalf("add output = %q, want explicit source and %q", got, wantCommand)
	}

	registry, err := config.LoadRegistry()
	if err != nil {
		t.Fatal(err)
	}
	if site := registry.Find("example"); site == nil || site.Cmd != wantCommand {
		t.Fatalf("registered site = %#v, want command %q", site, wantCommand)
	}

	var whichOutput bytes.Buffer
	which := newWhichCmd()
	which.SetOut(&whichOutput)
	which.SetArgs([]string{"example"})
	if err := which.Execute(); err != nil {
		t.Fatalf("which explicit site: %v", err)
	}
	if got, want := whichOutput.String(), "source: explicit\ncommand: "+wantCommand+"\n"; got != want {
		t.Fatalf("which output = %q, want %q", got, want)
	}
}

func TestAddRejectsRemovedCmdFlagAndInvalidCommandVector(t *testing.T) {
	t.Run("removed cmd flag", func(t *testing.T) {
		command := newAddCmd()
		command.SetArgs([]string{"--cmd", "echo", "."})
		if err := command.Execute(); err == nil || !strings.Contains(err.Error(), "unknown flag: --cmd") {
			t.Fatalf("add --cmd error = %v, want unknown flag", err)
		}
	})

	for _, tc := range []struct {
		name string
		args []string
		want string
	}{
		{name: "missing command", args: []string{".", "--"}, want: "no command given after --"},
		{name: "multiple paths", args: []string{".", "other", "--", "echo"}, want: "expected exactly one <path> before --"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newAddCmd()
			command.SetArgs(tc.args)
			if err := command.Execute(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("add %v error = %v, want %q", tc.args, err, tc.want)
			}
		})
	}
}
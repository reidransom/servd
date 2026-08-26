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

func TestStatusCommandIsolatesInvalidSite(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	project := t.TempDir()
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "valid", Path: project, Port: 4001, Cmd: "sleep 30"},
		{Slug: "invalid", Path: project, Port: 4002},
	}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}

	var aggregate bytes.Buffer
	all := newStatusCmd()
	all.SetOut(&aggregate)
	all.SetErr(&bytes.Buffer{})
	all.SilenceErrors, all.SilenceUsage = true, true
	if err := all.Execute(); err != nil {
		t.Fatalf("aggregate status = %v", err)
	}
	if got := aggregate.String(); !strings.Contains(got, "valid") || !strings.Contains(got, "invalid") || !strings.Contains(got, "error") {
		t.Errorf("aggregate status = %q, want both rows and the invalid error", got)
	}

	var named bytes.Buffer
	one := newStatusCmd()
	one.SetOut(&named)
	one.SetErr(&bytes.Buffer{})
	one.SilenceErrors, one.SilenceUsage = true, true
	one.SetArgs([]string{"invalid"})
	if err := one.Execute(); err == nil || !strings.Contains(err.Error(), "no command configured") {
		t.Fatalf("named invalid status error = %v, want command-resolution error", err)
	}
	if got := named.String(); !strings.Contains(got, "invalid") || !strings.Contains(got, "error") {
		t.Errorf("named status = %q, want rendered invalid row", got)
	}

	var structured bytes.Buffer
	jsonStatus := newStatusCmd()
	jsonStatus.SetOut(&structured)
	jsonStatus.SetErr(&bytes.Buffer{})
	jsonStatus.SilenceErrors, jsonStatus.SilenceUsage = true, true
	jsonStatus.SetArgs([]string{"--json", "invalid"})
	if err := jsonStatus.Execute(); err == nil {
		t.Fatal("named JSON status should return a resolution error")
	}
	if got := structured.String(); !strings.Contains(got, `"status": "error"`) || !strings.Contains(got, `"error":`) {
		t.Errorf("structured status = %q, want status and error", got)
	} else if strings.Contains(got, `"launcher"`) || strings.Contains(got, `"source"`) {
		t.Errorf("structured status exposes removed command metadata: %s", got)
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

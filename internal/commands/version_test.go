package commands

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/reidransom/servd/internal/buildinfo"
)

func TestVersionInvocationsMatch(t *testing.T) {
	want := buildinfo.String() + "\n"
	for _, args := range [][]string{{"--version"}, {"version"}} {
		got := executeRoot(t, args...)
		if got != want {
			t.Errorf("servd %v output = %q, want %q", args, got, want)
		}
	}
}

func TestVersionDoesNotLoadConfig(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	configDir := filepath.Join(configHome, "servd")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte("invalid = ["), 0o644); err != nil {
		t.Fatal(err)
	}

	want := buildinfo.String() + "\n"
	if got := executeRoot(t, "version"); got != want {
		t.Fatalf("servd version output = %q, want %q", got, want)
	}
}

func executeRoot(t *testing.T, args ...string) string {
	t.Helper()
	var output bytes.Buffer
	command := newRootCmd()
	command.SetArgs(args)
	command.SetOut(&output)
	command.SetErr(&output)
	if err := command.Execute(); err != nil {
		t.Fatalf("execute servd %v: %v", args, err)
	}
	return output.String()
}

package commands

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostsfile"
)

func TestPrimaryHostnamesOwnsOnlyPrimaryAndDeduplicates(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Hostnames.TLDsFallback = []string{"test", "100.101.102.103.nip.io", "localhost"}
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "acme"},
		{Slug: "acme"},
		{Slug: "docs", HostPrefix: "auth"},
	}}
	got, err := primaryHostnames(settings, registry)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"acme.localhost", "auth.docs.localhost"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("primaryHostnames() = %v, want %v", got, want)
	}

	path := filepath.Join(t.TempDir(), "hosts")
	unmanaged := "100.101.102.103 acme.100.101.102.103.nip.io\n"
	stale := unmanaged + hostsfile.BuildBlock([]string{"acme.localhost", "acme.test", "auth.docs.test"})
	if err := os.WriteFile(path, []byte(stale), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := hostsfile.SyncAt(path, got); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if managed := hostsfile.ExtractManagedBlock(string(content)); !reflect.DeepEqual(managed, want) {
		t.Errorf("managed hosts after migration = %v, want %v", managed, want)
	}
	if !strings.Contains(string(content), unmanaged) {
		t.Errorf("unmanaged fallback was changed: %s", content)
	}
}

func TestHostsWriteErrorIncludesPrivilegeRemediation(t *testing.T) {
	err := hostsWriteError(fs.ErrPermission)
	want := "could not update " + hostsfile.Path() + ": permission denied"
	if got := err.Error(); !strings.Contains(got, want) || !strings.Contains(got, hostsPermissionInstruction()) {
		t.Fatalf("hostsWriteError() = %q", got)
	}
}

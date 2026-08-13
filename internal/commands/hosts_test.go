package commands

import (
	"io/fs"
	"reflect"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostsfile"
)

func TestPrimaryHostnamesSortsAndDeduplicates(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Hostnames.TLDs = []string{"localhost", "test"}
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "acme"},
		{Slug: "acme"},
		{Slug: "docs", HostPrefix: "auth"},
	}}
	got, err := primaryHostnames(settings, registry)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"acme.localhost", "acme.test", "auth.docs.localhost", "auth.docs.test"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("primaryHostnames() = %v, want %v", got, want)
	}
}

func TestHostsWriteErrorIncludesPrivilegeRemediation(t *testing.T) {
	err := hostsWriteError(fs.ErrPermission)
	want := "could not update " + hostsfile.Path() + ": permission denied"
	if got := err.Error(); !strings.Contains(got, want) || !strings.Contains(got, hostsPermissionInstruction()) {
		t.Fatalf("hostsWriteError() = %q", got)
	}
}

package hostsfile

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestRemoveBlockPreservesUnmanagedEntries(t *testing.T) {
	content := "127.0.0.1 localhost\n\n# servd-start\n127.0.0.1 acme.localhost\n# servd-end\n\n::1 localhost\n"
	want := "127.0.0.1 localhost\n\n::1 localhost\n"
	if got := RemoveBlock(content); got != want {
		t.Fatalf("RemoveBlock() = %q, want %q", got, want)
	}
}

func TestRemoveBlockLeavesIncompleteBlockUntouched(t *testing.T) {
	content := "127.0.0.1 localhost\n# servd-start\n127.0.0.1 acme.localhost\n"
	if got := RemoveBlock(content); got != content {
		t.Fatalf("RemoveBlock() = %q, want incomplete block preserved", got)
	}
}

func TestBuildBlockSortsAndDeduplicates(t *testing.T) {
	hosts := []string{"Acme.Localhost", "auth.acme.localhost", "acme.localhost", ""}
	want := "# servd-start\n127.0.0.1 acme.localhost\n127.0.0.1 auth.acme.localhost\n# servd-end\n"
	if got := BuildBlock(hosts); got != want {
		t.Fatalf("BuildBlock() = %q, want %q", got, want)
	}
}

func TestSyncAtReplacesOnlyManagedBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	initial := "127.0.0.1 localhost\n# servd-start\n127.0.0.1 stale.test\n# servd-end\n::1 localhost\n"
	if err := os.WriteFile(path, []byte(initial), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := SyncAt(path, []string{"auth.acme.test", "acme.test"}); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	want := "127.0.0.1 localhost\n::1 localhost\n\n# servd-start\n127.0.0.1 acme.test\n127.0.0.1 auth.acme.test\n# servd-end\n"
	if got := string(content); got != want {
		t.Fatalf("hosts content = %q, want %q", got, want)
	}
	if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("hosts mode = %v, %v; want 0640", info.Mode(), err)
	}
	managed, err := ManagedHostnamesAt(path)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"acme.test", "auth.acme.test"}; !reflect.DeepEqual(managed, want) {
		t.Fatalf("managed hosts = %v, want %v", managed, want)
	}
}

func TestSyncAtEmptyHostnamesCleansManagedBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hosts")
	if err := os.WriteFile(path, []byte("127.0.0.1 localhost\n# servd-start\n127.0.0.1 acme.test\n# servd-end\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := SyncAt(path, nil); err != nil {
		t.Fatal(err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := string(content), "127.0.0.1 localhost\n"; got != want {
		t.Fatalf("hosts content = %q, want %q", got, want)
	}
}

func TestExtractManagedBlockIgnoresCommentsAndOtherAddresses(t *testing.T) {
	content := "# servd-start\n# comment\n::1 acme.localhost\n127.0.0.1 auth.acme.localhost acme.localhost\n# servd-end\n"
	if got, want := ExtractManagedBlock(content), []string{"acme.localhost", "auth.acme.localhost"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractManagedBlock() = %v, want %v", got, want)
	}
}

func TestNeedsHostsFile(t *testing.T) {
	cases := []struct {
		tlds    []string
		browser string
		want    bool
	}{
		{[]string{"localhost"}, "", false},
		{[]string{"localhost"}, "Safari", true},
		{[]string{"test"}, "", true},
		{[]string{"localhost", "dev.example.com"}, "", true},
	}
	for _, tc := range cases {
		t.Run(strings.Join(tc.tlds, ",")+tc.browser, func(t *testing.T) {
			if got := NeedsHostsFile(tc.tlds, tc.browser); got != tc.want {
				t.Fatalf("NeedsHostsFile(%v, %q) = %v, want %v", tc.tlds, tc.browser, got, tc.want)
			}
		})
	}
}

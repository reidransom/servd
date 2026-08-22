package state

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestEntryAlive(t *testing.T) {
	pid := os.Getpid()
	identity, err := ProcessIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		e    Entry
		want bool
	}{
		{"self with matching identity", Entry{PID: pid, Identity: identity}, true},
		{"self without recorded identity", Entry{PID: pid}, true},
		{"identity mismatch (recycled pid)", Entry{PID: pid, Identity: identity + 1}, false},
		{"zero pid", Entry{PID: 0}, false},
		{"negative pid", Entry{PID: -5}, false},
	}
	for _, c := range cases {
		if got := EntryAlive(c.e); got != c.want {
			t.Errorf("%s: EntryAlive = %v, want %v", c.name, got, c.want)
		}
	}
}

// TestMutateConcurrent exercises the flock around state read-modify-write:
// without it, concurrent mutators drop each other's entries.
func TestMutateConcurrent(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	pid := os.Getpid()
	identity, err := ProcessIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}

	const n = 20
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			slug := fmt.Sprintf("site-%d", i)
			err := Mutate(func(s *State) error {
				s.Entries[slug] = Entry{Slug: slug, PID: pid, Identity: identity, StartedAt: time.Now()}
				return nil
			})
			if err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()

	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Entries) != n {
		t.Fatalf("got %d entries, want %d (lost updates)", len(s.Entries), n)
	}
}

func TestLoadRetainsDeadEntries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	pid := os.Getpid()
	identity, err := ProcessIdentity(pid)
	if err != nil {
		t.Fatal(err)
	}
	err = Mutate(func(s *State) error {
		s.Entries["alive"] = Entry{Slug: "alive", PID: pid, Identity: identity}
		s.Entries["dead"] = Entry{Slug: "dead", PID: 1<<30 - 7}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("alive"); !ok {
		t.Error("alive entry dropped")
	}
	if _, ok := s.Get("dead"); !ok {
		t.Error("dead runtime attempt was removed")
	}
}

func TestLoadLegacyEntryHasEmptyFailureFields(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := os.MkdirAll(filepath.Dir(statePath()), 0o755); err != nil {
		t.Fatal(err)
	}
	data := []byte(`{"entries":{"legacy":{"slug":"legacy","pid":123,"pgid":123,"port":4001,"cmd":"serve","log":"/tmp/site.log","started_at":"2026-01-01T00:00:00Z"}}}`)
	if err := os.WriteFile(statePath(), data, 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := s.Get("legacy")
	if !ok {
		t.Fatal("legacy entry was not loaded")
	}
	if entry.Failure != "" || !entry.FailedAt.IsZero() {
		t.Errorf("legacy failure fields = %q, %s; want empty", entry.Failure, entry.FailedAt)
	}
}

func TestDeleteRemovesFailedEntry(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	if err := Mutate(func(s *State) error {
		s.Entries["failed"] = Entry{Slug: "failed", Failure: "shell start failed", FailedAt: time.Now()}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := Delete("failed"); err != nil {
		t.Fatal(err)
	}
	s, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := s.Get("failed"); ok {
		t.Fatal("failed entry was not deleted")
	}
}

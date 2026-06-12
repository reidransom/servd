package state

import (
	"fmt"
	"os"
	"sync"
	"syscall"
	"testing"
	"time"
)

func TestEntryAlive(t *testing.T) {
	pid := os.Getpid()
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		e    Entry
		want bool
	}{
		{"self with matching pgid", Entry{PID: pid, PGID: pgid}, true},
		{"self without recorded pgid", Entry{PID: pid}, true},
		{"pgid mismatch (recycled pid)", Entry{PID: pid, PGID: pgid + 99999}, false},
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
	pgid, err := syscall.Getpgid(pid)
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
				s.Entries[slug] = Entry{Slug: slug, PID: pid, PGID: pgid, StartedAt: time.Now()}
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

func TestLoadReconcilesDeadEntries(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	pid := os.Getpid()
	pgid, _ := syscall.Getpgid(pid)
	err := Mutate(func(s *State) error {
		s.Entries["alive"] = Entry{Slug: "alive", PID: pid, PGID: pgid}
		s.Entries["dead"] = Entry{Slug: "dead", PID: 1<<30 - 7} // unlikely to exist
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
	if _, ok := s.Get("dead"); ok {
		t.Error("dead entry not reconciled away")
	}
}

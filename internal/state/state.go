// Package state tracks the live runtime of supervised dev servers.
//
// state.json maps a site slug to the running process's pid, port group,
// logfile and resolved command. It is the source of truth for "what is
// running right now", independent of the registry (which is "what is known").
//
// Reads (Load) are lock-free: the file is written atomically, so a read is
// always consistent, and dead entries are dropped in memory. Writes go
// through Mutate, which holds an exclusive file lock around the whole
// load-modify-save cycle so concurrent servd processes (CLI, TUI, proxy
// control) can't lose each other's updates.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/flock"
)

// Entry is the runtime record for one running process.
type Entry struct {
	Slug          string    `json:"slug"`
	PID           int       `json:"pid"`
	PGID          int       `json:"pgid"`
	Port          int       `json:"port"`
	Cmd           string    `json:"cmd"`
	Log           string    `json:"log"`
	StartedAt     time.Time `json:"started_at"`
	PublishedMDNS []string  `json:"published_mdns,omitempty"`
}

// State is the whole state.json document.
type State struct {
	Entries map[string]Entry `json:"entries"`
}

func statePath() string { return filepath.Join(config.StateDir(), "state.json") }

// Load reads and reconciles state.json. Missing file yields empty state.
// Entries whose process is no longer alive are dropped in memory only; the
// next Mutate persists the healed view.
func Load() (*State, error) {
	s := &State{Entries: map[string]Entry{}}
	data, err := os.ReadFile(statePath())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}
	if err := json.Unmarshal(data, s); err != nil {
		return s, err
	}
	if s.Entries == nil {
		s.Entries = map[string]Entry{}
	}
	for slug, e := range s.Entries {
		if !EntryAlive(e) {
			delete(s.Entries, slug)
		}
	}
	return s, nil
}

// Mutate loads the state, applies fn and saves the result, all under an
// exclusive file lock. All writers must go through here.
func Mutate(fn func(*State) error) error {
	return flock.WithLock(statePath(), func() error {
		s, err := Load()
		if err != nil {
			return err
		}
		if err := fn(s); err != nil {
			return err
		}
		return s.save()
	})
}

// Delete removes an entry under the file lock.
func Delete(slug string) error {
	return Mutate(func(s *State) error {
		delete(s.Entries, slug)
		return nil
	})
}

// save writes state.json atomically. Callers must hold the file lock.
func (s *State) save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return config.WriteAtomic(statePath(), data)
}

// Get returns the entry for slug and whether it exists.
func (s *State) Get(slug string) (Entry, bool) {
	e, ok := s.Entries[slug]
	return e, ok
}

// EntryAlive reports whether e's process is alive and still plausibly ours:
// the pid exists AND its current process group matches the recorded one. This
// guards against pid reuse (reboot, pid-counter wraparound) — a recycled pid
// is very unlikely to also lead the same process group. A live pid whose
// Getpgid fails is treated as dead (it vanished mid-check).
func EntryAlive(e Entry) bool {
	if !ProcessAlive(e.PID) {
		return false
	}
	if e.PGID > 0 {
		pgid, err := syscall.Getpgid(e.PID)
		if err != nil || pgid != e.PGID {
			return false
		}
	}
	return true
}

// ProcessAlive reports whether a process with the given pid exists.
func ProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	// Signal 0 performs error checking without actually sending a signal.
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	// EPERM means the process exists but we can't signal it (still alive).
	return errors.Is(err, syscall.EPERM)
}

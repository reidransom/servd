// Package state tracks the latest supervised runtime attempt for dev servers.
//
// state.json maps a site slug to its latest supervised runtime attempt,
// including the process details, logfile, resolved command, and any recorded
// launch failure. It is independent of the registry (which is "what is
// known").
//
// Reads (Load) are lock-free: the file is written atomically. Writes go
// through Mutate, which holds an exclusive file lock around the whole
// load-modify-save cycle so concurrent servd processes (CLI, TUI, proxy
// control) can't lose each other's updates.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/flock"
)

// Entry is the runtime record for one supervised attempt.
type Entry struct {
	Slug          string    `json:"slug"`
	PID           int       `json:"pid"`
	PGID          int       `json:"pgid"`
	Identity      uint64    `json:"identity,omitempty"`
	Port          int       `json:"port"`
	Cmd           string    `json:"cmd"`
	Log           string    `json:"log"`
	StartedAt     time.Time `json:"started_at"`
	Failure       string    `json:"failure,omitempty"`
	FailedAt      time.Time `json:"failed_at,omitempty"`
	PublishedMDNS []string  `json:"published_mdns,omitempty"`
}

// State is the whole state.json document.
type State struct {
	Entries map[string]Entry `json:"entries"`
}

func statePath() string { return filepath.Join(config.StateDir(), "state.json") }

// Load reads state.json. Missing file yields empty state.
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

// EntryAlive reports whether e's process is alive and still plausibly ours.
// New entries carry an OS-specific identity to guard against PID reuse. PGID
// remains as a compatibility check for state written by older Unix releases.
func EntryAlive(e Entry) bool {
	if !ProcessAlive(e.PID) {
		return false
	}
	if e.Identity > 0 {
		identity, err := ProcessIdentity(e.PID)
		return err == nil && identity == e.Identity
	}
	return legacyEntryAlive(e)
}

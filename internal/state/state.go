// Package state tracks the live runtime of supervised dev servers.
//
// state.json maps a site slug to the running process's pid, port group,
// logfile and resolved command. It is the source of truth for "what is
// running right now", independent of the registry (which is "what is known").
//
// On every load the state is reconciled: entries whose process is no longer
// alive are dropped, so a stale file left by a crash heals itself.
package state

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/reidransom/servd/internal/config"
)

// Entry is the runtime record for one running process.
type Entry struct {
	Slug      string    `json:"slug"`
	PID       int       `json:"pid"`
	PGID      int       `json:"pgid"`
	Port      int       `json:"port"`
	Cmd       string    `json:"cmd"`
	Log       string    `json:"log"`
	StartedAt time.Time `json:"started_at"`
}

// State is the whole state.json document.
type State struct {
	Entries map[string]Entry `json:"entries"`
}

func statePath() string { return filepath.Join(config.StateDir(), "state.json") }

// Load reads and reconciles state.json. Missing file yields empty state.
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
	// Reconcile: drop dead processes.
	changed := false
	for slug, e := range s.Entries {
		if !ProcessAlive(e.PID) {
			delete(s.Entries, slug)
			changed = true
		}
	}
	if changed {
		_ = s.Save()
	}
	return s, nil
}

// Save writes state.json atomically.
func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	path := statePath()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// Get returns the entry for slug and whether it exists.
func (s *State) Get(slug string) (Entry, bool) {
	e, ok := s.Entries[slug]
	return e, ok
}

// Set records an entry and persists.
func (s *State) Set(e Entry) error {
	s.Entries[e.Slug] = e
	return s.Save()
}

// Delete removes an entry and persists.
func (s *State) Delete(slug string) error {
	delete(s.Entries, slug)
	return s.Save()
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

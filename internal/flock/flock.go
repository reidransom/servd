// Package flock provides advisory file locking for servd's read-modify-write
// cycles on its persistent files (state.json, sites.toml).
package flock

import (
	"os"
	"path/filepath"
)

// WithLock holds an exclusive flock on path+".lock" while fn runs. The lock
// file is created (and its parent directories) if missing, and is left in
// place afterwards — only the lock itself is released.
func WithLock(path string, fn func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if err := lockFile(f); err != nil {
		return err
	}
	defer func() { _ = unlockFile(f) }()
	return fn()
}

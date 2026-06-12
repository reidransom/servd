package launcher

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// procfileNames are checked in order; the first that exists wins.
var procfileNames = []string{"Procfile.dev", "Procfile"}

// procEntry is one "name: command" line from a Procfile.
type procEntry struct {
	Name string
	Cmd  string
}

// readProcfile returns the parsed entries of the first Procfile found in dir,
// or nil if none exists.
func readProcfile(dir string) []procEntry {
	for _, name := range procfileNames {
		f, err := os.Open(filepath.Join(dir, name))
		if err != nil {
			continue
		}
		entries := parseProcfile(f)
		f.Close()
		return entries
	}
	return nil
}

// parseProcfile parses "name: command" lines, ignoring blanks and # comments.
func parseProcfile(r io.Reader) []procEntry {
	var entries []procEntry
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		colon := strings.IndexByte(line, ':')
		if colon < 0 {
			continue
		}
		name := strings.TrimSpace(line[:colon])
		cmd := strings.TrimSpace(line[colon+1:])
		if name == "" || cmd == "" {
			continue
		}
		entries = append(entries, procEntry{Name: name, Cmd: cmd})
	}
	return entries
}

// webProcess returns the "web" entry, or the first entry if no "web" exists.
func webProcess(entries []procEntry) (procEntry, bool) {
	if len(entries) == 0 {
		return procEntry{}, false
	}
	for _, e := range entries {
		if e.Name == "web" {
			return e, true
		}
	}
	return entries[0], true
}

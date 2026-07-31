// Package hostsfile manages the servd-owned block in the system hosts file.
package hostsfile

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const (
	MarkerStart = "# servd-start"
	MarkerEnd   = "# servd-end"
)

// ResolutionResult records the addresses resolved for one hostname and whether
// at least one of them is a loopback address.
type ResolutionResult struct {
	Hostname  string
	Addresses []net.IPAddr
	Loopback  bool
}

// Path returns the platform's system hosts file path.
func Path() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

// ExtractManagedBlock returns hostnames in the first complete servd-managed
// block. Malformed or incomplete blocks are deliberately ignored.
func ExtractManagedBlock(content string) []string {
	lines := strings.Split(normalizeNewlines(content), "\n")
	start, end := managedBlockRange(lines)
	if start < 0 || end < 0 {
		return nil
	}
	var hosts []string
	for _, line := range lines[start+1 : end] {
		fields := strings.Fields(line)
		if len(fields) >= 2 && fields[0] == "127.0.0.1" {
			hosts = append(hosts, fields[1:]...)
		}
	}
	return normalizedHostnames(hosts)
}

// RemoveBlock removes complete servd-managed blocks while preserving all other
// hosts-file entries. Incomplete blocks are left untouched for safety.
func RemoveBlock(content string) string {
	lines := strings.Split(normalizeNewlines(content), "\n")
	out := make([]string, 0, len(lines))
	for index := 0; index < len(lines); {
		if strings.TrimSpace(lines[index]) != MarkerStart {
			out = append(out, lines[index])
			index++
			continue
		}
		end := index + 1
		for end < len(lines) && strings.TrimSpace(lines[end]) != MarkerEnd {
			end++
		}
		if end == len(lines) {
			out = append(out, lines[index])
			index++
			continue
		}
		index = end + 1
	}
	return normalizeBlankLines(out)
}

// BuildBlock renders a sorted, de-duplicated servd hosts block. An empty input
// produces no block, so syncing an empty registry removes stale entries.
func BuildBlock(hostnames []string) string {
	hosts := normalizedHostnames(hostnames)
	if len(hosts) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(MarkerStart)
	b.WriteByte('\n')
	for _, hostname := range hosts {
		b.WriteString("127.0.0.1 ")
		b.WriteString(hostname)
		b.WriteByte('\n')
	}
	b.WriteString(MarkerEnd)
	b.WriteByte('\n')
	return b.String()
}

// ManagedHostnames returns the hostname entries in the system hosts file's
// servd-managed block.
func ManagedHostnames() ([]string, error) { return ManagedHostnamesAt(Path()) }

// ManagedHostnamesAt is ManagedHostnames for an explicit path. It supports
// callers that need to inspect a sandboxed hosts file.
func ManagedHostnamesAt(path string) ([]string, error) {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return ExtractManagedBlock(string(content)), nil
}

// Sync replaces only the servd-managed block in the system hosts file.
func Sync(hostnames []string) error { return SyncAt(Path(), hostnames) }

// SyncAt is Sync for an explicit path. It is useful when a caller needs an
// isolated hosts file, such as a test or a sandboxed installation.
func SyncAt(path string, hostnames []string) error {
	content, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	updated := RemoveBlock(string(content))
	if block := BuildBlock(hostnames); block != "" {
		if updated != "" {
			updated += "\n"
		}
		updated += block
	}
	return writeAtomic(path, []byte(updated))
}

// Clean removes only the servd-managed block from the system hosts file.
func Clean() error { return CleanAt(Path()) }

// CleanAt is Clean for an explicit path.
func CleanAt(path string) error {
	content, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return writeAtomic(path, []byte(RemoveBlock(string(content))))
}

// CheckResolution looks up hostname and reports whether it resolves to a
// loopback address.
func CheckResolution(hostname string) (ResolutionResult, error) {
	result := ResolutionResult{Hostname: hostname}
	addresses, err := net.DefaultResolver.LookupIPAddr(context.Background(), hostname)
	if err != nil {
		return result, err
	}
	result.Addresses = addresses
	for _, address := range addresses {
		if address.IP.IsLoopback() {
			result.Loopback = true
			break
		}
	}
	return result, nil
}

// NeedsHostsFile reports whether the configured TLDs require managed hosts
// entries for normal browser resolution. Safari is included because it can fail
// to resolve .localhost subdomains even though that TLD is normally loopback.
func NeedsHostsFile(tlds []string, browser string) bool {
	if strings.EqualFold(strings.TrimSpace(browser), "safari") {
		return true
	}
	for _, tld := range tlds {
		if !strings.EqualFold(strings.Trim(strings.TrimSpace(tld), "."), "localhost") {
			return true
		}
	}
	return false
}

func managedBlockRange(lines []string) (int, int) {
	for start, line := range lines {
		if strings.TrimSpace(line) != MarkerStart {
			continue
		}
		for end := start + 1; end < len(lines); end++ {
			if strings.TrimSpace(lines[end]) == MarkerEnd {
				return start, end
			}
		}
		return -1, -1
	}
	return -1, -1
}

func normalizedHostnames(hostnames []string) []string {
	seen := make(map[string]struct{}, len(hostnames))
	for _, hostname := range hostnames {
		hostname = strings.ToLower(strings.TrimSpace(hostname))
		if hostname != "" {
			seen[hostname] = struct{}{}
		}
	}
	out := make([]string, 0, len(seen))
	for hostname := range seen {
		out = append(out, hostname)
	}
	sort.Strings(out)
	return out
}

func normalizeNewlines(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func normalizeBlankLines(lines []string) string {
	for len(lines) > 0 && strings.TrimSpace(lines[0]) == "" {
		lines = lines[1:]
	}
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	out := make([]string, 0, len(lines))
	previousBlank := false
	for _, line := range lines {
		blank := strings.TrimSpace(line) == ""
		if blank && previousBlank {
			continue
		}
		out = append(out, line)
		previousBlank = blank
	}
	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

func writeAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	mode := os.FileMode(0o644)
	if info != nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".servd-hosts-*")
	if err != nil {
		return os.WriteFile(path, data, mode)
	}
	tmpPath := tmp.Name()
	defer func() { _ = os.Remove(tmpPath) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return os.WriteFile(path, data, mode)
	}
	return nil
}

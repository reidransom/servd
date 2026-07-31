// Package hostnames composes and validates the DNS names served by servd.
package hostnames

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// MaxDNSLabelLength is the RFC 1035 maximum length of one DNS label.
const MaxDNSLabelLength = 63

const maxHostnameLength = 253

var labelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// InferredName records a project name and the source from which it was inferred.
type InferredName struct {
	Name   string
	Source string
}

// WorktreePrefix records the branch-derived prefix for a linked Git worktree.
type WorktreePrefix struct {
	Prefix string
	Source string
}

// SanitizeLabel converts a name into one valid DNS label. It lowercases,
// replaces invalid characters with hyphens, collapses repeated hyphens, and
// truncates labels that exceed the DNS limit.
func SanitizeLabel(name string) string {
	var b strings.Builder
	lastHyphen := true
	for _, r := range strings.ToLower(name) {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if valid {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	return TruncateLabel(strings.TrimSuffix(b.String(), "-"))
}

// TruncateLabel keeps a DNS label within MaxDNSLabelLength. Truncated labels
// end with a deterministic six-character SHA-256 suffix to retain uniqueness.
func TruncateLabel(label string) string {
	if len(label) <= MaxDNSLabelLength {
		return label
	}

	digest := sha256.Sum256([]byte(label))
	hash := hex.EncodeToString(digest[:])[:6]
	prefix := strings.TrimRight(label[:MaxDNSLabelLength-7], "-")
	return prefix + "-" + hash
}

// InferProjectName finds a usable project name from the nearest package.json,
// then the Git repository root, then the current directory name.
func InferProjectName(cwd string) (InferredName, error) {
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return InferredName{}, fmt.Errorf("resolving current directory: %w", err)
	}

	if name := findPackageJSONName(cwd); name != "" {
		if name = SanitizeLabel(name); name != "" {
			return InferredName{Name: name, Source: "package.json"}, nil
		}
	}
	if root := findGitRoot(cwd); root != "" {
		if name := SanitizeLabel(filepath.Base(root)); name != "" {
			return InferredName{Name: name, Source: "git root"}, nil
		}
	}
	if name := SanitizeLabel(filepath.Base(cwd)); name != "" {
		return InferredName{Name: name, Source: "directory name"}, nil
	}
	return InferredName{}, errors.New("could not infer a project name from package.json, git root, or directory name")
}

func findPackageJSONName(startDir string) string {
	for dir := startDir; ; dir = filepath.Dir(dir) {
		data, err := os.ReadFile(filepath.Join(dir, "package.json"))
		if err == nil {
			var pkg struct {
				Name string `json:"name"`
			}
			if json.Unmarshal(data, &pkg) == nil && pkg.Name != "" {
				return strings.TrimPrefix(pkg.Name, scopedPackagePrefix(pkg.Name))
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

func scopedPackagePrefix(name string) string {
	if !strings.HasPrefix(name, "@") {
		return ""
	}
	if slash := strings.IndexByte(name, '/'); slash > 1 {
		return name[:slash+1]
	}
	return ""
}

func findGitRoot(cwd string) string {
	if output, err := gitOutput(cwd, "rev-parse", "--show-toplevel"); err == nil && output != "" {
		return output
	}
	for dir := cwd; ; dir = filepath.Dir(dir) {
		info, err := os.Stat(filepath.Join(dir, ".git"))
		if err == nil && (info.IsDir() || info.Mode().IsRegular()) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
	}
}

// DetectWorktreePrefix returns a prefix for a linked worktree's non-default
// branch. The main checkout is never prefixed, even when it is on a feature
// branch. Default branches main and master are not prefixed.
func DetectWorktreePrefix(cwd string) (WorktreePrefix, error) {
	cwd, err := filepath.Abs(cwd)
	if err != nil {
		return WorktreePrefix{}, fmt.Errorf("resolving current directory: %w", err)
	}
	if prefix, available := detectWorktreeViaGit(cwd); available {
		return prefix, nil
	}
	return detectWorktreeViaFilesystem(cwd), nil
}

// ApplyWorktreePrefix prepends prefix as a DNS label when it is non-empty.
func ApplyWorktreePrefix(base, prefix string) string {
	if prefix == "" {
		return base
	}
	return prefix + "." + base
}

func detectWorktreeViaGit(cwd string) (WorktreePrefix, bool) {
	list, err := gitOutput(cwd, "worktree", "list", "--porcelain")
	if err != nil {
		return WorktreePrefix{}, false
	}
	if strings.Count("\n"+list, "\nworktree ") <= 1 {
		return WorktreePrefix{}, true
	}
	gitDir, err := gitOutput(cwd, "rev-parse", "--git-dir")
	if err != nil {
		return WorktreePrefix{}, false
	}
	commonDir, err := gitOutput(cwd, "rev-parse", "--git-common-dir")
	if err != nil {
		return WorktreePrefix{}, false
	}
	if resolveGitPath(cwd, gitDir) == resolveGitPath(cwd, commonDir) {
		return WorktreePrefix{}, true
	}
	branch, err := gitOutput(cwd, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return WorktreePrefix{}, false
	}
	return branchPrefix(branch), true
}

func resolveGitPath(cwd, path string) string {
	if filepath.IsAbs(path) {
		return filepath.Clean(path)
	}
	return filepath.Clean(filepath.Join(cwd, path))
}

func detectWorktreeViaFilesystem(startDir string) WorktreePrefix {
	for dir := startDir; ; dir = filepath.Dir(dir) {
		gitPath := filepath.Join(dir, ".git")
		info, err := os.Stat(gitPath)
		if err == nil {
			if info.IsDir() {
				return WorktreePrefix{}
			}
			if info.Mode().IsRegular() {
				data, readErr := os.ReadFile(gitPath)
				if readErr != nil {
					return WorktreePrefix{}
				}
				gitDir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
				if gitDir == "" || gitDir == strings.TrimSpace(string(data)) || !isWorktreeGitDir(gitDir) {
					return WorktreePrefix{}
				}
				if !filepath.IsAbs(gitDir) {
					gitDir = filepath.Join(dir, gitDir)
				}
				branch := readBranch(filepath.Clean(gitDir))
				return branchPrefix(branch)
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return WorktreePrefix{}
		}
	}
}

func isWorktreeGitDir(gitDir string) bool {
	parts := strings.Split(filepath.ToSlash(filepath.Clean(gitDir)), "/")
	for i := 0; i+1 < len(parts); i++ {
		if parts[i] == "worktrees" && parts[i+1] != "" {
			return i+2 == len(parts)
		}
	}
	return false
}

func readBranch(gitDir string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return ""
	}
	const prefix = "ref: refs/heads/"
	head := strings.TrimSpace(string(data))
	if !strings.HasPrefix(head, prefix) {
		return ""
	}
	return strings.TrimPrefix(head, prefix)
}

func branchPrefix(branch string) WorktreePrefix {
	if branch == "" || branch == "HEAD" || branch == "main" || branch == "master" {
		return WorktreePrefix{}
	}
	if slash := strings.LastIndexByte(branch, '/'); slash >= 0 {
		branch = branch[slash+1:]
	}
	if prefix := SanitizeLabel(branch); prefix != "" {
		return WorktreePrefix{Prefix: prefix, Source: "git branch"}
	}
	return WorktreePrefix{}
}

func gitOutput(cwd string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = cwd
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// ValidateLabel verifies one DNS label.
func ValidateLabel(label string) error {
	if label == "" {
		return errors.New("DNS label cannot be empty")
	}
	if len(label) > MaxDNSLabelLength {
		return fmt.Errorf("invalid DNS label %q: exceeds 63-character DNS limit", label)
	}
	if !labelPattern.MatchString(label) {
		return fmt.Errorf("invalid DNS label %q: must contain only lowercase letters, digits, and interior hyphens", label)
	}
	return nil
}

// ValidateTLD verifies an explicitly configured DNS suffix.
func ValidateTLD(tld string) error {
	if tld == "" {
		return errors.New("TLD cannot be empty")
	}
	if len(tld) > maxHostnameLength {
		return fmt.Errorf("invalid TLD %q: exceeds 253-character DNS limit", tld)
	}
	for _, label := range strings.Split(tld, ".") {
		if label == "" {
			return fmt.Errorf("invalid TLD %q: labels cannot be empty", tld)
		}
		if len(label) > MaxDNSLabelLength {
			return fmt.Errorf("invalid TLD %q: label %q exceeds 63-character DNS limit", tld, label)
		}
		if !labelPattern.MatchString(label) {
			return fmt.Errorf("invalid TLD %q: labels must contain only lowercase letters, digits, and interior hyphens", tld)
		}
	}
	return nil
}

// FormatURL returns an HTTP or HTTPS URL and omits the standard protocol port.
func FormatURL(hostname string, port int, tls bool) string {
	protocol, defaultPort := "http", 80
	if tls {
		protocol, defaultPort = "https", 443
	}
	if port == defaultPort {
		return protocol + "://" + hostname
	}
	return fmt.Sprintf("%s://%s:%d", protocol, hostname, port)
}

// ParseHostname normalizes input as a subdomain of tld and validates the full
// resulting hostname.
func ParseHostname(input, tld string) (string, error) {
	if err := ValidateTLD(tld); err != nil {
		return "", err
	}
	hostname := normalizeInput(input)
	suffix := "." + tld
	if tld != "localhost" && strings.HasSuffix(hostname, ".localhost") {
		hostname = strings.TrimSuffix(hostname, ".localhost")
	}
	if hostname == "" || hostname == suffix {
		return "", errors.New("hostname cannot be empty")
	}
	if !strings.HasSuffix(hostname, suffix) {
		hostname += suffix
	}
	name := strings.TrimSuffix(hostname, suffix)
	if err := validateHostnameBase(name); err != nil {
		return "", err
	}
	if len(hostname) > maxHostnameLength {
		return "", fmt.Errorf("invalid hostname %q: exceeds 253-character DNS limit", hostname)
	}
	return hostname, nil
}

// ParseHostnames normalizes input for each configured TLD. If input already
// ends with a configured TLD, the longest matching suffix is stripped first.
// Invalid TLD-specific results are skipped while valid results are retained.
func ParseHostnames(input string, tlds []string) ([]string, error) {
	uniqueTLDs := uniqueStrings(tlds)
	if len(uniqueTLDs) == 0 {
		return nil, errors.New("at least one TLD is required")
	}
	base := normalizeInput(input)
	sortedTLDs := append([]string(nil), uniqueTLDs...)
	sort.SliceStable(sortedTLDs, func(i, j int) bool { return len(sortedTLDs[i]) > len(sortedTLDs[j]) })
	for _, tld := range sortedTLDs {
		if strings.HasSuffix(base, "."+tld) {
			base = strings.TrimSuffix(base, "."+tld)
			break
		}
	}

	var hostnames []string
	var firstErr error
	for _, tld := range uniqueTLDs {
		hostname, err := ParseHostname(base, tld)
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		hostnames = append(hostnames, hostname)
	}
	if len(hostnames) == 0 {
		return nil, firstErr
	}
	return hostnames, nil
}

func normalizeInput(input string) string {
	hostname := strings.ToLower(strings.TrimSpace(input))
	hostname = strings.TrimPrefix(hostname, "http://")
	hostname = strings.TrimPrefix(hostname, "https://")
	if slash := strings.IndexByte(hostname, '/'); slash >= 0 {
		hostname = hostname[:slash]
	}
	return hostname
}

func validateHostnameBase(name string) error {
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid hostname %q: consecutive dots are not allowed", name)
	}
	if !labelPattern.MatchString(name) && !validDottedName(name) {
		return fmt.Errorf("invalid hostname %q: must contain only lowercase letters, digits, hyphens, and dots", name)
	}
	for _, label := range strings.Split(name, ".") {
		if len(label) > MaxDNSLabelLength {
			return fmt.Errorf("invalid hostname %q: label %q exceeds 63-character DNS limit", name, label)
		}
	}
	return nil
}

func validDottedName(name string) bool {
	for _, label := range strings.Split(name, ".") {
		if !labelPattern.MatchString(label) {
			return false
		}
	}
	return true
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	unique := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		unique = append(unique, value)
	}
	return unique
}

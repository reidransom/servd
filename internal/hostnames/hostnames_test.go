package hostnames

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSanitizeLabel(t *testing.T) {
	tests := map[string]string{
		"MyApp":             "myapp",
		"my_app":            "my-app",
		"a___b":             "a-b",
		"--My App--":        "my-app",
		"@@@":               "",
		"my-app-123":        "my-app-123",
		"My_Feature_Branch": "my-feature-branch",
	}
	for input, want := range tests {
		t.Run(input, func(t *testing.T) {
			if got := SanitizeLabel(input); got != want {
				t.Fatalf("SanitizeLabel(%q) = %q, want %q", input, got, want)
			}
		})
	}
}

func TestTruncateLabel(t *testing.T) {
	if got := TruncateLabel(strings.Repeat("a", MaxDNSLabelLength)); got != strings.Repeat("a", MaxDNSLabelLength) {
		t.Fatalf("label at limit changed: %q", got)
	}
	long := strings.Repeat("a", 80)
	got := TruncateLabel(long)
	if len(got) != MaxDNSLabelLength {
		t.Fatalf("truncated label length = %d, want %d", len(got), MaxDNSLabelLength)
	}
	if !strings.HasSuffix(got, "-0f45e8") {
		t.Fatalf("truncated label = %q, want deterministic SHA-256 suffix", got)
	}
	if got == TruncateLabel(strings.Repeat("b", 80)) {
		t.Fatal("different long labels must not share a truncated value")
	}
	if strings.Contains(TruncateLabel(strings.Repeat("a", 55)+"-"+strings.Repeat("b", 20)), "--") {
		t.Fatal("truncated label contains a duplicate separator")
	}
}

func TestFormatURL(t *testing.T) {
	tests := []struct {
		host string
		port int
		tls  bool
		want string
	}{
		{"app.localhost", 80, false, "http://app.localhost"},
		{"app.localhost", 8080, false, "http://app.localhost:8080"},
		{"app.localhost", 443, true, "https://app.localhost"},
		{"app.localhost", 8443, true, "https://app.localhost:8443"},
	}
	for _, test := range tests {
		if got := FormatURL(test.host, test.port, test.tls); got != test.want {
			t.Errorf("FormatURL(%q, %d, %t) = %q, want %q", test.host, test.port, test.tls, got, test.want)
		}
	}
}

func TestParseHostname(t *testing.T) {
	tests := []struct {
		input string
		tld   string
		want  string
	}{
		{"myapp", "localhost", "myapp.localhost"},
		{"https://api.myapp.localhost/path", "localhost", "api.myapp.localhost"},
		{"MyApp", "localhost", "myapp.localhost"},
		{"api.myapp", "test", "api.myapp.test"},
		{"api.myapp.localhost", "test", "api.myapp.test"},
		{"myapp", "local.example.dev", "myapp.local.example.dev"},
	}
	for _, test := range tests {
		got, err := ParseHostname(test.input, test.tld)
		if err != nil {
			t.Fatalf("ParseHostname(%q, %q): %v", test.input, test.tld, err)
		}
		if got != test.want {
			t.Errorf("ParseHostname(%q, %q) = %q, want %q", test.input, test.tld, got, test.want)
		}
	}
}

func TestParseHostnameRejectsInvalidNames(t *testing.T) {
	for _, input := range []string{"", "   ", "my app", "-myapp", "myapp-", "my..app", strings.Repeat("a", 64)} {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseHostname(input, "localhost"); err == nil {
				t.Fatalf("ParseHostname(%q) succeeded", input)
			}
		})
	}
	if _, err := ParseHostname("app", "Bad"); err == nil {
		t.Fatal("uppercase TLD succeeded")
	}
}

func TestParseHostnames(t *testing.T) {
	hosts, err := ParseHostnames("app.dev.example.com", []string{"example.com", "dev.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"app.example.com", "app.dev.example.com"}
	if strings.Join(hosts, ",") != strings.Join(want, ",") {
		t.Fatalf("ParseHostnames() = %v, want %v", hosts, want)
	}

	hosts, err = ParseHostnames("app", []string{"test", "test"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0] != "app.test" {
		t.Fatalf("ParseHostnames deduplication = %v", hosts)
	}

	label := strings.Repeat("a", 62)
	longTLD := strings.Join([]string{label, label, label, label}, ".")
	hosts, err = ParseHostnames("app", []string{"localhost", longTLD})
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0] != "app.localhost" {
		t.Fatalf("TLD-specific overflow should be skipped: %v", hosts)
	}
}

func TestInferProjectName(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "src", "components")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"name":"@org/My_App"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	inferred, err := InferProjectName(nested)
	if err != nil {
		t.Fatal(err)
	}
	if inferred != (InferredName{Name: "my-app", Source: "package.json"}) {
		t.Fatalf("inferred = %#v", inferred)
	}

	gitRoot := filepath.Join(t.TempDir(), "git-project")
	if err := os.MkdirAll(filepath.Join(gitRoot, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	inferred, err = InferProjectName(gitRoot)
	if err != nil {
		t.Fatal(err)
	}
	if inferred != (InferredName{Name: "git-project", Source: "git root"}) {
		t.Fatalf("git inferred = %#v", inferred)
	}

	dir := filepath.Join(t.TempDir(), "directory-project")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	inferred, err = InferProjectName(dir)
	if err != nil {
		t.Fatal(err)
	}
	if inferred != (InferredName{Name: "directory-project", Source: "directory name"}) {
		t.Fatalf("directory inferred = %#v", inferred)
	}
}

func TestDetectWorktreePrefix(t *testing.T) {
	mainCheckout := t.TempDir()
	if err := os.Mkdir(filepath.Join(mainCheckout, ".git"), 0o755); err != nil {
		t.Fatal(err)
	}
	prefix, err := DetectWorktreePrefix(mainCheckout)
	if err != nil {
		t.Fatal(err)
	}
	if prefix != (WorktreePrefix{}) {
		t.Fatalf("main checkout prefix = %#v", prefix)
	}

	worktree := t.TempDir()
	gitDir := filepath.Join(t.TempDir(), "bare.git", "worktrees", "feature-auth")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/feature/My_Branch\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(worktree, ".git"), []byte("gitdir: "+gitDir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	prefix, err = DetectWorktreePrefix(worktree)
	if err != nil {
		t.Fatal(err)
	}
	if prefix != (WorktreePrefix{Prefix: "my-branch", Source: "git branch"}) {
		t.Fatalf("worktree prefix = %#v", prefix)
	}

	if got := ApplyWorktreePrefix("api", prefix.Prefix); got != "my-branch.api" {
		t.Fatalf("ApplyWorktreePrefix() = %q", got)
	}
}

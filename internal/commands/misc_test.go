package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDotHidingFS(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"index.html":  "<h1>hi</h1>",
		".env":        "SECRET=1",
		".git/config": "[core]",
		"sub/ok.txt":  "ok",
		"sub/.secret": "shh",
	}
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	srv := httptest.NewServer(http.FileServer(dotHidingFS{http.Dir(dir)}))
	defer srv.Close()

	get := func(path string) (int, string) {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return resp.StatusCode, string(body)
	}

	if code, body := get("/"); code != http.StatusOK || !strings.Contains(body, "hi") {
		t.Errorf("GET / = %d %q, want index.html", code, body)
	}
	for _, p := range []string{"/.env", "/.git/config", "/sub/.secret"} {
		if code, _ := get(p); code != http.StatusForbidden {
			t.Errorf("GET %s = %d, want 403", p, code)
		}
	}
	if code, body := get("/sub/ok.txt"); code != http.StatusOK || body != "ok" {
		t.Errorf("GET /sub/ok.txt = %d %q", code, body)
	}
	// Directory listing must not reveal dotfiles.
	if code, body := get("/sub/"); code != http.StatusOK || strings.Contains(body, ".secret") || !strings.Contains(body, "ok.txt") {
		t.Errorf("GET /sub/ = %d %q, want listing without .secret", code, body)
	}
}

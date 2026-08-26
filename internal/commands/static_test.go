package commands

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestStaticServerConfigPrecedence(t *testing.T) {
	newConfig := func(t *testing.T) (*cobra.Command, string) {
		t.Helper()
		dir := t.TempDir()
		command := newStaticCmd()
		if err := command.Flags().Set("dir", dir); err != nil {
			t.Fatal(err)
		}
		return command, dir
	}

	t.Run("defaults", func(t *testing.T) {
		unsetStaticEnv(t, "HOST")
		unsetStaticEnv(t, "PORT")
		command, _ := newConfig(t)
		config, err := staticServerConfig(command)
		if err != nil {
			t.Fatal(err)
		}
		if config.addr != "127.0.0.1:8080" {
			t.Fatalf("default address = %q, want 127.0.0.1:8080", config.addr)
		}
	})

	t.Run("environment", func(t *testing.T) {
		t.Setenv("HOST", "127.0.0.2")
		t.Setenv("PORT", "4321")
		command, _ := newConfig(t)
		config, err := staticServerConfig(command)
		if err != nil {
			t.Fatal(err)
		}
		if config.addr != "127.0.0.2:4321" {
			t.Fatalf("environment address = %q, want 127.0.0.2:4321", config.addr)
		}
	})

	t.Run("flags", func(t *testing.T) {
		t.Setenv("HOST", "127.0.0.2")
		t.Setenv("PORT", "4321")
		command, _ := newConfig(t)
		if err := command.Flags().Set("host", "127.0.0.3"); err != nil {
			t.Fatal(err)
		}
		if err := command.Flags().Set("port", "5432"); err != nil {
			t.Fatal(err)
		}
		config, err := staticServerConfig(command)
		if err != nil {
			t.Fatal(err)
		}
		if config.addr != "127.0.0.3:5432" {
			t.Fatalf("flag address = %q, want 127.0.0.3:5432", config.addr)
		}
	})
}

func TestStaticCommandRejectsInvalidInputBeforeListening(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(t *testing.T, command *cobra.Command)
	}{
		{
			name: "positional argument",
			setup: func(t *testing.T, command *cobra.Command) {
				command.SetArgs([]string{"unexpected"})
			},
		},
		{
			name: "empty host",
			setup: func(t *testing.T, command *cobra.Command) {
				if err := command.Flags().Set("host", ""); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "invalid port",
			setup: func(t *testing.T, command *cobra.Command) {
				if err := command.Flags().Set("port", "0"); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "missing root",
			setup: func(t *testing.T, command *cobra.Command) {
				if err := command.Flags().Set("dir", filepath.Join(t.TempDir(), "missing")); err != nil {
					t.Fatal(err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			command := newStaticCmd()
			tc.setup(t, command)
			if err := command.Execute(); err == nil {
				t.Fatal("Execute() succeeded, want validation error")
			}
		})
	}
}

func TestStaticHandlerServesContainedFilesOnly(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"index.html":          "root index",
		"plain.txt":           "plain file",
		"nested/index.html":   "nested index",
		"without-index/file":  "no listing",
		".hidden.txt":         "hidden",
		".well-known/challenge": "hidden well-known",
	}
	for name, content := range files {
		filename := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(filename), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filename, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	canonicalRoot, err := canonicalStaticRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(staticHandler{root: canonicalRoot})
	defer server.Close()

	get := func(path string) (int, string) {
		t.Helper()
		response, err := http.Get(server.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, string(body)
	}

	for _, tc := range []struct {
		path string
		code int
		body string
	}{
		{path: "/", code: http.StatusOK, body: "root index"},
		{path: "/plain.txt", code: http.StatusOK, body: "plain file"},
		{path: "/nested/", code: http.StatusOK, body: "nested index"},
		{path: "/without-index/", code: http.StatusNotFound},
		{path: "/missing", code: http.StatusNotFound},
		{path: "/.hidden.txt", code: http.StatusForbidden},
		{path: "/.well-known/challenge", code: http.StatusForbidden},
	} {
		t.Run(tc.path, func(t *testing.T) {
			code, body := get(tc.path)
			if code != tc.code {
				t.Fatalf("GET %s = %d %q, want %d", tc.path, code, body, tc.code)
			}
			if tc.body != "" && body != tc.body {
				t.Fatalf("GET %s body = %q, want %q", tc.path, body, tc.body)
			}
			if tc.path == "/without-index/" && strings.Contains(body, "file") {
				t.Fatalf("GET %s exposed a directory listing: %q", tc.path, body)
			}
		})
	}
}

func TestStaticHandlerContainsSymlinkTargets(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "visible.txt"), []byte("visible"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".hidden.txt"), []byte("hidden"), 0o644); err != nil {
		t.Fatal(err)
	}
	outsideFile := filepath.Join(outside, "outside.txt")
	if err := os.WriteFile(outsideFile, []byte("outside"), 0o644); err != nil {
		t.Fatal(err)
	}
	for link, target := range map[string]string{
		"contained-link": "visible.txt",
		"hidden-link":    ".hidden.txt",
		"outside-link":   outsideFile,
	} {
		if err := os.Symlink(target, filepath.Join(root, link)); err != nil {
			t.Skipf("create symlink: %v", err)
		}
	}
	canonicalRoot, err := canonicalStaticRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(staticHandler{root: canonicalRoot})
	defer server.Close()

	for _, tc := range []struct {
		path string
		code int
		body string
	}{
		{path: "/contained-link", code: http.StatusOK, body: "visible"},
		{path: "/hidden-link", code: http.StatusForbidden},
		{path: "/outside-link", code: http.StatusForbidden},
	} {
		t.Run(tc.path, func(t *testing.T) {
			response, err := http.Get(server.URL + tc.path)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != tc.code || (tc.body != "" && string(body) != tc.body) {
				t.Fatalf("GET %s = %d %q, want %d %q", tc.path, response.StatusCode, body, tc.code, tc.body)
			}
		})
	}
}

func unsetStaticEnv(t *testing.T, key string) {
	t.Helper()
	value, set := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if set {
			_ = os.Setenv(key, value)
		} else {
			_ = os.Unsetenv(key)
		}
	})
}

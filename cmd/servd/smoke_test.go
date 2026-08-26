package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

const smokeSlug = "ci-smoke"

func TestCLIStaticSiteLifecycle(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	tmp := t.TempDir()
	binary := filepath.Join(tmp, "servd")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	build := exec.Command("go", "build", "-trimpath", "-ldflags", strings.Join([]string{
		"-X github.com/reidransom/servd/internal/buildinfo.Version=ci-smoke",
		"-X github.com/reidransom/servd/internal/buildinfo.Commit=smoke-commit",
		"-X github.com/reidransom/servd/internal/buildinfo.Date=smoke-date",
	}, " "), "-o", binary, "./cmd/servd")
	build.Dir = root
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build smoke binary: %v\n%s", err, output)
	}

	project := filepath.Join(tmp, "site")
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}
	const fixture = "servd native lifecycle smoke"
	if err := os.WriteFile(filepath.Join(project, "index.html"), []byte(fixture), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".servd.toml"), []byte("cmd = \"servd static\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	configHome := filepath.Join(tmp, "config")
	stateHome := filepath.Join(tmp, "state")
	environment := append(os.Environ(),
		"XDG_CONFIG_HOME="+configHome,
		"XDG_STATE_HOME="+stateHome,
		"PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"),
	)
	port := availableSmokePort(t)
	runSmokeCommand(t, environment, binary, "add", project, "--slug", smokeSlug, "--port", fmt.Sprint(port))
	t.Cleanup(func() {
		cleanup := exec.Command(binary, "down", smokeSlug)
		cleanup.Env = environment
		_ = cleanup.Run()
	})

	version := runSmokeCommand(t, environment, binary, "version")
	if want := "servd version=ci-smoke commit=smoke-commit date=smoke-date"; strings.TrimSpace(version) != want {
		t.Fatalf("version = %q, want %q", strings.TrimSpace(version), want)
	}

	runSmokeCommand(t, environment, binary, "up", "--all", "--wait", "--timeout", "10s")
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if body := fetchSmokeSite(t, url); !strings.Contains(body, fixture) {
		t.Fatalf("GET %s = %q, want fixture content", url, body)
	}

	// The launching command has exited; the detached site must remain reachable.
	time.Sleep(200 * time.Millisecond)
	if body := fetchSmokeSite(t, url); !strings.Contains(body, fixture) {
		t.Fatalf("detached GET %s = %q, want fixture content", url, body)
	}

	logData, err := os.ReadFile(filepath.Join(stateHome, "servd", "logs", smokeSlug+".log"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(logData), `servd starting "`+smokeSlug+`"`) {
		t.Fatalf("site log missing startup header:\n%s", logData)
	}

	runSmokeCommand(t, environment, binary, "down", smokeSlug)
	waitForSmokePortClosed(t, port)
	status := runSmokeCommand(t, environment, binary, "status", smokeSlug, "--json")
	var payload struct {
		Sites []struct {
			Status string `json:"status"`
		} `json:"sites"`
	}
	if err := json.Unmarshal([]byte(status), &payload); err != nil {
		t.Fatalf("decode status: %v\n%s", err, status)
	}
	if len(payload.Sites) != 1 || payload.Sites[0].Status != "stopped" {
		t.Fatalf("status after down = %s", status)
	}
}

func runSmokeCommand(t *testing.T, environment []string, binary string, arguments ...string) string {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", binary, strings.Join(arguments, " "), err, output)
	}
	return string(output)
}

func availableSmokePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func fetchSmokeSite(t *testing.T, url string) string {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	response, err := client.Get(url)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := response.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %s, body = %s", url, response.Status, body)
	}
	return string(body)
}

func waitForSmokePortClosed(t *testing.T, port int) {
	t.Helper()
	address := fmt.Sprintf("127.0.0.1:%d", port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		connection, err := net.DialTimeout("tcp", address, 100*time.Millisecond)
		if err != nil {
			return
		}
		_ = connection.Close()
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("port %d remained open after servd down", port)
}

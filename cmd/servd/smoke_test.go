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
		cleanup := exec.Command(binary, "down", "--all")
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

	otherProject := filepath.Join(tmp, "other")
	if err := os.Mkdir(otherProject, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(otherProject, "index.html"), []byte("other site"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSmokeCommand(t, environment, binary, "add", otherProject, "--slug", "other", "--", "servd", "static")
	t.Chdir(project)
	runSmokeCommand(t, environment, binary, "up", "--wait", "--timeout", "10s")
	if body := fetchSmokeSite(t, url); !strings.Contains(body, fixture) {
		t.Fatalf("cwd-targeted GET %s = %q, want fixture content", url, body)
	}
	currentStatus := runSmokeCommand(t, environment, binary, "status", "--json")
	var current struct {
		Sites []struct {
			Slug   string `json:"slug"`
			Status string `json:"status"`
		} `json:"sites"`
	}
	if err := json.Unmarshal([]byte(currentStatus), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Sites) != 1 || current.Sites[0].Slug != smokeSlug || current.Sites[0].Status != "running" {
		t.Fatalf("cwd-targeted status = %s, want only running %s", currentStatus, smokeSlug)
	}
	otherStatus := runSmokeCommand(t, environment, binary, "status", "other", "--json")
	if err := json.Unmarshal([]byte(otherStatus), &current); err != nil {
		t.Fatal(err)
	}
	if len(current.Sites) != 1 || current.Sites[0].Slug != "other" || current.Sites[0].Status != "stopped" {
		t.Fatalf("explicit status = %s, want other still stopped", otherStatus)
	}
	runSmokeCommand(t, environment, binary, "up", "other", "--wait", "--timeout", "10s")
	otherPID := smokeSiteStatus(t, environment, binary, "other").PID
	runSmokeCommand(t, environment, binary, "down")
	waitForSmokePortClosed(t, port)
	if other := smokeSiteStatus(t, environment, binary, "other"); other.Status != "running" || other.PID != otherPID {
		t.Fatalf("cwd down changed the other site: %+v", other)
	}
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

	runSmokeCommand(t, environment, binary, "up", "--wait", "--timeout", "10s")
	beforeRestart := smokeSiteStatus(t, environment, binary, smokeSlug)
	next := filepath.Join(project, "next")
	if err := os.Mkdir(next, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(next, "index.html"), []byte("restarted command"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(project, ".servd.toml"), []byte("cmd = \"servd static --dir next\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSmokeCommand(t, environment, binary, "restart")
	waitForSmokeBody(t, url, "restarted command")
	if restarted := smokeSiteStatus(t, environment, binary, smokeSlug); restarted.PID == beforeRestart.PID {
		t.Fatalf("restart retained the previous process: %+v", restarted)
	}
	if other := smokeSiteStatus(t, environment, binary, "other"); other.Status != "running" || other.PID != otherPID {
		t.Fatalf("cwd restart changed the other site: %+v", other)
	}

	for _, command := range []string{"down", "restart"} {
		runSmokeFailure(t, environment, binary, "unknown site", command, "missing")
		runSmokeFailure(t, environment, binary, "not both", command, smokeSlug, "--all")
	}
	t.Run("lifecycle target boundaries", func(t *testing.T) {
		child := filepath.Join(project, "child")
		if err := os.Mkdir(child, 0o755); err != nil {
			t.Fatal(err)
		}
		t.Chdir(child)
		for _, command := range []string{"down", "restart"} {
			runSmokeFailure(t, environment, binary, "specify one or more slugs", command)
		}
		if err := os.WriteFile(filepath.Join(child, ".servd.toml"), []byte(`cmd = "unused"`), 0o644); err != nil {
			t.Fatal(err)
		}
		for _, command := range []string{"down", "restart"} {
			runSmokeFailure(t, environment, binary, "servd add .", command)
		}
		runSmokeCommand(t, environment, binary, "down", "--all")
		for _, slug := range []string{smokeSlug, "other"} {
			if site := smokeSiteStatus(t, environment, binary, slug); site.Status != "stopped" {
				t.Fatalf("down --all left %s active: %+v", slug, site)
			}
		}
		runSmokeCommand(t, environment, binary, "restart", "--all")
	})
	if t.Failed() {
		return
	}
	waitForSmokeBody(t, url, "restarted command")
	other := smokeSiteStatus(t, environment, binary, "other")
	waitForSmokeBody(t, other.DirectURL, "other site")
	currentPID := smokeSiteStatus(t, environment, binary, smokeSlug).PID
	runSmokeCommand(t, environment, binary, "down", "other")
	if site := smokeSiteStatus(t, environment, binary, "other"); site.Status != "stopped" {
		t.Fatalf("explicit down left other active: %+v", site)
	}
	runSmokeCommand(t, environment, binary, "restart", "other")
	waitForSmokeBody(t, other.DirectURL, "other site")
	if site := smokeSiteStatus(t, environment, binary, smokeSlug); site.PID != currentPID || site.Status != "running" {
		t.Fatalf("explicit lifecycle command changed cwd site: %+v", site)
	}
	otherPID = smokeSiteStatus(t, environment, binary, "other").PID
	if err := os.WriteFile(filepath.Join(project, ".servd.toml"), []byte("not toml [[["), 0o644); err != nil {
		t.Fatal(err)
	}
	runSmokeCommand(t, environment, binary, "down")
	waitForSmokePortClosed(t, port)
	runSmokeFailure(t, environment, binary, "invalid repository command", "restart")
	if other := smokeSiteStatus(t, environment, binary, "other"); other.PID != otherPID || other.Status != "running" {
		t.Fatalf("invalid cwd configuration affected other site: %+v", other)
	}
	if err := os.WriteFile(filepath.Join(project, ".servd.toml"), []byte("cmd = \"servd static --dir next\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runSmokeCommand(t, environment, binary, "down", smokeSlug, "other")
	runSmokeCommand(t, environment, binary, "restart", smokeSlug, "other")
	waitForSmokeBody(t, url, "restarted command")
	waitForSmokeBody(t, other.DirectURL, "other site")
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

type smokeSite struct {
	Slug      string `json:"slug"`
	Status    string `json:"status"`
	PID       int    `json:"pid"`
	URL       string `json:"url"`
	DirectURL string `json:"direct_url"`
}

func smokeSiteStatus(t *testing.T, environment []string, binary, slug string) smokeSite {
	t.Helper()
	output := runSmokeCommand(t, environment, binary, "status", slug, "--json")
	var payload struct {
		Sites []smokeSite `json:"sites"`
	}
	if err := json.Unmarshal([]byte(output), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Sites) != 1 || payload.Sites[0].Slug != slug {
		t.Fatalf("status %s = %s, want only the named site", slug, output)
	}
	return payload.Sites[0]
}

func waitForSmokeBody(t *testing.T, url, want string) {
	t.Helper()
	client := &http.Client{Timeout: time.Second}
	deadline := time.Now().Add(10 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		response, err := client.Get(url)
		if err == nil {
			body, readErr := io.ReadAll(response.Body)
			_ = response.Body.Close()
			got = string(body)
			if readErr == nil && response.StatusCode == http.StatusOK && got == want {
				return
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("GET %s = %q, want %q", url, got, want)
}

func runSmokeFailure(t *testing.T, environment []string, binary, want string, arguments ...string) string {
	t.Helper()
	command := exec.Command(binary, arguments...)
	command.Env = environment
	output, err := command.CombinedOutput()
	if _, ok := err.(*exec.ExitError); !ok || !strings.Contains(string(output), want) {
		t.Fatalf("%s %s: %v\n%s\nwant command failure containing %q", binary, strings.Join(arguments, " "), err, output, want)
	}
	return string(output)
}

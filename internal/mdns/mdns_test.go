package mdns

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestPublishUsesPlatformArgumentsAndCleansUp(t *testing.T) {
	binary, _ := publisherBinary()
	if binary == "" {
		t.Skip("mDNS publishing is unsupported on this platform")
	}
	dir := t.TempDir()
	argsPath := filepath.Join(dir, "args")
	fake := filepath.Join(dir, binary)
	script := "#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$ARGS_FILE\"\nexec /bin/sleep 30\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("ARGS_FILE", argsPath)

	publisher := NewPublisher()
	hostname, port, ip := "acme.local", 48123, "192.168.1.23"
	if err := publisher.Publish(context.Background(), hostname, port, ip); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = publisher.Cleanup() })
	waitForFile(t, argsPath)
	data, err := os.ReadFile(argsPath)
	if err != nil {
		t.Fatal(err)
	}
	var want []string
	if runtime.GOOS == "darwin" {
		want = []string{"-P", hostname, "_http._tcp", "local", strconv.Itoa(port), hostname, ip}
	} else {
		want = []string{"-R", hostname, ip}
	}
	if got := strings.Fields(string(data)); strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("mDNS args = %v, want %v", got, want)
	}
	if got := publisher.Published(); len(got) != 1 || got[0] != hostname {
		t.Fatalf("published = %v, want [%s]", got, hostname)
	}
	if err := publisher.Cleanup(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for len(publisher.Published()) != 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := publisher.Published(); len(got) != 0 {
		t.Fatalf("published after cleanup = %v", got)
	}
}

func TestPublishRejectsInvalidPort(t *testing.T) {
	publisher := NewPublisher()
	if err := publisher.Publish(context.Background(), "acme.local", 0, "192.168.1.23"); err == nil {
		t.Fatal("Publish accepted an invalid port")
	}
}

func waitForFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", path)
}

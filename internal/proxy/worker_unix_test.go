//go:build !windows

package proxy

import (
	"fmt"
	"net"
	"os"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/state"
)

// The test executable also runs the real inherited-listener worker, allowing
// StartBackground and StopBackground to execute in the same persistent parent.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "__proxy-worker" {
		settings, err := config.LoadSettings()
		if err == nil {
			err = RunInheritedWorker(settings, os.NewFile(3, "listener"), os.NewFile(4, "ready"))
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestBackgroundProxyStopsWhileParentRemainsAlive(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	settings := config.DefaultSettings()
	settings.Hostnames.HTTPPort = listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	if err := config.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := StartBackground(settings); err != nil {
		t.Fatal(err)
	}
	st, err := state.Load()
	if err != nil {
		t.Fatal(err)
	}
	entry, ok := st.Get(Slug)
	if !ok {
		t.Fatal("started proxy has no runtime entry")
	}
	t.Cleanup(func() {
		// Reap the child even when the regression leaves it a zombie.
		process, err := os.FindProcess(entry.PID)
		if err == nil {
			defer func() { _ = process.Release() }()
		}
		if err == nil && state.EntryAlive(entry) {
			_ = process.Kill()
			_, _ = process.Wait()
		}
	})
	if err := StopBackground(); err != nil {
		t.Fatal(err)
	}
	st, err = state.Load()
	if err != nil {
		t.Fatal(err)
	}
	if _, exists := st.Get(Slug); exists || state.EntryAlive(entry) {
		t.Fatal("stopped proxy still has live process or runtime state")
	}
}

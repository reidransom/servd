package proxy

import (
	"os"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
)

func TestProxySitePassesLANFlagToBackgroundProcess(t *testing.T) {
	settings := config.DefaultSettings()
	settings.EnableLAN()
	site, err := proxySite(settings)
	if err != nil {
		t.Fatal(err)
	}
	if suffix := launcher.ShellJoin([]string{"proxy", "--lan"}); !strings.HasSuffix(site.Cmd, suffix) {
		t.Fatalf("proxy command = %q, want LAN flag", site.Cmd)
	}
}

func TestProxySiteUsesValidWorkingDirectory(t *testing.T) {
	site, err := proxySite(config.DefaultSettings())
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(site.Path)
	if err != nil {
		t.Fatalf("proxy working directory %q: %v", site.Path, err)
	}
	if !info.IsDir() {
		t.Errorf("proxy working directory %q is not a directory", site.Path)
	}
}

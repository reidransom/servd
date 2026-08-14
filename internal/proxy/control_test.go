package proxy

import (
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

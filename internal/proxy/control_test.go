package proxy

import (
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestProxySitePassesLANFlagToBackgroundProcess(t *testing.T) {
	settings := config.DefaultSettings()
	settings.EnableLAN()
	site, err := proxySite(settings)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(site.Cmd, " proxy --lan") {
		t.Fatalf("proxy command = %q, want LAN flag", site.Cmd)
	}
}

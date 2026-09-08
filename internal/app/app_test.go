package app

import (
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
)

func TestFmtDuration(t *testing.T) {
	cases := []struct {
		d    time.Duration
		want string
	}{
		{45 * time.Second, "45s"},
		{90 * time.Second, "1m30s"},
		{3*time.Hour + 5*time.Minute, "3h5m"},
	}
	for _, c := range cases {
		if got := FmtDuration(c.d); got != c.want {
			t.Errorf("FmtDuration(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}

func TestDash(t *testing.T) {
	if Dash("") != "-" || Dash("x") != "x" {
		t.Error("Dash should substitute '-' only for empty strings")
	}
}

func TestLoadRejectsInvalidRegisteredHostnames(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := config.DefaultSettings()
	label := strings.Repeat("a", 62)
	settings.Hostnames.TLDsFallback = []string{strings.Join([]string{label, label, label, label}, ".")}
	if err := config.SaveSettings(settings); err != nil {
		t.Fatal(err)
	}
	registry := &config.Registry{Sites: []config.Site{{Slug: "acme", HostPrefix: "auth"}}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := Load(); err == nil || !strings.Contains(err.Error(), "253-character") {
		t.Fatalf("Load must reject a fallback that overflows with the stored site identity: %v", err)
	}
}

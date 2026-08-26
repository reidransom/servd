package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func writeConfig(t *testing.T, content string) {
	t.Helper()
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	configDir := filepath.Join(configHome, "servd")
	if err := os.Mkdir(configDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.toml"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadSettingsWithSourceReportsConfigPresence(t *testing.T) {
	t.Run("missing config", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", t.TempDir())

		_, source, err := LoadSettingsWithSource()
		if err != nil {
			t.Fatal(err)
		}
		if source.ConfigPresent {
			t.Fatal("ConfigPresent = true, want false")
		}
	})

	t.Run("present malformed config", func(t *testing.T) {
		writeConfig(t, "[hostnames\n")

		_, source, err := LoadSettingsWithSource()
		if err == nil {
			t.Fatal("LoadSettingsWithSource unexpectedly succeeded")
		}
		if !source.ConfigPresent {
			t.Fatal("ConfigPresent = false, want true")
		}
	})
}

func TestDefaultHostnameSettings(t *testing.T) {
	settings := DefaultSettings()
	if got := settings.Hostnames.TLDs; len(got) != 1 || got[0] != "localhost" {
		t.Fatalf("default TLDs = %v, want [localhost]", got)
	}
	if settings.Hostnames.HTTPS || settings.Hostnames.HTTPPort != 8080 || settings.Hostnames.HostsMode != HostsAuto || !settings.Hostnames.NipIO || settings.Hostnames.NipIOSuffix != "127.0.0.1.nip.io" {
		t.Fatalf("unexpected hostname defaults: %#v", settings.Hostnames)
	}
}

func TestLANForcesLocalHostnames(t *testing.T) {
	settings := DefaultSettings()
	settings.Hostnames.TLDs = []string{"dev.example.com"}
	settings.EnableLAN()
	if got := settings.SiteURL(Site{Slug: "acme"}); got != "http://acme.local:8080/" {
		t.Fatalf("LAN SiteURL = %q", got)
	}

	writeConfig(t, "[hostnames]\nlan = true\ntlds = [\"dev.example.com\"]\n")
	loaded, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := loaded.Hostnames.TLDs; len(got) != 1 || got[0] != "local" {
		t.Fatalf("loaded LAN TLDs = %v, want [local]", got)
	}
}

func TestSiteURLsUsePrimaryAndFallbackHostnames(t *testing.T) {
	site := Site{Slug: "acme", HostPrefix: "auth"}
	settings := DefaultSettings()
	if got := settings.SiteURL(site); got != "http://auth.acme.localhost:8080/" {
		t.Fatalf("default SiteURL = %q", got)
	}
	if got, ok := settings.FallbackURL(site); !ok || got != "http://auth.acme.127.0.0.1.nip.io:8080/" {
		t.Fatalf("fallback URL = %q, %v", got, ok)
	}

	settings.Hostnames = HostnameSettings{TLDs: []string{"dev.example.com"}, HTTPPort: 80, HostsMode: HostsAuto, NipIO: false}
	if got := settings.SiteURL(site); got != "http://auth.acme.dev.example.com/" {
		t.Fatalf("HTTP default-port SiteURL = %q", got)
	}
	if _, ok := settings.FallbackURL(site); ok {
		t.Fatal("disabled nip.io fallback emitted a URL")
	}
}

func TestLoadSettingsMigratesLegacyValuesWithNewPrecedence(t *testing.T) {
	t.Run("legacy proxy port fills missing HTTP port", func(t *testing.T) {
		writeConfig(t, "proxy_port = 9123\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings.Hostnames.HTTPPort != 9123 {
			t.Fatalf("legacy proxy port migration = %d, want 9123", settings.Hostnames.HTTPPort)
		}
	})
	t.Run("explicit hostname HTTP port wins", func(t *testing.T) {
		writeConfig(t, "proxy_port = 9123\n[hostnames]\nhttp_port = 8081\ntlds = [\"test\"]\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings.Hostnames.HTTPPort != 8081 || settings.SiteURL(Site{Slug: "acme"}) != "http://acme.test:8081/" {
			t.Fatalf("unexpected settings: %#v", settings)
		}
	})
	t.Run("legacy custom suffix preserves fallback", func(t *testing.T) {
		writeConfig(t, "domain_suffix = \"dev.example.com\"\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got := settings.Hostnames.TLDs; len(got) != 1 || got[0] != "dev.example.com" || !settings.Hostnames.NipIO {
			t.Fatalf("custom suffix migration = %#v", settings.Hostnames)
		}
	})
	t.Run("explicit TLDs win", func(t *testing.T) {
		writeConfig(t, "domain_suffix = \"dev.example.com\"\n[hostnames]\ntlds = [\"internal.test\"]\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got := settings.Hostnames.TLDs; len(got) != 1 || got[0] != "internal.test" {
			t.Fatalf("new TLD precedence = %v", got)
		}
	})
	t.Run("legacy sync hosts preserves intent", func(t *testing.T) {
		writeConfig(t, "[hostnames]\nsync_hosts = false\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings.Hostnames.HostsMode != HostsNever {
			t.Fatalf("hosts mode = %q, want never", settings.Hostnames.HostsMode)
		}
	})
}

func TestSaveSettingsOmitsLegacyKeys(t *testing.T) {
	configHome := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", configHome)
	if err := SaveSettings(DefaultSettings()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(configHome, "servd", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"proxy_port", "domain_suffix", "sync_hosts", "default_enabled"} {
		if strings.Contains(string(data), key) {
			t.Fatalf("saved settings contain legacy key %q:\n%s", key, data)
		}
	}
}

func TestLegacyEnablementKeysAreIgnoredAndOmittedOnSave(t *testing.T) {
	t.Run("settings", func(t *testing.T) {
		writeConfig(t, "default_enabled = true\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(settings, DefaultSettings()) {
			t.Fatalf("legacy default_enabled changed settings: %#v", settings)
		}
		if err := SaveSettings(settings); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(settingsPath())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "default_enabled") {
			t.Fatalf("saved settings contain default_enabled:\n%s", data)
		}
	})

	t.Run("registry", func(t *testing.T) {
		configHome := t.TempDir()
		t.Setenv("XDG_CONFIG_HOME", configHome)
		if err := os.MkdirAll(filepath.Join(configHome, "servd"), 0o755); err != nil {
			t.Fatal(err)
		}
		legacy := "[[site]]\nslug = \"alpha\"\npath = \"/tmp/alpha\"\nport = 4001\nlauncher = \"static\"\nenabled = true\n\n[[site]]\nslug = \"bravo\"\npath = \"/tmp/bravo\"\nport = 4002\nenabled = false\n"
		if err := os.WriteFile(registryPath(), []byte(legacy), 0o644); err != nil {
			t.Fatal(err)
		}
		registry, err := LoadRegistry()
		if err != nil {
			t.Fatal(err)
		}
		if len(registry.Sites) != 2 || registry.Find("alpha") == nil || registry.Find("bravo") == nil {
			t.Fatalf("legacy registry sites = %#v, want alpha and bravo", registry.Sites)
		}
		if err := registry.Save(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(registryPath())
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "enabled") || strings.Contains(string(data), "launcher") {
			t.Fatalf("saved registry retains removed fields:\n%s", data)
		}
	})
}

func TestSettingsValidation(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Settings)
	}{
		{"empty TLDs", func(s *Settings) { s.Hostnames.TLDs = nil }},
		{"invalid TLD", func(s *Settings) { s.Hostnames.TLDs = []string{"Bad"} }},
		{"invalid nip suffix", func(s *Settings) { s.Hostnames.NipIOSuffix = "bad..suffix" }},
		{"invalid port", func(s *Settings) { s.Hostnames.HTTPPort = 65536 }},
		{"invalid hosts mode", func(s *Settings) { s.Hostnames.HostsMode = "sometimes" }},
		{"unsupported HTTPS", func(s *Settings) { s.Hostnames.HTTPS = true }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			tc.edit(&settings)
			if err := settings.Validate(); err == nil {
				t.Fatal("Validate succeeded")
			}
		})
	}
}

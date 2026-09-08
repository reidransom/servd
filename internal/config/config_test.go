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

func TestDefaultHostnameRoutes(t *testing.T) {
	settings := DefaultSettings()
	site := Site{Slug: "acme"}
	hosts, err := settings.RouteHostnames(site)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(hosts, []string{"acme.localhost"}) {
		t.Fatalf("default routes = %v", hosts)
	}
	if len(settings.FallbackURLs(site)) != 0 || len(settings.FallbackURLPatterns()) != 0 {
		t.Fatal("default settings generated fallback URLs")
	}
}

func TestEnableMDNSForcesLocalHostnames(t *testing.T) {
	settings := DefaultSettings()
	settings.Hostnames.TLD = "dev.example.com"
	settings.EnableMDNS()
	if got := settings.SiteURL(Site{Slug: "acme"}); got != "http://acme.local:8080/" {
		t.Fatalf("mDNS SiteURL = %q", got)
	}

	writeConfig(t, "[hostnames]\nenable_mdns = true\ntlds = \"dev.example.com\"\ntlds_fallback = [\"local\", \"dev.example.com\", \"test\"]\n")
	loaded, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	hosts, err := loaded.RouteHostnames(Site{Slug: "acme"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"acme.local", "acme.dev.example.com", "acme.test"}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("mDNS routes = %v, want %v", hosts, want)
	}
	if got := loaded.PrimaryURLPattern(); got != "http://<slug>.local:8080/" {
		t.Fatalf("mDNS primary pattern = %q", got)
	}
	if got := loaded.FallbackURLPatterns(); !reflect.DeepEqual(got, []string{"http://<slug>.dev.example.com:8080/", "http://<slug>.test:8080/"}) {
		t.Fatalf("mDNS fallback patterns = %v", got)
	}
	if err := SaveSettings(loaded); err != nil {
		t.Fatal(err)
	}
	reloaded, err := LoadSettings()
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.SiteURL(Site{Slug: "acme"}); got != "http://acme.local:8080/" {
		t.Fatalf("saved mDNS setting was not restored: %q", got)
	}
	reloaded.Hostnames.EnableMDNS = false
	if got := reloaded.SiteURL(Site{Slug: "acme"}); got != "http://acme.dev.example.com:8080/" {
		t.Fatalf("saved mDNS settings lost declared primary: %q", got)
	}
}

func TestSiteURLsUsePrimaryAndFallbackHostnames(t *testing.T) {
	site := Site{Slug: "acme", HostPrefix: "auth"}
	settings := DefaultSettings()
	settings.Hostnames.TLDsFallback = []string{"127.0.0.1.nip.io", "localhost", "dev.example.com", "127.0.0.1.nip.io"}
	if got := settings.SiteURL(site); got != "http://auth.acme.localhost:8080/" {
		t.Fatalf("SiteURL = %q", got)
	}
	wantURLs := []string{"http://auth.acme.127.0.0.1.nip.io:8080/", "http://auth.acme.dev.example.com:8080/"}
	if got := settings.FallbackURLs(site); !reflect.DeepEqual(got, wantURLs) {
		t.Fatalf("fallback URLs = %v, want %v", got, wantURLs)
	}
	wantPatterns := []string{"http://<slug>.127.0.0.1.nip.io:8080/", "http://<slug>.dev.example.com:8080/"}
	if got := settings.FallbackURLPatterns(); !reflect.DeepEqual(got, wantPatterns) {
		t.Fatalf("fallback patterns = %v, want %v", got, wantPatterns)
	}
	hosts, err := settings.RouteHostnames(site)
	if err != nil {
		t.Fatal(err)
	}
	wantHosts := []string{"auth.acme.localhost", "auth.acme.127.0.0.1.nip.io", "auth.acme.dev.example.com"}
	if !reflect.DeepEqual(hosts, wantHosts) {
		t.Fatalf("route hosts = %v, want %v", hosts, wantHosts)
	}

	settings.Hostnames.TLD = "dev.example.com"
	settings.Hostnames.TLDsFallback = []string{"localhost"}
	settings.Hostnames.HTTPPort = 80
	if got := settings.SiteURL(site); got != "http://auth.acme.dev.example.com/" {
		t.Fatalf("HTTP default-port SiteURL = %q", got)
	}
	if got := settings.FallbackURLs(site); !reflect.DeepEqual(got, []string{"http://auth.acme.localhost/"}) {
		t.Fatalf("HTTP default-port fallback URLs = %v", got)
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
		writeConfig(t, "proxy_port = 9123\n[hostnames]\nhttp_port = 8081\ntlds = \"test\"\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if settings.Hostnames.HTTPPort != 8081 || settings.SiteURL(Site{Slug: "acme"}) != "http://acme.test:8081/" {
			t.Fatalf("unexpected settings: %#v", settings)
		}
	})
	for _, suffix := range []string{"dev.example.com", "127.0.0.1.nip.io"} {
		t.Run("legacy suffix "+suffix, func(t *testing.T) {
			writeConfig(t, "domain_suffix = \""+suffix+"\"\n")
			settings, err := LoadSettings()
			if err != nil {
				t.Fatal(err)
			}
			hosts, err := settings.RouteHostnames(Site{Slug: "acme"})
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(hosts, []string{"acme." + suffix}) {
				t.Fatalf("legacy suffix routes = %v", hosts)
			}
		})
	}
	t.Run("explicit primary wins", func(t *testing.T) {
		writeConfig(t, "domain_suffix = \"Bad\"\n[hostnames]\ntlds = \"internal.test\"\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		if got := settings.SiteURL(Site{Slug: "acme"}); got != "http://acme.internal.test:8080/" {
			t.Fatalf("new primary precedence URL = %q", got)
		}
	})
	t.Run("fallback key does not override legacy primary", func(t *testing.T) {
		writeConfig(t, "domain_suffix = \"dev.example.com\"\n[hostnames]\ntlds_fallback = [\"localhost\"]\n")
		settings, err := LoadSettings()
		if err != nil {
			t.Fatal(err)
		}
		hosts, err := settings.RouteHostnames(Site{Slug: "acme"})
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(hosts, []string{"acme.dev.example.com", "acme.localhost"}) {
			t.Fatalf("legacy primary and explicit fallback routes = %v", hosts)
		}
	})
	t.Run("explicit empty primary does not use legacy suffix", func(t *testing.T) {
		writeConfig(t, "domain_suffix = \"dev.example.com\"\n[hostnames]\ntlds = \"\"\n")
		if _, err := LoadSettings(); err == nil {
			t.Fatal("explicit empty primary was accepted")
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
	t.Run("explicit hosts mode wins", func(t *testing.T) {
		writeConfig(t, "[hostnames]\nsync_hosts = true\nhosts_mode = \"never\"\n")
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
	for _, key := range []string{"proxy_port", "domain_suffix", "sync_hosts", "default_enabled", "nip_io", "nip_io_suffix"} {
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
		{"empty primary", func(s *Settings) { s.Hostnames.TLD = "" }},
		{"invalid primary", func(s *Settings) { s.Hostnames.TLD = "Bad" }},
		{"invalid fallback", func(s *Settings) { s.Hostnames.TLDsFallback = []string{"test", "bad..suffix"} }},
		{"invalid mDNS primary", func(s *Settings) {
			s.Hostnames.TLD = "Bad"
			s.EnableMDNS()
		}},
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

func TestLoadSettingsRejectsInvalidHostnameSchema(t *testing.T) {
	for _, content := range []string{
		`tlds = ["localhost"]`,
		`tlds = []`,
		`tlds = 42`,
		`tlds_fallback = "test"`,
		`tlds_fallback = ["test", 42]`,
		`tlds_fallback = [""]`,
		`tlds_fallback = ["test", " bad"]`,
		`tlds = " localhost"`,
		"tlds = \"Bad\"\nenable_mdns = true",
		`enable_mdns = "true"`,
	} {
		t.Run(content, func(t *testing.T) {
			writeConfig(t, "[hostnames]\n"+content+"\n")
			_, err := LoadSettings()
			if err == nil {
				t.Fatal("invalid hostname schema was accepted")
			}
			if strings.HasPrefix(content, "tlds = [") && !strings.Contains(err.Error(), "tlds_fallback") {
				t.Fatalf("old primary array lacks migration guidance: %v", err)
			}
		})
	}
}

func TestLoadSettingsRejectsRemovedKeysBeforeSchemaMigration(t *testing.T) {
	for _, tc := range []struct {
		key      string
		settings string
	}{
		{"nip_io", "nip_io = false\n"},
		{"nip_io", "nip_io = true\ntlds = \"test\"\ntlds_fallback = []\n"},
		{"nip_io_suffix", "nip_io_suffix = \"100.101.102.103.nip.io\"\n"},
		{"nip_io_suffix", "nip_io_suffix = false\ntlds = [\"test\"]\n"},
	} {
		t.Run(tc.settings, func(t *testing.T) {
			content := "[hostnames]\n" + tc.settings
			writeConfig(t, content)
			_, source, err := LoadSettingsWithSource()
			if err == nil || !strings.Contains(err.Error(), "hostnames."+tc.key) || !strings.Contains(err.Error(), "tlds_fallback") {
				t.Fatalf("expected removed-key migration guidance, got %v", err)
			}
			if !source.ConfigPresent {
				t.Fatal("removed-key config was treated as absent")
			}
			data, err := os.ReadFile(settingsPath())
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != content {
				t.Fatal("loader rewrote rejected config")
			}
		})
	}
}

func TestLoadSettingsRejectsRenamedLANKey(t *testing.T) {
	for _, settings := range []string{
		"lan = true",
		"lan = false",
		"lan = true\nenable_mdns = false",
		"lan = false\nenable_mdns = true",
	} {
		t.Run(settings, func(t *testing.T) {
			content := "[hostnames]\n" + settings + "\n"
			writeConfig(t, content)
			_, source, err := LoadSettingsWithSource()
			if err == nil || !strings.Contains(err.Error(), "hostnames.enable_mdns") {
				t.Fatalf("expected mDNS rename guidance, got %v", err)
			}
			if !source.ConfigPresent {
				t.Fatal("renamed-key config was treated as absent")
			}
			data, err := os.ReadFile(settingsPath())
			if err != nil {
				t.Fatal(err)
			}
			if string(data) != content {
				t.Fatal("loader rewrote rejected config")
			}
		})
	}
}

func TestSettingsRoundTripPreservesDeclaredHostnames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		content   string
		primary   string
		fallbacks []string
	}{
		{"omitted table", "", "localhost", []string{}},
		{"omitted primary", "[hostnames]\ntlds_fallback = [\"test\"]\n", "localhost", []string{"test"}},
		{"omitted fallbacks", "[hostnames]\ntlds = \"test\"\n", "test", []string{}},
		{"explicit empty", "[hostnames]\ntlds = \"test\"\ntlds_fallback = []\n", "test", []string{}},
		{"redundant fallbacks", "[hostnames]\ntlds = \"test\"\ntlds_fallback = [\"other.test\", \"test\", \"other.test\", \"localhost\"]\n", "test", []string{"other.test", "test", "other.test", "localhost"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeConfig(t, tc.content)
			settings, err := LoadSettings()
			if err != nil {
				t.Fatal(err)
			}
			if err := SaveSettings(settings); err != nil {
				t.Fatal(err)
			}
			reloaded, err := LoadSettings()
			if err != nil {
				t.Fatal(err)
			}
			if reloaded.Hostnames.TLD != tc.primary || !reflect.DeepEqual(reloaded.Hostnames.TLDsFallback, tc.fallbacks) {
				t.Fatalf("round-trip hostname settings = %#v", reloaded.Hostnames)
			}
		})
	}
}

func TestRouteHostnamesPreserveStoredIdentity(t *testing.T) {
	for _, tc := range []struct {
		site Site
		tld  string
	}{
		{Site{Slug: "localhost", HostPrefix: "auth"}, "localhost"},
		{Site{Slug: "test", HostPrefix: "auth"}, "test"},
		{Site{Slug: "localhost", HostPrefix: "dev"}, "dev.localhost"},
		{Site{Slug: "app", HostPrefix: "test"}, "test"},
		{Site{Slug: "test"}, "test"},
	} {
		t.Run(tc.site.HostPrefix+"."+tc.site.Slug+"/"+tc.tld, func(t *testing.T) {
			settings := DefaultSettings()
			settings.Hostnames.TLD = tc.tld
			settings.Hostnames.TLDsFallback = []string{"example.com", "localhost"}
			base := tc.site.Slug
			if tc.site.HostPrefix != "" {
				base = tc.site.HostPrefix + "." + base
			}
			primary, err := settings.PrimaryHostname(tc.site)
			if err != nil || primary != base+"."+tc.tld {
				t.Fatalf("primary = %q, %v; want %q", primary, err, base+"."+tc.tld)
			}
			hosts, err := settings.RouteHostnames(tc.site)
			if err != nil {
				t.Fatal(err)
			}
			want := []string{base + "." + tc.tld, base + ".example.com"}
			if tc.tld != "localhost" {
				want = append(want, base+".localhost")
			}
			if !reflect.DeepEqual(hosts, want) {
				t.Fatalf("route hosts = %v, want %v", hosts, want)
			}
		})
	}
}

func TestRouteHostnamesRejectInvalidNamesWithoutPartialRoutes(t *testing.T) {
	label := strings.Repeat("a", 62)
	longSuffix := strings.Join([]string{label, label, label, label}, ".")
	for _, tc := range []struct {
		name string
		site Site
		edit func(*Settings)
	}{
		{"invalid slug", Site{Slug: "Bad"}, func(s *Settings) {}},
		{"invalid stored prefix", Site{Slug: "acme", HostPrefix: "Bad"}, func(s *Settings) {}},
		{"primary overflow", Site{Slug: "app"}, func(s *Settings) {
			s.Hostnames.TLD = longSuffix
			s.Hostnames.TLDsFallback = []string{"test"}
		}},
		{"fallback overflow", Site{Slug: "app"}, func(s *Settings) {
			s.Hostnames.TLDsFallback = []string{longSuffix, "test"}
		}},
		{"invalid primary", Site{Slug: "app"}, func(s *Settings) {
			s.Hostnames.TLD = "Bad"
			s.Hostnames.TLDsFallback = []string{"test"}
		}},
		{"invalid later fallback", Site{Slug: "app"}, func(s *Settings) {
			s.Hostnames.TLDsFallback = []string{"test", "Bad"}
		}},
		{"prefixed overflow", Site{Slug: strings.Repeat("b", 63), HostPrefix: strings.Repeat("c", 63)}, func(s *Settings) {
			s.Hostnames.TLDsFallback = []string{label + "a." + label}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := DefaultSettings()
			tc.edit(&settings)
			hosts, err := settings.RouteHostnames(tc.site)
			if err == nil || hosts != nil {
				t.Fatalf("invalid routes = %v, %v; want no routes and an error", hosts, err)
			}
		})
	}
}

func TestRouteHostnamesAllowDNSLengthLimit(t *testing.T) {
	label := strings.Repeat("a", 62)
	site := Site{Slug: strings.Repeat("b", 63), HostPrefix: strings.Repeat("c", 63)}
	settings := DefaultSettings()
	settings.Hostnames.TLD = label + "." + label
	settings.Hostnames.TLDsFallback = []string{"test"}
	hosts, err := settings.RouteHostnames(site)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		site.HostPrefix + "." + site.Slug + "." + settings.Hostnames.TLD,
		site.HostPrefix + "." + site.Slug + ".test",
	}
	if !reflect.DeepEqual(hosts, want) {
		t.Fatalf("routes at DNS length limit = %v, want %v", hosts, want)
	}
}

package commands

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestFallbackResolutionChecksRegisteredNames(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Hostnames.TLDsFallback = []string{"dev.example.com", "100.101.102.103.nip.io", "localhost", "dev.example.com"}
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "acme"},
		{Slug: "docs", HostPrefix: "auth"},
	}}
	wantNames := []string{
		"acme.dev.example.com",
		"acme.100.101.102.103.nip.io",
		"auth.docs.dev.example.com",
		"auth.docs.100.101.102.103.nip.io",
	}
	var lookedUp []string
	lookupHost := func(hostname string) ([]string, error) {
		lookedUp = append(lookedUp, hostname)
		return []string{"100.101.102.103"}, nil
	}
	var output bytes.Buffer
	if err := checkFallbackResolution(&output, settings, registry, lookupHost); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(lookedUp, wantNames) {
		t.Fatalf("resolved names = %v, want %v", lookedUp, wantNames)
	}
	for _, hostname := range wantNames {
		if !strings.Contains(output.String(), hostname+" -> [100.101.102.103]") {
			t.Errorf("remote fallback address missing for %s: %s", hostname, &output)
		}
	}
}

func TestFallbackResolutionFailuresAreAdvisory(t *testing.T) {
	settings := config.DefaultSettings()
	settings.Hostnames.TLDsFallback = []string{"missing.example.com", "empty.example.com", "working.example.com"}
	registry := &config.Registry{Sites: []config.Site{{Slug: "acme"}}}
	lookupHost := func(hostname string) ([]string, error) {
		switch hostname {
		case "acme.missing.example.com":
			return nil, errors.New("controlled DNS failure")
		case "acme.empty.example.com":
			return nil, nil
		case "acme.working.example.com":
			return []string{"192.0.2.10"}, nil
		default:
			t.Fatalf("unexpected DNS lookup: %s", hostname)
			return nil, nil
		}
	}
	var output bytes.Buffer
	if err := checkFallbackResolution(&output, settings, registry, lookupHost); err != nil {
		t.Fatalf("optional DNS failure became fatal: %v", err)
	}
	for _, hostname := range []string{"acme.missing.example.com", "acme.empty.example.com"} {
		if !strings.Contains(output.String(), hostname) {
			t.Errorf("missing advisory for %s: %s", hostname, &output)
		}
	}
	if !strings.Contains(output.String(), "controlled DNS failure") {
		t.Errorf("resolver error omitted: %s", &output)
	}
	if !strings.Contains(output.String(), "acme.working.example.com -> [192.0.2.10]") {
		t.Errorf("diagnostics stopped after optional failure: %s", &output)
	}
}

func TestFallbackResolutionSkipsWithoutEffectiveFallbackNames(t *testing.T) {
	for _, tc := range []struct {
		name      string
		sites     []config.Site
		fallbacks []string
	}{
		{name: "no registered sites", fallbacks: []string{"dev.example.com"}},
		{name: "no declared fallbacks", sites: []config.Site{{Slug: "acme"}}},
		{name: "only primary duplicates", sites: []config.Site{{Slug: "acme"}}, fallbacks: []string{"localhost", "localhost"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			settings := config.DefaultSettings()
			settings.Hostnames.TLDsFallback = tc.fallbacks
			var output bytes.Buffer
			err := checkFallbackResolution(&output, settings, &config.Registry{Sites: tc.sites}, func(hostname string) ([]string, error) {
				t.Fatalf("unexpected DNS lookup: %s", hostname)
				return nil, nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if output.Len() != 0 {
				t.Fatalf("fallback section printed without fallback names: %s", &output)
			}
		})
	}
}

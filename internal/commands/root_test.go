package commands

import (
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestSelectSitesAllReturnsEveryRegisteredSite(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	sites, err := selectSites(registry, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 2 || sites[0].Slug != "alpha" || sites[1].Slug != "bravo" {
		t.Fatalf("selectSites --all = %#v, want alpha and bravo", sites)
	}
	sites[0].Slug = "changed"
	if registry.Sites[0].Slug != "alpha" {
		t.Fatalf("selectSites --all mutated registry: %#v", registry.Sites)
	}
}

func TestSelectSitesExplicitSlugsAndErrors(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	sites, err := selectSites(registry, []string{"bravo"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Slug != "bravo" {
		t.Fatalf("selectSites bravo = %#v, want only bravo", sites)
	}

	for _, tc := range []struct {
		name string
		args []string
		all  bool
		want string
	}{
		{name: "all with slugs", args: []string{"alpha"}, all: true, want: "pass slugs or --all"},
		{name: "no target", want: "specify one or more slugs"},
		{name: "unknown slug", args: []string{"missing"}, want: `unknown site "missing"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := selectSites(registry, tc.args, tc.all)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("selectSites(%v, %v) error = %v, want %q", tc.args, tc.all, err, tc.want)
			}
		})
	}
}

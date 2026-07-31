package commands

import (
	"strings"
	"testing"

	"github.com/reidransom/servd/internal/config"
)

func TestStatusSites(t *testing.T) {
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "alpha", Port: 4001},
		{Slug: "bravo", Port: 4002},
	}}

	all, err := statusSites(registry, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("statusSites without slug returned %d sites, want 2", len(all))
	}

	sites, err := statusSites(registry, []string{"bravo"})
	if err != nil {
		t.Fatal(err)
	}
	if len(sites) != 1 || sites[0].Slug != "bravo" {
		t.Fatalf("statusSites for bravo = %#v, want only bravo", sites)
	}

	_, err = statusSites(registry, []string{"missing"})
	if err == nil || !strings.Contains(err.Error(), `unknown site "missing"`) {
		t.Fatalf("statusSites missing error = %v, want unknown-site error", err)
	}
}

func TestStatusCommandAcceptsAtMostOneSlug(t *testing.T) {
	command := newStatusCmd()
	if err := command.Args(command, []string{"alpha", "bravo"}); err == nil {
		t.Fatal("status accepted multiple slugs")
	}
}

// Package commands wires up the servd CLI (Cobra) and its subcommands.
package commands

import (
	"fmt"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/tui"
	"github.com/spf13/cobra"
)

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "servd",
		Short:         "Run and manage many local dev servers at once",
		Long:          "servd discovers web projects, runs each dev server on a stable port,\nand reverse-proxies them as <slug>.127.0.0.1.nip.io subdomains.",
		SilenceUsage:  true,
		SilenceErrors: true,
		// Bare `servd` launches the TUI.
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
	root.AddCommand(
		newAddCmd(),
		newRmCmd(),
		newWhichCmd(),
		newLaunchersCmd(),
		newStatusCmd(),
		newUpCmd(),
		newDownCmd(),
		newRestartCmd(),
		newEnableCmd(),
		newDisableCmd(),
		newLogsCmd(),
		newOpenCmd(),
		newProxyCmd(),
		newDoctorCmd(),
		newTUICmd(),
		newStaticCmd(),
	)
	return root
}

// selectSites resolves command args (slugs) plus an --all flag into sites.
//
// With --all and onlyEnabled, disabled sites are skipped — this is how bulk
// start operations (`up --all`, `restart --all`) honor enable/disable. Sites
// named explicitly by slug are always returned regardless of their enabled
// flag, so you can still start a disabled site on purpose.
func selectSites(reg *config.Registry, args []string, all, onlyEnabled bool) ([]config.Site, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("pass slugs or --all, not both")
	}
	if all {
		if !onlyEnabled {
			return reg.Sites, nil
		}
		var out []config.Site
		for _, s := range reg.Sites {
			if s.Enabled {
				out = append(out, s)
			}
		}
		return out, nil
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("specify one or more slugs, or --all")
	}
	var out []config.Site
	for _, slug := range args {
		s := reg.Find(slug)
		if s == nil {
			return nil, fmt.Errorf("unknown site %q (try `servd status`)", slug)
		}
		out = append(out, *s)
	}
	return out, nil
}

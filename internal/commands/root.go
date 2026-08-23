// Package commands wires up the servd CLI (Cobra) and its subcommands.
package commands

import (
	"fmt"
	"slices"

	"github.com/reidransom/servd/internal/buildinfo"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/tui"
	"github.com/spf13/cobra"
)

// Execute runs the root command.
func Execute() error {
	return newRootCmd().Execute()
}

func newRootCmd() *cobra.Command {
	version := buildinfo.String()
	root := &cobra.Command{
		Use:           "servd",
		Short:         "Run and manage many local dev servers at once",
		Long:          "servd runs registered web projects on stable ports and reverse-proxies them as local hostnames.",
		SilenceUsage:  true,
		SilenceErrors: true,
		Version:       version,
		// Bare `servd` launches the TUI.
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(
		newAddCmd(),
		newRmCmd(),
		newWhichCmd(),
		newLaunchersCmd(),
		newStatusCmd(),
		newUpCmd(),
		newDownCmd(),
		newRestartCmd(),
		newOpenCmd(),
		newProxyCmd(),
		newHostsCmd(),
		newDoctorCmd(),
		newTUICmd(),
		newVersionCmd(),
		newStaticCmd(),
	)
	addProxyInternalCommands(root)
	return root
}

// selectSites resolves command args (slugs) plus an --all flag into sites.
func selectSites(reg *config.Registry, args []string, all bool) ([]config.Site, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("pass slugs or --all, not both")
	}
	if all {
		return slices.Clone(reg.Sites), nil
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

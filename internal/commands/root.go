// Package commands wires up the servd CLI (Cobra) and its subcommands.
package commands

import (
	"fmt"
	"os"
	"path/filepath"
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
		newStatusCmd(),
		newUpCmd(),
		newDownCmd(),
		newRestartCmd(),
		newLogsCmd(),
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

// selectSites resolves slugs, --all, or the configured current directory into sites.
func selectSites(reg *config.Registry, args []string, all bool) ([]config.Site, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("pass slugs or --all, not both")
	}
	if all {
		return slices.Clone(reg.Sites), nil
	}
	args, err := defaultSiteArgs(reg, args)
	if err != nil {
		return nil, err
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

// selectSite resolves one explicit slug or the configured current directory.
func selectSite(cmd *cobra.Command, reg *config.Registry, args []string) (*config.Site, error) {
	args, err := defaultSiteArgs(reg, args)
	if err != nil {
		return nil, err
	}
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return nil, err
	}
	site := reg.Find(args[0])
	if site == nil {
		return nil, fmt.Errorf("unknown site %q", args[0])
	}
	return site, nil
}

// defaultSiteArgs targets the registered cwd only when it contains .servd.toml.
// Explicit slugs bypass directory detection.
func defaultSiteArgs(reg *config.Registry, args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get current directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(cwd, ".servd.toml")); os.IsNotExist(err) {
		return args, nil
	} else if err != nil {
		return nil, fmt.Errorf("check current directory configuration: %w", err)
	}
	site := reg.FindByPath(cwd)
	if site == nil {
		return nil, fmt.Errorf("current directory %q is not registered (run `servd add .`)", cwd)
	}
	return []string{site.Slug}, nil
}

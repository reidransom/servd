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

// selectSites resolves targets, --all, or the configured current directory into sites.
func selectSites(reg *config.Registry, args []string, all bool) ([]config.Site, error) {
	if all && len(args) > 0 {
		return nil, fmt.Errorf("pass targets or --all, not both")
	}
	if all {
		return slices.Clone(reg.Sites), nil
	}
	args, err := defaultSiteArgs(reg, args)
	if err != nil {
		return nil, err
	}
	if len(args) == 0 {
		return nil, fmt.Errorf("specify one or more targets (slugs or paths), or --all")
	}
	out := make([]config.Site, len(args))
	for i, target := range args {
		s, err := resolveSiteTarget(reg, target)
		if err != nil {
			return nil, err
		}
		out[i] = *s
	}
	return out, nil
}

// selectSite resolves one explicit target or the configured current directory.
func selectSite(cmd *cobra.Command, reg *config.Registry, args []string) (*config.Site, error) {
	args, err := defaultSiteArgs(reg, args)
	if err != nil {
		return nil, err
	}
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		return nil, err
	}
	return resolveSiteTarget(reg, args[0])
}

// resolveSiteTarget prefers a registered slug over a registered root directory.
func resolveSiteTarget(reg *config.Registry, target string) (*config.Site, error) {
	if site := reg.Find(target); site != nil {
		return site, nil
	}
	path, err := filepath.Abs(target)
	if err != nil {
		return nil, fmt.Errorf("resolve site directory %q: %w", target, err)
	}
	site, err := findSiteByDirectory(reg, path)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, fmt.Errorf("unknown site %q (try `servd status`)", target)
	}
	return site, nil
}

// defaultSiteArgs targets the registered cwd only when it contains .servd.toml.
// Explicit targets bypass current-directory inference.
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
	site, err := findSiteByDirectory(reg, cwd)
	if err != nil {
		return nil, err
	}
	if site == nil {
		return nil, fmt.Errorf("current directory %q is not registered (run `servd add .`)", cwd)
	}
	return []string{site.Slug}, nil
}

// findSiteByDirectory prefers an exact registered path, then directory identity
// (e.g. symlinks or Windows short names). It never searches parent directories.
func findSiteByDirectory(reg *config.Registry, path string) (*config.Site, error) {
	if site := reg.FindByPath(path); site != nil {
		return site, nil
	}
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("stat site directory %q: %w", path, err)
	}
	if !info.IsDir() {
		return nil, nil
	}
	var site *config.Site
	for i := range reg.Sites {
		candidate := &reg.Sites[i]
		candidateInfo, err := os.Stat(candidate.Path)
		if err != nil || !os.SameFile(info, candidateInfo) {
			continue
		}
		if site != nil {
			return nil, fmt.Errorf("directory %q matches multiple registered sites; specify a slug", path)
		}
		site = candidate
	}
	return site, nil
}

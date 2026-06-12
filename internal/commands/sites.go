package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/scan"
	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

func newScanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "scan [dir]",
		Short: "Discover servable projects and add them to the registry",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			root := settings.ProjectsDir
			if len(args) == 1 {
				root = args[0]
			}
			var added []scan.Result
			err = config.MutateRegistry(func(reg *config.Registry) error {
				added, err = scan.Scan(root, reg, settings)
				return err
			})
			if err != nil {
				return err
			}
			if len(added) == 0 {
				fmt.Printf("No new projects found under %s.\n", root)
				return nil
			}
			fmt.Printf("Added %d site(s):\n", len(added))
			for _, a := range added {
				fmt.Printf("  %-20s :%d  %s\n", a.Slug, a.Port, a.Path)
			}
			return nil
		},
	}
}

func newAddCmd() *cobra.Command {
	var slug, cmdline string
	var port int
	var enable bool
	c := &cobra.Command{
		Use:   "add <path>",
		Short: "Register a single project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			abs, err := filepath.Abs(args[0])
			if err != nil {
				return err
			}
			var site config.Site
			err = config.MutateRegistry(func(reg *config.Registry) error {
				if reg.FindByPath(abs) != nil {
					return fmt.Errorf("%s is already registered", abs)
				}
				if slug == "" {
					slug = scan.Slugify(filepath.Base(abs))
				}
				if reg.Find(slug) != nil {
					return fmt.Errorf("slug %q already in use", slug)
				}
				if port == 0 {
					port = scan.NextFreePort(reg, settings)
				} else if reg.HasPort(port) {
					return fmt.Errorf("port %d already assigned", port)
				}
				site = config.Site{Slug: slug, Path: abs, Port: port, Enabled: settings.DefaultEnabled || enable, Cmd: cmdline}
				if res, err := launcher.Resolve(site, settings); err == nil {
					site.Launcher = res.Kind
				} else if cmdline == "" {
					return fmt.Errorf("cannot determine how to serve %s; pass --cmd: %w", abs, err)
				}
				reg.Sites = append(reg.Sites, site)
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Printf("Added %s :%d (%s) enabled=%v\n  %s\n", site.Slug, site.Port, site.Launcher, site.Enabled, abs)
			return nil
		},
	}
	c.Flags().StringVar(&slug, "slug", "", "slug (defaults to folder name)")
	c.Flags().IntVar(&port, "port", 0, "port (defaults to next free)")
	c.Flags().StringVar(&cmdline, "cmd", "", "manual launch command (overrides detection; {port}/{host} allowed)")
	c.Flags().BoolVar(&enable, "enable", false, "enable the site immediately (overrides default_enabled)")
	return c
}

func newRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <slug>",
		Short: "Remove a site from the registry (stops it first)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			reg, err := config.LoadRegistry()
			if err != nil {
				return err
			}
			slug := args[0]
			if reg.Find(slug) == nil {
				return fmt.Errorf("unknown site %q", slug)
			}
			// Stop outside the registry lock — it can take several seconds.
			_ = supervisor.Stop(slug)
			err = config.MutateRegistry(func(reg *config.Registry) error {
				out := reg.Sites[:0]
				for _, s := range reg.Sites {
					if s.Slug != slug {
						out = append(out, s)
					}
				}
				reg.Sites = out
				return nil
			})
			if err != nil {
				return err
			}
			fmt.Printf("Removed %s.\n", slug)
			return nil
		},
	}
}

func newWhichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which <slug>",
		Short: "Show the resolved launch command for a site",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := load()
			if err != nil {
				return err
			}
			s := reg.Find(args[0])
			if s == nil {
				return fmt.Errorf("unknown site %q", args[0])
			}
			res, err := launcher.Resolve(*s, settings)
			if err != nil {
				return err
			}
			fmt.Printf("slug:     %s\n", s.Slug)
			fmt.Printf("path:     %s\n", s.Path)
			fmt.Printf("launcher: %s\n", res.Kind)
			fmt.Printf("port:     %d  (PORT/HOST exported to the process)\n", s.Port)
			fmt.Printf("command:  %s\n", res.Cmd)
			return nil
		},
	}
}

func newStatusCmd() *cobra.Command {
	c := &cobra.Command{
		Use:     "status",
		Aliases: []string{"ls"},
		Short:   "List sites with their port, URL, launcher and live status",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, st, err := load()
			if err != nil {
				return err
			}
			if len(reg.Sites) == 0 {
				fmt.Println("No sites registered. Run `servd scan`.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(tw, "SLUG\tPORT\tLAUNCHER\tENABLED\tSTATUS\tUPTIME\tURL")
			for _, s := range reg.Sites {
				status := supervisor.StatusOf(s, st)
				up := ""
				if d := supervisor.Uptime(s.Slug, st); d > 0 {
					up = app.FmtDuration(d)
				}
				fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
					s.Slug, s.Port, app.Dash(s.Launcher), enabledLabel(s.Enabled), status, app.Dash(up), settings.SiteURL(s))
			}
			return tw.Flush()
		},
	}
	return c
}

// newEnableCmd and newDisableCmd flip the registry `enabled` flag. Disabled
// sites are skipped by `up --all` / `restart --all` but can still be started
// explicitly by slug.
func newEnableCmd() *cobra.Command {
	return setEnabledCmd("enable", "Enable sites so `up --all` starts them", true)
}

func newDisableCmd() *cobra.Command {
	return setEnabledCmd("disable", "Disable sites so `up --all` skips them", false)
}

func setEnabledCmd(use, short string, enabled bool) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <slug...>",
		Short: short,
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			err := config.MutateRegistry(func(reg *config.Registry) error {
				for _, slug := range args {
					s := reg.Find(slug)
					if s == nil {
						return fmt.Errorf("unknown site %q", slug)
					}
					s.Enabled = enabled
				}
				return nil
			})
			if err != nil {
				return err
			}
			verb := "Enabled"
			if !enabled {
				verb = "Disabled"
			}
			fmt.Printf("%s: %v\n", verb, args)
			return nil
		},
	}
}

func enabledLabel(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

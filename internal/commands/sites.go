package commands

import (
	"fmt"
	"os"
	"slices"
	"text/tabwriter"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/registration"
	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

func newAddCmd() *cobra.Command {
	var slug, hostPrefix, cmdline string
	var port int
	var enable, noWorktreePrefix bool
	c := &cobra.Command{
		Use:   "add <path> [-- <command>...]",
		Short: "Register a single project",
		Args: func(cmd *cobra.Command, args []string) error {
			// Exactly one <path>; anything after `--` is the launch command.
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				if dash != 1 {
					return fmt.Errorf("expected exactly one <path> before --, got %d", dash)
				}
				if len(args) == dash {
					return fmt.Errorf("no command given after --")
				}
				return nil
			}
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				if cmdline != "" {
					return fmt.Errorf("use either --cmd or a trailing -- command, not both")
				}
				cmdline = launcher.ShellJoin(args[dash:])
			}
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			var site config.Site
			err = config.MutateRegistry(func(reg *config.Registry) error {
				site, err = registration.AddSite(reg, settings, registration.AddParams{
					Path: args[0], Slug: slug, HostPrefix: hostPrefix, NoWorktreePrefix: noWorktreePrefix,
					Port: port, Cmd: cmdline, Enable: enable,
				})
				return err
			})
			if err != nil {
				return err
			}
			fmt.Printf("Added %s :%d (%s) enabled=%v\n  %s\n  %s\n  direct: http://127.0.0.1:%d/\n",
				site.Slug, site.Port, site.Launcher, site.Enabled, site.Path, settings.SiteURL(site), site.Port)
			return nil
		},
	}
	c.Flags().StringVar(&slug, "slug", "", "stable CLI slug (defaults to inferred project name)")
	c.Flags().StringVar(&hostPrefix, "host-prefix", "", "hostname prefix for a linked worktree")
	c.Flags().BoolVar(&noWorktreePrefix, "no-worktree-prefix", false, "do not derive a hostname prefix from a linked worktree")
	c.Flags().IntVar(&port, "port", 0, "port (defaults to next free)")
	c.Flags().StringVar(&cmdline, "cmd", "", "manual launch command (overrides detection; {port}/{host} allowed; or pass it after --)")
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
				reg.Sites = slices.DeleteFunc(reg.Sites, func(s config.Site) bool { return s.Slug == slug })
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
			settings, reg, _, err := app.Load()
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
	var jsonOut bool
	c := &cobra.Command{
		Use:     "status",
		Aliases: []string{"ls"},
		Short:   "List sites with their port, URL, launcher and live status",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, st, err := app.Load()
			if err != nil {
				return err
			}
			if jsonOut {
				infos := make([]siteInfo, 0, len(reg.Sites))
				for _, s := range reg.Sites {
					infos = append(infos, newSiteInfo(settings, s, st))
				}
				return printJSON(struct {
					Proxy proxyInfo  `json:"proxy"`
					Sites []siteInfo `json:"sites"`
				}{newProxyInfo(settings, st), infos})
			}
			if len(reg.Sites) == 0 {
				fmt.Println("No sites registered. Run `servd add <path>`.")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			_, _ = fmt.Fprintln(tw, "SLUG\tPORT\tLAUNCHER\tENABLED\tSTATUS\tUPTIME\tURL")
			for _, s := range reg.Sites {
				status := supervisor.StatusOf(s, st)
				up := ""
				if d := supervisor.Uptime(s.Slug, st); d > 0 {
					up = app.FmtDuration(d)
				}
				_, _ = fmt.Fprintf(tw, "%s\t%d\t%s\t%s\t%s\t%s\t%s\n",
					s.Slug, s.Port, app.Dash(s.Launcher), enabledLabel(s.Enabled), status, app.Dash(up), settings.SiteURL(s))
			}
			return tw.Flush()
		},
	}
	c.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON (proxy + sites)")
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

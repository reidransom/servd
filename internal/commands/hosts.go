package commands

import (
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostsfile"
	"github.com/spf13/cobra"
)

func newHostsCmd() *cobra.Command {
	command := &cobra.Command{
		Use:   "hosts",
		Short: "Manage servd hostnames in the system hosts file",
	}
	command.AddCommand(newHostsStatusCmd(), newHostsSyncCmd(), newHostsCleanCmd())
	return command
}

func newHostsStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show servd hosts-file synchronization status",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, registry, err := loadHostsContext()
			if err != nil {
				return err
			}
			if settings.Hostnames.LAN {
				fmt.Println("LAN mode does not use the system hosts file.")
				return nil
			}
			desired, err := primaryHostnames(settings, registry)
			if err != nil {
				return err
			}
			managed, err := hostsfile.ManagedHostnames()
			if err != nil {
				return fmt.Errorf("could not read %s: %w", hostsfile.Path(), err)
			}
			if sameHostnames(desired, managed) {
				fmt.Printf("%s: synced (%d hostname(s))\n", hostsfile.Path(), len(desired))
				return nil
			}
			fmt.Printf("%s: not synced (%d managed, %d expected)\n", hostsfile.Path(), len(managed), len(desired))
			fmt.Println("run: sudo servd hosts sync")
			return nil
		},
	}
}

func newHostsSyncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync",
		Short: "Sync registered primary hostnames into the system hosts file",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, registry, err := loadHostsContext()
			if err != nil {
				return err
			}
			if err := syncHosts(settings, registry); err != nil {
				return err
			}
			hostnames, err := primaryHostnames(settings, registry)
			if err != nil {
				return err
			}
			fmt.Printf("Synced %d hostname(s) to %s.\n", len(hostnames), hostsfile.Path())
			return nil
		},
	}
}

func newHostsCleanCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clean",
		Short: "Remove only servd entries from the system hosts file",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, err := loadHostsContext()
			if err != nil {
				return err
			}
			if settings.Hostnames.LAN {
				return errors.New("cannot clean hosts entries while LAN mode is enabled")
			}
			if err := hostsfile.Clean(); err != nil {
				return hostsWriteError(err)
			}
			fmt.Printf("Removed servd entries from %s.\n", hostsfile.Path())
			return nil
		},
	}
}

func loadHostsContext() (config.Settings, *config.Registry, error) {
	settings, registry, _, err := app.Load()
	return settings, registry, err
}

func primaryHostnames(settings config.Settings, registry *config.Registry) ([]string, error) {
	seen := make(map[string]struct{})
	for _, site := range registry.Sites {
		hostnames, err := settings.PrimaryHostnames(site)
		if err != nil {
			return nil, fmt.Errorf("site %q: %w", site.Slug, err)
		}
		for _, hostname := range hostnames {
			seen[hostname] = struct{}{}
		}
	}
	hostnames := make([]string, 0, len(seen))
	for hostname := range seen {
		hostnames = append(hostnames, hostname)
	}
	sort.Strings(hostnames)
	return hostnames, nil
}

func sameHostnames(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func syncHosts(settings config.Settings, registry *config.Registry) error {
	if settings.Hostnames.LAN {
		return errors.New("LAN mode publishes .local hostnames through mDNS; it does not write the system hosts file")
	}
	hostnames, err := primaryHostnames(settings, registry)
	if err != nil {
		return err
	}
	if err := hostsfile.Sync(hostnames); err != nil {
		return hostsWriteError(err)
	}
	return nil
}

func syncHostsForProxy(settings config.Settings, registry *config.Registry) error {
	if settings.Hostnames.LAN {
		return nil
	}
	customTLD := hostsfile.NeedsHostsFile(settings.Hostnames.TLDs, "")
	switch settings.Hostnames.HostsMode {
	case config.HostsNever:
		if customTLD {
			fmt.Printf("warning: hosts-file synchronization is disabled; %v may not resolve locally\n", settings.Hostnames.TLDs)
		}
		return nil
	case config.HostsAlways:
		return syncHosts(settings, registry)
	case config.HostsAuto:
		if customTLD {
			return syncHosts(settings, registry)
		}
		managed, err := hostsfile.ManagedHostnames()
		if err != nil {
			return fmt.Errorf("could not inspect %s: %w", hostsfile.Path(), err)
		}
		if len(managed) > 0 {
			return syncHosts(settings, registry)
		}
		return nil
	default:
		return fmt.Errorf("unknown hosts mode %q", settings.Hostnames.HostsMode)
	}
}

func hostsWriteError(err error) error {
	if errors.Is(err, fs.ErrPermission) {
		return fmt.Errorf("could not update %s: permission denied\nretry with: sudo servd hosts sync", hostsfile.Path())
	}
	return fmt.Errorf("could not update %s: %w", hostsfile.Path(), err)
}

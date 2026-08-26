package commands

import (
	"fmt"
	"github.com/reidransom/servd/internal/mdns"
	"net"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/hostsfile"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/tui"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
}

// newLaunchersCmd prints the effective launcher rules — the user's
// launchers.toml entries followed by the surviving built-ins — in the same
// TOML format, so any rule can be copied into launchers.toml and tweaked.
func newLaunchersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "launchers",
		Short: "Print the effective launcher rules (launchers.toml + built-ins)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := launcher.MarshalRules(launcher.EffectiveRules())
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check tools, ports and hostname resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := app.Load()
			if err != nil {
				return err
			}
			ok := true

			fmt.Println("Launcher tools:")
			for _, bin := range launcher.Tools(launcher.EffectiveRules()) {
				if p, err := exec.LookPath(bin); err == nil {
					fmt.Printf("  ✓ %-7s %s\n", bin, p)
				} else {
					fmt.Printf("  · %-7s (not installed)\n", bin)
				}
			}

			fmt.Println("Proxy port:")
			if netcheck.PortFree(settings.BindHost, settings.Hostnames.HTTPPort) {
				fmt.Printf("  ✓ :%d is free\n", settings.Hostnames.HTTPPort)
			} else {
				fmt.Printf("  i :%d is in use (proxy may already be running)\n", settings.Hostnames.HTTPPort)
			}

			fmt.Println("Assigned ports:")
			conflicts := 0
			for _, s := range reg.Sites {
				if !netcheck.PortFree(settings.BindHost, s.Port) {
					conflicts++
				}
			}
			fmt.Printf("  %d site(s) registered, %d port(s) currently bound\n", len(reg.Sites), conflicts)

			fmt.Println("Primary hostname resolution:")
			if len(reg.Sites) == 0 {
				fmt.Println("  · no sites registered")
			} else {
				for _, site := range reg.Sites {
					hostname, err := settings.PrimaryHostname(site)
					if err != nil {
						return fmt.Errorf("site %q: %w", site.Slug, err)
					}
					result, err := hostsfile.CheckResolution(hostname)
					if err == nil && result.Loopback {
						fmt.Printf("  ✓ %s -> %v\n", hostname, result.Addresses)
						continue
					}
					if hostsfile.NeedsHostsFile(settings.Hostnames.TLDs, "") {
						ok = false
						fmt.Printf("  ✗ %s does not resolve to loopback; %s\n", hostname, hostsSyncInstruction())
					} else {
						fmt.Printf("  · %s does not resolve to loopback (%s)\n", hostname, hostsSyncInstruction())
					}
				}
			}

			if !settings.Hostnames.LAN && hostsfile.NeedsHostsFile(settings.Hostnames.TLDs, "") {
				fmt.Println("Hosts-file sync:")
				desired, err := primaryHostnames(settings, reg)
				if err != nil {
					return err
				}
				managed, err := hostsfile.ManagedHostnames()
				if err != nil {
					ok = false
					fmt.Printf("  ✗ could not read %s: %v\n", hostsfile.Path(), err)
				} else if sameHostnames(desired, managed) {
					fmt.Printf("  ✓ %d hostname(s) synced\n", len(desired))
				} else if settings.Hostnames.HostsMode == config.HostsNever {
					fmt.Println("  · synchronization disabled by hostnames.hosts_mode = \"never\"")
				} else {
					ok = false
					fmt.Printf("  ✗ %d hostname(s) expected; %s\n", len(desired), hostsSyncInstruction())
				}
			}

			if settings.Hostnames.LAN {
				fmt.Println("LAN mDNS publishing:")
				if supported, hint := mdns.Supported(); supported {
					fmt.Println("  ✓ platform publisher is available")
				} else {
					ok = false
					fmt.Printf("  ✗ %s\n", hint)
				}
			}

			if settings.Hostnames.NipIO {
				fmt.Println("nip.io fallback resolution:")
				host := "test." + settings.Hostnames.NipIOSuffix
				if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
					fmt.Printf("  ✓ %s -> %v\n", host, addrs)
				} else {
					fmt.Printf("  · could not resolve optional fallback %s: %v\n", host, err)
				}
			}

			if ok {
				fmt.Println("\nAll essential checks passed.")
			} else {
				fmt.Println("\nSome checks failed (see ✗ above).")
			}
			return nil
		},
	}
}


package commands

import (
	"fmt"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/mdns"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/spf13/cobra"
)

func newProxyCmd() *cobra.Command {
	var lan bool
	c := &cobra.Command{
		Use:   "proxy",
		Short: "Run the native hostname router (foreground)",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, _, err := app.Load()
			if err != nil {
				return err
			}
			settings, err = withLAN(settings, lan)
			if err != nil {
				return err
			}
			if err := requireMDNSSupport(settings); err != nil {
				return err
			}
			fmt.Printf("servd proxy listening on %s:%d — sites at %s\n",
				settings.BindHost, settings.Hostnames.HTTPPort, settings.PrimaryURLPattern())
			if fallback, ok := settings.FallbackURLPattern(); ok {
				fmt.Printf("nip.io fallback: %s\n", fallback)
			}
			return proxy.New(settings).ListenAndServe()
		},
	}
	c.PersistentFlags().BoolVar(&lan, "lan", false, "publish .local hostnames on the LAN with mDNS")
	c.AddCommand(newProxyUpCmd(&lan), newProxyDownCmd(), newProxyStatusCmd(&lan))
	return c
}

func newProxyUpCmd(lan *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the reverse proxy in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, registry, st, err := app.Load()
			if err != nil {
				return err
			}
			settings, err = withLAN(settings, *lan)
			if err != nil {
				return err
			}
			if err := requireMDNSSupport(settings); err != nil {
				return err
			}
			if running, pid := proxy.Running(st); running {
				effective := proxy.EffectiveSettings(settings, st)
				fmt.Printf("Proxy already running (pid %d) on :%d.\n", pid, effective.Hostnames.HTTPPort)
				return nil
			}
			if err := syncHostsForProxy(settings, registry); err != nil {
				return err
			}
			result, err := proxy.StartBackground(settings)
			if err != nil {
				return err
			}
			settings.Hostnames.HTTPPort = result.Port
			if result.UsedFallback {
				fmt.Printf("Could not acquire port 80: %v\n", result.PreferredErr)
				fmt.Println("Falling back to 127.0.0.1:8080.")
			}
			fmt.Printf("Proxy started on :%d — sites at %s\n", result.Port, settings.PrimaryURLPattern())
			if fallback, ok := settings.FallbackURLPattern(); ok {
				fmt.Printf("nip.io fallback: %s\n", fallback)
			}
			return nil
		},
	}
}

func newProxyDownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Stop the background reverse proxy",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := proxy.StopBackground(); err != nil {
				return err
			}
			fmt.Println("Proxy stopped.")
			return nil
		},
	}
}

func newProxyStatusCmd(lan *bool) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the reverse proxy is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, st, err := app.Load()
			if err != nil {
				return err
			}
			settings, err = withLAN(settings, *lan)
			if err != nil {
				return err
			}
			effective := proxy.EffectiveSettings(settings, st)
			running, pid := proxy.Running(st)
			switch {
			case running && proxy.Accepting(effective):
				fmt.Printf("running (pid %d) on :%d\n", pid, effective.Hostnames.HTTPPort)
			case running:
				fmt.Printf("starting (pid %d), not yet accepting on :%d\n", pid, effective.Hostnames.HTTPPort)
			default:
				fmt.Println("stopped")
			}
			if settings.Hostnames.LAN {
				if supported, hint := mdns.Supported(); supported {
					published := 0
					if entry, exists := st.Get(proxy.Slug); exists {
						published = len(entry.PublishedMDNS)
					}
					fmt.Printf("LAN mDNS: supported, %d hostname(s) published\n", published)
				} else {
					fmt.Printf("LAN mDNS: unavailable — %s\n", hint)
				}
			}
			return nil
		},
	}
}

func withLAN(settings config.Settings, enabled bool) (config.Settings, error) {
	if enabled {
		settings.EnableLAN()
	}
	return settings, nil
}

func requireMDNSSupport(settings config.Settings) error {
	if !settings.Hostnames.LAN {
		return nil
	}
	if supported, hint := mdns.Supported(); !supported {
		return fmt.Errorf("LAN mode is unavailable: %s", hint)
	}
	return nil
}

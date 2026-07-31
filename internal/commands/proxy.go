package commands

import (
	"fmt"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/spf13/cobra"
)

func newProxyCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "proxy",
		Short: "Run the native hostname router (foreground)",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, _, err := app.Load()
			if err != nil {
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
	c.AddCommand(newProxyUpCmd(), newProxyDownCmd(), newProxyStatusCmd())
	return c
}

func newProxyUpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Start the reverse proxy in the background",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, st, err := app.Load()
			if err != nil {
				return err
			}
			if running, pid := proxy.Running(st); running {
				fmt.Printf("Proxy already running (pid %d) on :%d.\n", pid, settings.Hostnames.HTTPPort)
				return nil
			}
			if err := proxy.StartBackground(settings); err != nil {
				return err
			}
			fmt.Printf("Proxy started on :%d — sites at %s\n", settings.Hostnames.HTTPPort, settings.PrimaryURLPattern())
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

func newProxyStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether the reverse proxy is running",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, _, st, err := app.Load()
			if err != nil {
				return err
			}
			running, pid := proxy.Running(st)
			switch {
			case running && proxy.Accepting(settings):
				fmt.Printf("running (pid %d) on :%d\n", pid, settings.Hostnames.HTTPPort)
			case running:
				fmt.Printf("starting (pid %d), not yet accepting on :%d\n", pid, settings.Hostnames.HTTPPort)
			default:
				fmt.Println("stopped")
			}
			return nil
		},
	}
}

//go:build windows

package commands

import (
	"log"
	"os"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

func addProxyInternalCommands(root *cobra.Command) {
	root.AddCommand(newProxyWorkerCmd())
}

func newProxyWorkerCmd() *cobra.Command {
	var port int
	var lan bool
	command := &cobra.Command{
		Use:    "__proxy-worker",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			settings.Hostnames.HTTPPort = port
			if lan {
				settings.EnableLAN()
			}
			if err := os.MkdirAll(config.LogDir(), 0o755); err != nil {
				return err
			}
			logFile, err := os.OpenFile(supervisor.LogPath(proxy.Slug), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
			if err != nil {
				return err
			}
			defer logFile.Close()
			log.SetOutput(logFile)
			return proxy.New(settings).ListenAndServe()
		},
	}
	command.Flags().IntVar(&port, "port", 0, "listener port")
	command.Flags().BoolVar(&lan, "lan", false, "enable LAN mode")
	_ = command.MarkFlagRequired("port")
	return command
}

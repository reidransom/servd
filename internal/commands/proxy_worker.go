//go:build !windows

package commands

import (
	"fmt"
	"log"
	"os"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

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
			logFile, err := openProxyWorkerLog()
			if err != nil {
				return err
			}
			defer logFile.Close()
			log.SetOutput(logFile)

			listenerFile := os.NewFile(3, "proxy-listener")
			readyFile := os.NewFile(4, "proxy-ready")
			if listenerFile == nil || readyFile == nil {
				return fmt.Errorf("missing proxy listener descriptors")
			}
			defer listenerFile.Close()
			return proxy.RunInheritedWorker(settings, listenerFile, readyFile)
		},
	}
	command.Flags().IntVar(&port, "port", 0, "listener port")
	command.Flags().BoolVar(&lan, "lan", false, "enable LAN mode")
	_ = command.MarkFlagRequired("port")
	return command
}

func openProxyWorkerLog() (*os.File, error) {
	if err := os.MkdirAll(config.LogDir(), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(supervisor.LogPath(proxy.Slug), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
}

//go:build !windows

package commands

import (
	"fmt"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/spf13/cobra"
	"os"
	"strconv"
)

func addProxyInternalCommands(root *cobra.Command) {
	root.AddCommand(newProxyBindCmd(), newProxyWorkerCmd())
}

func newProxyBindCmd() *cobra.Command {
	var worker, groups, home, configHome, stateHome, bindHost string
	var port int
	var uid, gid uint32
	var lan bool
	command := &cobra.Command{
		Use:    "__proxy-bind",
		Hidden: true,
		Args:   cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsedGroups, err := proxy.ParseGroups(groups)
			if err != nil {
				return err
			}
			if err := validateBindIdentity(uid, gid, parsedGroups); err != nil {
				return err
			}
			pid, err := proxy.RunPrivilegedBind(proxy.BindRequest{
				BindHost: bindHost, Worker: worker, Port: port, LAN: lan, UID: uid, GID: gid, Groups: parsedGroups,
				Home: home, ConfigHome: configHome, StateHome: stateHome,
			})
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), pid)
			return err
		},
	}
	command.Flags().StringVar(&worker, "worker", "", "worker executable")
	command.Flags().IntVar(&port, "port", 0, "listener port")
	command.Flags().Uint32Var(&uid, "uid", 0, "worker user ID")
	command.Flags().Uint32Var(&gid, "gid", 0, "worker group ID")
	command.Flags().StringVar(&groups, "groups", "", "worker supplementary groups")
	command.Flags().StringVar(&home, "home", "", "worker home directory")
	command.Flags().StringVar(&bindHost, "bind-host", "", "listener bind host")
	command.Flags().StringVar(&configHome, "config-home", "", "worker XDG config directory")
	command.Flags().StringVar(&stateHome, "state-home", "", "worker XDG state directory")
	command.Flags().BoolVar(&lan, "lan", false, "enable LAN mode")
	_ = command.MarkFlagRequired("worker")
	_ = command.MarkFlagRequired("bind-host")
	_ = command.MarkFlagRequired("port")
	return command
}

func validateBindIdentity(uid, gid uint32, groups []uint32) error {
	sudoUID, err := sudoIdentity("SUDO_UID")
	if err != nil {
		return err
	}
	sudoGID, err := sudoIdentity("SUDO_GID")
	if err != nil {
		return err
	}
	if uid != sudoUID || gid != sudoGID {
		return fmt.Errorf("requested worker identity does not match the sudo caller")
	}
	actual, err := os.Getgroups()
	if err != nil {
		return fmt.Errorf("read sudo caller groups: %w", err)
	}
	return validateSupplementaryGroups(gid, groups, actual)
}

func validateSupplementaryGroups(gid uint32, groups []uint32, actual []int) error {
	allowed := map[uint32]struct{}{gid: {}}
	for _, group := range actual {
		allowed[uint32(group)] = struct{}{}
	}
	for _, group := range groups {
		if _, ok := allowed[group]; !ok {
			return fmt.Errorf("requested supplementary group %d does not belong to the sudo caller", group)
		}
	}
	return nil
}

func sudoIdentity(name string) (uint32, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, fmt.Errorf("%s is required for the privileged bind helper", name)
	}
	value, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid %s: %w", name, err)
	}
	return uint32(value), nil
}

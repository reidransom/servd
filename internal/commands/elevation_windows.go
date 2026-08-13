//go:build windows

package commands

func hostsSyncInstruction() string {
	return "run an elevated terminal, then: servd hosts sync"
}

func hostsPermissionInstruction() string {
	return "retry from an elevated terminal: servd hosts sync"
}

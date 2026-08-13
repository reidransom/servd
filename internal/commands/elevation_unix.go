//go:build !windows

package commands

func hostsSyncInstruction() string {
	return "run: sudo servd hosts sync"
}

func hostsPermissionInstruction() string {
	return "retry with: sudo servd hosts sync"
}

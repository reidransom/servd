//go:build windows

package proxy

import "os"

func interruptSignals() []os.Signal {
	return []os.Signal{os.Interrupt}
}

// Package netcheck has small TCP port probes shared by scan, doctor and the
// proxy control code.
package netcheck

import (
	"net"
	"strconv"
)

// PortFree reports whether host:port can be bound (i.e. nothing is listening).
func PortFree(host string, port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

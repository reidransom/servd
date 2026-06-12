// Package netcheck has small TCP port probes shared by scan, doctor and the
// proxy control code.
package netcheck

import (
	"net"
	"strconv"
	"time"
)

// PortFree reports whether host:port can be bound (i.e. nothing is listening).
func PortFree(host string, port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	_ = ln.Close()
	return true
}

// PortAccepting reports whether host:port is accepting TCP connections.
func PortAccepting(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", net.JoinHostPort(host, strconv.Itoa(port)), 200*time.Millisecond)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// Package mdns publishes servd LAN hostnames for the lifetime of the proxy.
package mdns

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"sync"
	"time"
)

type commandFactory func(context.Context, string, ...string) *exec.Cmd
type lookPath func(string) (string, error)

// Publisher owns the platform publisher processes for one proxy instance.
type Publisher struct {
	mu        sync.Mutex
	processes map[string]*exec.Cmd
	command   commandFactory
	lookPath  lookPath
}

// NewPublisher creates an empty mDNS publisher.
func NewPublisher() *Publisher {
	return &Publisher{
		processes: make(map[string]*exec.Cmd),
		command:   exec.CommandContext,
		lookPath:  exec.LookPath,
	}
}

// Supported reports whether the platform mDNS publisher is available.
func Supported() (bool, string) {
	return NewPublisher().Supported()
}

// Supported reports whether this publisher can start its platform command.
func (p *Publisher) Supported() (bool, string) {
	binary, hint := publisherBinary()
	if binary == "" {
		return false, hint
	}
	if _, err := p.lookPath(binary); err != nil {
		return false, fmt.Sprintf("%s is required for LAN mode; %s", binary, hint)
	}
	return true, ""
}

// Publish starts a publisher process for hostname when it is not already
// published. The process remains active until Unpublish, Cleanup, or ctx ends.
func (p *Publisher) Publish(ctx context.Context, hostname string, port int, ip string) error {
	if hostname == "" || ip == "" {
		return fmt.Errorf("hostname and LAN IP are required for mDNS publishing")
	}
	binary, args, err := publisherCommand(hostname, port, ip)
	if err != nil {
		return err
	}
	if _, err := p.lookPath(binary); err != nil {
		_, hint := publisherBinary()
		return fmt.Errorf("%s is required for LAN mode; %s", binary, hint)
	}

	p.mu.Lock()
	defer p.mu.Unlock()
	if _, exists := p.processes[hostname]; exists {
		return nil
	}
	cmd := p.command(ctx, binary, args...)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start mDNS publisher for %s: %w", hostname, err)
	}
	p.processes[hostname] = cmd
	go p.wait(hostname, cmd)
	return nil
}

// Unpublish stops the publisher process for hostname. It is a no-op for an
// unpublished hostname.
func (p *Publisher) Unpublish(hostname string) error {
	p.mu.Lock()
	cmd, exists := p.processes[hostname]
	if exists {
		delete(p.processes, hostname)
	}
	p.mu.Unlock()
	if !exists || cmd.Process == nil {
		return nil
	}
	if err := cmd.Process.Kill(); err != nil && err.Error() != "os: process already finished" {
		return fmt.Errorf("stop mDNS publisher for %s: %w", hostname, err)
	}
	return nil
}

// Cleanup stops every active publisher process.
func (p *Publisher) Cleanup() error {
	for _, hostname := range p.Published() {
		if err := p.Unpublish(hostname); err != nil {
			return err
		}
	}
	return nil
}

// Published returns the currently active hostname set in sorted order.
func (p *Publisher) Published() []string {
	p.mu.Lock()
	hostnames := make([]string, 0, len(p.processes))
	for hostname := range p.processes {
		hostnames = append(hostnames, hostname)
	}
	p.mu.Unlock()
	sort.Strings(hostnames)
	return hostnames
}

// DetectLANIP returns the first active, non-loopback IPv4 interface address.
func DetectLANIP() (string, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range interfaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}
		addresses, err := iface.Addrs()
		if err != nil {
			continue
		}
		for _, address := range addresses {
			ip, _, err := net.ParseCIDR(address.String())
			if err != nil || ip.To4() == nil || ip.IsLoopback() || !ip.IsGlobalUnicast() {
				continue
			}
			return ip.String(), nil
		}
	}
	return "", fmt.Errorf("could not detect an active LAN IPv4 address; set hostnames.lan_ip")
}

// StartLANIPMonitor checks for address changes until ctx is canceled. The
// callback is invoked only after a successful change from the initial value.
func StartLANIPMonitor(ctx context.Context, initial string, onChange func(next, previous string)) {
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		previous := initial
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				next, err := DetectLANIP()
				if err != nil || next == previous {
					continue
				}
				onChange(next, previous)
				previous = next
			}
		}
	}()
}

func (p *Publisher) wait(hostname string, cmd *exec.Cmd) {
	_ = cmd.Wait()
	p.mu.Lock()
	if p.processes[hostname] == cmd {
		delete(p.processes, hostname)
	}
	p.mu.Unlock()
}

func publisherBinary() (string, string) {
	switch runtime.GOOS {
	case "darwin":
		return "dns-sd", "install the macOS Bonjour tools"
	case "linux":
		return "avahi-publish-address", "install avahi-utils"
	default:
		return "", "LAN mode is supported on macOS and Linux"
	}
}

func publisherCommand(hostname string, port int, ip string) (string, []string, error) {
	if port < 1 || port > 65535 {
		return "", nil, fmt.Errorf("invalid mDNS port %d", port)
	}
	binary, hint := publisherBinary()
	if binary == "" {
		return "", nil, fmt.Errorf("%s", hint)
	}
	switch runtime.GOOS {
	case "darwin":
		return binary, []string{"-P", hostname, "_http._tcp", "local", strconv.Itoa(port), hostname, ip}, nil
	case "linux":
		return binary, []string{"-R", hostname, ip}, nil
	default:
		return "", nil, fmt.Errorf("%s", hint)
	}
}

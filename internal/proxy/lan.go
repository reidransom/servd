package proxy

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/mdns"
	"github.com/reidransom/servd/internal/state"
)

func (s *Server) startLAN(ctx context.Context) error {
	if !s.settings.Hostnames.LAN {
		return nil
	}
	if supported, hint := mdns.Supported(); !supported {
		return fmt.Errorf("LAN mode is unavailable: %s", hint)
	}
	ip := s.settings.Hostnames.LANIP
	if ip == "" {
		var err error
		ip, err = mdns.DetectLANIP()
		if err != nil {
			return err
		}
	}
	publisher := mdns.NewPublisher()
	s.mu.Lock()
	s.publisher = publisher
	s.lanContext = ctx
	s.lanIP = ip
	sites := append([]config.Site(nil), s.sites...)
	s.mu.Unlock()
	if err := s.reconcileMDNS(sites); err != nil {
		_ = publisher.Cleanup()
		return err
	}
	if s.settings.Hostnames.LANIP == "" {
		mdns.StartLANIPMonitor(ctx, ip, s.replaceLANIP)
	}
	return nil
}

func (s *Server) stopLAN() {
	s.mu.Lock()
	publisher := s.publisher
	s.publisher = nil
	s.lanContext = nil
	s.mu.Unlock()
	s.recordPublishedMDNS(nil)
	if publisher != nil {
		_ = publisher.Cleanup()
	}
}

func (s *Server) watchRegistry(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reload()
		}
	}
}

func (s *Server) replaceLANIP(next, _ string) {
	s.mu.Lock()
	publisher := s.publisher
	s.lanIP = next
	sites := append([]config.Site(nil), s.sites...)
	s.mu.Unlock()
	if publisher == nil {
		return
	}
	for _, hostname := range publisher.Published() {
		if err := publisher.Unpublish(hostname); err != nil {
			log.Printf("proxy: stop mDNS publisher for %s: %v", hostname, err)
		}
	}
	if err := s.reconcileMDNS(sites); err != nil {
		log.Printf("proxy: republish mDNS after LAN IP change: %v", err)
	}
}

func (s *Server) reconcileMDNS(sites []config.Site) error {
	s.mu.RLock()
	publisher, ctx, ip := s.publisher, s.lanContext, s.lanIP
	s.mu.RUnlock()
	if publisher == nil || ctx == nil {
		return nil
	}
	desired := make(map[string]struct{})
	for _, site := range sites {
		siteHostnames, err := s.settings.PrimaryHostnames(site)
		if err != nil {
			return fmt.Errorf("site %q: %w", site.Slug, err)
		}
		for _, hostname := range siteHostnames {
			desired[hostname] = struct{}{}
		}
	}
	for _, hostname := range publisher.Published() {
		if _, keep := desired[hostname]; keep {
			continue
		}
		if err := publisher.Unpublish(hostname); err != nil {
			return err
		}
	}
	hostnames := make([]string, 0, len(desired))
	for hostname := range desired {
		hostnames = append(hostnames, hostname)
	}
	sort.Strings(hostnames)
	for _, hostname := range hostnames {
		if err := publisher.Publish(ctx, hostname, s.settings.Hostnames.HTTPPort, ip); err != nil {
			return err
		}
	}
	s.recordPublishedMDNS(publisher.Published())
	return nil
}

func (s *Server) recordPublishedMDNS(hostnames []string) {
	if err := state.Mutate(func(runtime *state.State) error {
		entry, exists := runtime.Get(Slug)
		if !exists {
			return nil
		}
		entry.PublishedMDNS = append(entry.PublishedMDNS[:0], hostnames...)
		runtime.Entries[Slug] = entry
		return nil
	}); err != nil {
		log.Printf("proxy: record mDNS publishers: %v", err)
	}
}

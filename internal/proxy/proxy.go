// Package proxy is an exact-host routing reverse proxy.
//
// Requests are routed only when their normalized full Host header matches a
// hostname generated from the site's stored hostname identity. The registry is
// reloaded whenever sites.toml changes, so newly added sites route without a
// restart. Unknown or bare hosts get a landing page listing every site.
//
// The Host header sent to the backend is rewritten to the backend's own
// address by default, so dev servers with host allowlists (Vite 5+, Next,
// Rails host authorization) accept proxied requests. Sites that need the
// original Host can set preserve_host = true. X-Forwarded-Host/Proto/For are
// always set from the inbound request.
//
// httputil.ReverseProxy transparently handles HTTP/1.1 Upgrade (websockets),
// which jigyll live-reload and Vite/Next HMR depend on.
package proxy

import (
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/reidransom/servd/internal/config"
)

// Server is a running proxy instance.
type Server struct {
	settings config.Settings

	mu           sync.RWMutex
	routes       map[string]*httputil.ReverseProxy // normalized FQDN -> backend proxy
	sites        []config.Site                     // for the landing page (sorted by slug)
	regMtime     time.Time
	lastErrMtime time.Time // mtime of the last sites.toml version we logged a reload error for
}

// New builds a proxy server and loads the initial routing table.
func New(settings config.Settings) *Server {
	s := &Server{settings: settings, routes: map[string]*httputil.ReverseProxy{}}
	s.reload()
	return s
}

// ListenAndServe starts the proxy on the active HTTP listener port.
func (s *Server) ListenAndServe() error {
	addr := net.JoinHostPort(s.settings.BindHost, strconv.Itoa(s.settings.Hostnames.HTTPPort))
	srv := &http.Server{Addr: addr, Handler: s}
	return srv.ListenAndServe()
}

// buildProxy creates the reverse proxy for one site.
func buildProxy(settings config.Settings, site config.Site) *httputil.ReverseProxy {
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(settings.BindHost, strconv.Itoa(site.Port))}
	slug, port, preserve := site.Slug, site.Port, site.PreserveHost
	return &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetXForwarded()
			pr.SetURL(target) // also rewrites Out.Host to the backend address
			if preserve {
				pr.Out.Host = pr.In.Host
			}
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, err error) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = fmt.Fprintf(w, "servd: %q is registered but not responding on port %d.\n", slug, port)
			_, _ = fmt.Fprintf(w, "Start it with: servd up %s\n\n(%v)\n", slug, err)
		},
	}
}

// buildRouteTable creates one backend proxy per site and assigns it to every
// exact primary and optional fallback hostname generated for that site.
func buildRouteTable(settings config.Settings, sites []config.Site) (map[string]*httputil.ReverseProxy, error) {
	routes := make(map[string]*httputil.ReverseProxy)
	owners := make(map[string]string)
	for _, site := range sites {
		backend := buildProxy(settings, site)
		hosts, err := settings.RouteHostnames(site)
		if err != nil {
			return nil, fmt.Errorf("site %q: %w", site.Slug, err)
		}
		seen := make(map[string]struct{}, len(hosts))
		for _, host := range hosts {
			key := normalizeRequestHostname(host)
			if key == "" {
				return nil, fmt.Errorf("site %q generated invalid hostname %q", site.Slug, host)
			}
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			if owner, exists := owners[key]; exists {
				return nil, fmt.Errorf("hostname collision for %q between sites %q and %q", key, owner, site.Slug)
			}
			owners[key] = site.Slug
			routes[key] = backend
		}
	}
	return routes, nil
}

// reload re-reads the registry if sites.toml changed since last load.
func (s *Server) reload() {
	fi, err := os.Stat(config.RegistryPath())
	if err == nil {
		s.mu.RLock()
		unchanged := fi.ModTime().Equal(s.regMtime)
		s.mu.RUnlock()
		if unchanged {
			return
		}
	}
	reg, err := config.LoadRegistry()
	if err != nil {
		// Keep serving stale routes, but say so — once per broken file version.
		s.mu.Lock()
		if fi == nil || !fi.ModTime().Equal(s.lastErrMtime) {
			log.Printf("proxy: reload of %s failed, keeping %d stale route(s): %v",
				config.RegistryPath(), len(s.routes), err)
			if fi != nil {
				s.lastErrMtime = fi.ModTime()
			}
		}
		s.mu.Unlock()
		return
	}
	routes, err := buildRouteTable(s.settings, reg.Sites)
	if err != nil {
		s.mu.Lock()
		if fi == nil || !fi.ModTime().Equal(s.lastErrMtime) {
			log.Printf("proxy: reload of %s failed, keeping %d stale route(s): %v",
				config.RegistryPath(), len(s.routes), err)
			if fi != nil {
				s.lastErrMtime = fi.ModTime()
			}
		}
		s.mu.Unlock()
		return
	}
	sites := append([]config.Site(nil), reg.Sites...)
	sort.Slice(sites, func(i, j int) bool { return sites[i].Slug < sites[j].Slug })

	s.mu.Lock()
	s.routes = routes
	s.sites = sites
	if fi != nil {
		s.regMtime = fi.ModTime()
	}
	s.mu.Unlock()
}

// normalizeRequestHostname turns an inbound Host header into a route-table
// key. It accepts a host with an optional valid port, folds case, and removes
// one terminal root-label dot. It intentionally has no suffix fallback.
func normalizeRequestHostname(host string) string {
	if h, port, err := net.SplitHostPort(host); err == nil {
		if _, err := strconv.Atoi(port); err != nil {
			return ""
		}
		host = h
	} else if strings.Contains(host, ":") {
		return ""
	}
	host = strings.ToLower(host)
	host = strings.TrimSuffix(host, ".")
	if host == "" {
		return ""
	}
	return host
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reload()

	host := normalizeRequestHostname(r.Host)
	if host == "" {
		s.landing(w, r)
		return
	}
	s.mu.RLock()
	rp, ok := s.routes[host]
	s.mu.RUnlock()
	if !ok {
		s.landing(w, r)
		return
	}
	rp.ServeHTTP(w, r)
}

// landing renders an index of all sites with clickable primary-hostname links.
func (s *Server) landing(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	sites := s.sites
	s.mu.RUnlock()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var b strings.Builder
	b.WriteString(`<!doctype html><html><head><meta charset="utf-8"><title>servd</title>`)
	b.WriteString(`<style>body{font:16px/1.5 system-ui,sans-serif;max-width:640px;margin:3rem auto;padding:0 1rem;color:#222}`)
	b.WriteString(`h1{font-size:1.4rem}a{color:#0a58ca;text-decoration:none}a:hover{text-decoration:underline}`)
	b.WriteString(`li{margin:.4rem 0}.port{color:#888;font-size:.85em}.kind{color:#aaa;font-size:.8em}</style></head><body>`)
	b.WriteString(`<h1>servd &mdash; local sites</h1>`)
	if len(sites) == 0 {
		b.WriteString(`<p>No sites registered yet. Run <code>servd add /path/to/project</code>.</p>`)
	} else {
		b.WriteString("<ul>")
		for _, site := range sites {
			u := s.settings.SiteURL(site)
			b.WriteString("<li><a href=\"" + html.EscapeString(u) + "\">" + html.EscapeString(site.Slug) + "</a> ")
			b.WriteString(fmt.Sprintf(`<span class="port">:%d</span> `, site.Port))
			if site.Launcher != "" {
				b.WriteString(`<span class="kind">` + html.EscapeString(site.Launcher) + `</span>`)
			}
			b.WriteString("</li>")
		}
		b.WriteString("</ul>")
	}
	b.WriteString("</body></html>")
	_, _ = w.Write([]byte(b.String()))
}

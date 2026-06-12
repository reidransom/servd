// Package proxy is a host-routing reverse proxy for nip.io subdomains.
//
// A request to http://<slug>.127.0.0.1.nip.io:8080 is routed to the site's
// backend port on the bind host. The registry is reloaded whenever sites.toml
// changes on disk, so newly added sites route without a restart. Unknown or
// bare hosts get a landing page listing every site with live links.
//
// httputil.ReverseProxy transparently handles HTTP/1.1 Upgrade (websockets),
// which jigyll live-reload and Vite/Next HMR depend on.
package proxy

import (
	"fmt"
	"html"
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

	mu       sync.RWMutex
	routes   map[string]int // slug -> backend port
	sites    []config.Site  // for the landing page (sorted by slug)
	regMtime time.Time
}

// New builds a proxy server and loads the initial routing table.
func New(settings config.Settings) *Server {
	s := &Server{settings: settings, routes: map[string]int{}}
	s.reload()
	return s
}

// ListenAndServe starts the proxy on the configured proxy port.
func (s *Server) ListenAndServe() error {
	addr := net.JoinHostPort(s.settings.BindHost, strconv.Itoa(s.settings.ProxyPort))
	srv := &http.Server{Addr: addr, Handler: s}
	return srv.ListenAndServe()
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
		return
	}
	routes := make(map[string]int, len(reg.Sites))
	for _, site := range reg.Sites {
		routes[site.Slug] = site.Port
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

// slugFromHost extracts the leading subdomain label of a Host that ends with
// the configured domain suffix. Returns "" if the host doesn't match.
func (s *Server) slugFromHost(host string) string {
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	host = strings.ToLower(host)
	suffix := "." + s.settings.DomainSuffix
	if !strings.HasSuffix(host, suffix) {
		return ""
	}
	label := strings.TrimSuffix(host, suffix)
	// Only the left-most label is the slug.
	if i := strings.IndexByte(label, '.'); i >= 0 {
		label = label[:i]
	}
	return label
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.reload()

	slug := s.slugFromHost(r.Host)
	if slug == "" {
		s.landing(w, r)
		return
	}
	s.mu.RLock()
	port, ok := s.routes[slug]
	s.mu.RUnlock()
	if !ok {
		s.landing(w, r)
		return
	}

	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(s.settings.BindHost, strconv.Itoa(port))}
	rp := httputil.NewSingleHostReverseProxy(target)
	rp.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "servd: %q is registered but not responding on port %d.\n", slug, port)
		fmt.Fprintf(w, "Start it with: servd up %s\n\n(%v)\n", slug, err)
	}
	rp.ServeHTTP(w, r)
}

// landing renders an index of all sites with clickable nip.io links.
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
		b.WriteString(`<p>No sites registered yet. Run <code>servd scan</code>.</p>`)
	} else {
		b.WriteString("<ul>")
		for _, site := range sites {
			u := fmt.Sprintf("http://%s.%s:%d/", site.Slug, s.settings.DomainSuffix, s.settings.ProxyPort)
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

// Package proxy is a host-routing reverse proxy for nip.io subdomains.
//
// A request to http://<slug>.127.0.0.1.nip.io:8080 is routed to the site's
// backend port on the bind host. The registry is reloaded whenever sites.toml
// changes on disk, so newly added sites route without a restart. Unknown or
// bare hosts get a landing page listing every site with live links.
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
	proxies      map[string]*httputil.ReverseProxy // slug -> backend proxy
	sites        []config.Site                     // for the landing page (sorted by slug)
	regMtime     time.Time
	lastErrMtime time.Time // mtime of the last sites.toml version we logged a reload error for
}

// New builds a proxy server and loads the initial routing table.
func New(settings config.Settings) *Server {
	s := &Server{settings: settings, proxies: map[string]*httputil.ReverseProxy{}}
	s.reload()
	return s
}

// ListenAndServe starts the proxy on the configured proxy port.
func (s *Server) ListenAndServe() error {
	addr := net.JoinHostPort(s.settings.BindHost, strconv.Itoa(s.settings.ProxyPort))
	srv := &http.Server{Addr: addr, Handler: s}
	return srv.ListenAndServe()
}

// buildProxy creates the reverse proxy for one site.
func (s *Server) buildProxy(site config.Site) *httputil.ReverseProxy {
	target := &url.URL{Scheme: "http", Host: net.JoinHostPort(s.settings.BindHost, strconv.Itoa(site.Port))}
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
				config.RegistryPath(), len(s.proxies), err)
			if fi != nil {
				s.lastErrMtime = fi.ModTime()
			}
		}
		s.mu.Unlock()
		return
	}
	proxies := make(map[string]*httputil.ReverseProxy, len(reg.Sites))
	for _, site := range reg.Sites {
		proxies[site.Slug] = s.buildProxy(site)
	}
	sites := append([]config.Site(nil), reg.Sites...)
	sort.Slice(sites, func(i, j int) bool { return sites[i].Slug < sites[j].Slug })

	s.mu.Lock()
	s.proxies = proxies
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
	rp, ok := s.proxies[slug]
	s.mu.RUnlock()
	if !ok {
		s.landing(w, r)
		return
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

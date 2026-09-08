package proxy

import (
	"bufio"
	"context"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/reidransom/servd/internal/config"
)

func proxySettings() config.Settings {
	settings := config.DefaultSettings()
	settings.Hostnames.HTTPPort = 48080
	return settings
}

func TestNormalizeRequestHostname(t *testing.T) {
	cases := []struct{ host, want string }{
		{"acme.localhost:8080", "acme.localhost"},
		{"ACME.LOCALHOST.", "acme.localhost"},
		{"", ""},
		{"acme.localhost:bad", ""},
		{"[::1]:8080", "::1"},
	}
	for _, tc := range cases {
		if got := normalizeRequestHostname(tc.host); got != tc.want {
			t.Errorf("normalizeRequestHostname(%q) = %q, want %q", tc.host, got, tc.want)
		}
	}
}

func TestBuildRouteTableUsesExactGeneratedHostnames(t *testing.T) {
	settings := proxySettings()
	site := config.Site{Slug: "acme", HostPrefix: "auth", Port: 4001}
	routes, err := buildRouteTable(settings, []config.Site{site})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 || routes["auth.acme.localhost"] == nil {
		t.Fatalf("default routes = %v, want only auth.acme.localhost", routes)
	}
	for _, host := range []string{"auth.acme.127.0.0.1.nip.io", "acme.localhost", "acme.extra.localhost", "auth.acme.extra.localhost"} {
		if routes[host] != nil {
			t.Errorf("unexpected alias route %q", host)
		}
	}
}

func TestBuildRouteTableDeduplicatesExplicitFallbacks(t *testing.T) {
	settings := proxySettings()
	settings.Hostnames.TLDsFallback = []string{"127.0.0.1.nip.io", "dev.example.com", "localhost", "dev.example.com"}
	site := config.Site{Slug: "acme", HostPrefix: "auth", Port: 4001}
	routes, err := buildRouteTable(settings, []config.Site{site})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"auth.acme.localhost", "auth.acme.127.0.0.1.nip.io", "auth.acme.dev.example.com"}
	if len(routes) != len(want) {
		t.Fatalf("route count = %d, want %d", len(routes), len(want))
	}
	for _, host := range want {
		if routes[host] == nil {
			t.Errorf("missing generated route %q", host)
		}
	}
	if routes[want[0]] != routes[want[1]] || routes[want[0]] != routes[want[2]] {
		t.Fatal("primary and fallback routes do not share one backend proxy")
	}
}

func TestBuildRouteTableSupportsCustomTLDAndDetectsCollisions(t *testing.T) {
	settings := proxySettings()
	settings.Hostnames.TLD = "dev.example.com"
	routes, err := buildRouteTable(settings, []config.Site{{Slug: "acme", Port: 4001}})
	if err != nil {
		t.Fatal(err)
	}
	if routes["acme.dev.example.com"] == nil {
		t.Fatal("custom-TLD route missing")
	}
	_, err = buildRouteTable(settings, []config.Site{{Slug: "acme", Port: 4001}, {Slug: "acme", Port: 4002}})
	if err == nil || !strings.Contains(err.Error(), "hostname collision") {
		t.Fatalf("collision error = %v", err)
	}

	settings.Hostnames.TLD = "acme.localhost"
	settings.Hostnames.TLDsFallback = []string{"localhost"}
	_, err = buildRouteTable(settings, []config.Site{{Slug: "auth", Port: 4001}, {Slug: "acme", HostPrefix: "auth", Port: 4002}})
	if err == nil || !strings.Contains(err.Error(), "hostname collision") {
		t.Fatalf("primary/fallback collision error = %v", err)
	}
}

func TestBuildRouteTableUsesLocalHostnamesInLANMode(t *testing.T) {
	settings := proxySettings()
	settings.Hostnames.LAN = true
	settings.Hostnames.TLDsFallback = []string{"127.0.0.1.nip.io", "dev.example.com", "local"}
	routes, err := buildRouteTable(settings, []config.Site{{Slug: "acme", Port: 4001}})
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 3 || routes["acme.local"] == nil {
		t.Fatalf("LAN routes = %v, want primary and two fallbacks", routes)
	}
	if routes["acme.localhost"] != nil {
		t.Fatal("non-LAN hostname was routed in LAN mode")
	}
	for _, host := range []string{"acme.127.0.0.1.nip.io", "acme.dev.example.com"} {
		if routes[host] != routes["acme.local"] {
			t.Fatalf("LAN fallback %q does not share primary backend", host)
		}
	}
}

func TestLANPublishesOnlyPrimaryHostnames(t *testing.T) {
	var binary string
	switch runtime.GOOS {
	case "darwin":
		binary = "dns-sd"
	case "linux":
		binary = "avahi-publish-address"
	default:
		t.Skip("mDNS publishing is unsupported on this platform")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, binary), []byte("#!/bin/sh\nexec /bin/sleep 30\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	settings := proxySettings()
	settings.Hostnames.TLD = "dev.example.com"
	settings.Hostnames.LAN = true
	settings.Hostnames.LANIP = "192.168.1.23"
	settings.Hostnames.TLDsFallback = []string{"localhost", "127.0.0.1.nip.io", "local"}
	server := &Server{
		settings: settings,
		sites: []config.Site{
			{Slug: "acme", Port: 4001},
			{Slug: "acme", HostPrefix: "auth", Port: 4002},
		},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if err := server.startLAN(ctx); err != nil {
		t.Fatal(err)
	}
	defer server.stopLAN()
	if got, want := server.publisher.Published(), []string{"acme.local", "auth.acme.local"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("published = %v, want %v", got, want)
	}
	if err := server.reconcileMDNS(server.sites[1:]); err != nil {
		t.Fatal(err)
	}
	if got, want := server.publisher.Published(), []string{"auth.acme.local"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("published after removing site = %v, want %v", got, want)
	}
}

func TestServerRoutesExactHostsAndKeepsForwardedHeaders(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(r.Host + "|" + r.Header.Get("X-Forwarded-Host") + "|" + r.Header.Get("X-Forwarded-Proto")))
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(backendURL.Port())
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name      string
		fallbacks []string
		hosts     []string
		unknown   []string
	}{
		{
			name:    "default",
			hosts:   []string{"ACME.LOCALHOST.:48080", "auth.acme.localhost"},
			unknown: []string{"acme.127.0.0.1.nip.io", "auth.acme.127.0.0.1.nip.io"},
		},
		{
			name:      "explicit fallbacks",
			fallbacks: []string{"127.0.0.1.nip.io", "dev.example.com"},
			hosts: []string{
				"ACME.LOCALHOST.:48080", "acme.127.0.0.1.nip.io:48080", "acme.dev.example.com",
				"auth.acme.localhost", "auth.acme.127.0.0.1.nip.io", "auth.acme.dev.example.com",
			},
			unknown: []string{"acme.extra.localhost", "auth.acme.extra.localhost", "acme.unconfigured.example.com"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("XDG_CONFIG_HOME", t.TempDir())
			settings := proxySettings()
			settings.Hostnames.TLDsFallback = tc.fallbacks
			reg := &config.Registry{Sites: []config.Site{
				{Slug: "acme", Port: port},
				{Slug: "acme", HostPrefix: "auth", Port: port},
			}}
			if err := reg.Save(); err != nil {
				t.Fatal(err)
			}
			server := New(settings)

			for _, host := range tc.hosts {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
				req.Host = host
				server.ServeHTTP(rec, req)
				if got := rec.Body.String(); got != "127.0.0.1:"+strconv.Itoa(port)+"|"+host+"|http" {
					t.Fatalf("route %q body = %q", host, got)
				}
			}

			for _, host := range tc.unknown {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "http://"+host+"/", nil)
				req.Host = host
				server.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `href="http://acme.localhost:48080/"`) {
					t.Fatalf("unknown host %q did not reach landing page: %s", host, rec.Body.String())
				}
			}
		})
	}
}

func TestServerRoutesWebSocketUpgradesByHostname(t *testing.T) {
	newBackend := func(label string) (*httptest.Server, int) {
		backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path != "/livereload" {
				http.NotFound(w, r)
				return
			}
			if !strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				http.Error(w, "missing websocket upgrade", http.StatusBadRequest)
				return
			}

			connection, readWriter, err := http.NewResponseController(w).Hijack()
			if err != nil {
				t.Errorf("%s backend hijack: %v", label, err)
				return
			}
			defer func() {
				_ = connection.Close()
			}()

			acceptHash := sha1.Sum([]byte(r.Header.Get("Sec-WebSocket-Key") + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
			accept := base64.StdEncoding.EncodeToString(acceptHash[:])
			if _, err := fmt.Fprintf(readWriter, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: %s\r\n\r\n", accept); err != nil {
				t.Errorf("%s backend response: %v", label, err)
				return
			}
			if _, err := readWriter.Write([]byte{0x81, byte(len(label))}); err != nil {
				t.Errorf("%s backend frame header: %v", label, err)
				return
			}
			if _, err := readWriter.WriteString(label); err != nil {
				t.Errorf("%s backend frame: %v", label, err)
				return
			}
			if err := readWriter.Flush(); err != nil {
				t.Errorf("%s backend flush: %v", label, err)
			}
		}))
		backendURL, err := url.Parse(backend.URL)
		if err != nil {
			t.Fatal(err)
		}
		port, err := strconv.Atoi(backendURL.Port())
		if err != nil {
			t.Fatal(err)
		}
		return backend, port
	}

	firstBackend, firstPort := newBackend("first")
	defer firstBackend.Close()
	secondBackend, secondPort := newBackend("second")
	defer secondBackend.Close()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings := proxySettings()
	settings.Hostnames.TLDsFallback = []string{"127.0.0.1.nip.io", "dev.example.com"}
	registry := &config.Registry{Sites: []config.Site{
		{Slug: "first", Port: firstPort},
		{Slug: "second", Port: secondPort},
	}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}
	proxyServer := httptest.NewServer(New(settings))
	defer proxyServer.Close()

	const websocketKey = "dGhlIHNhbXBsZSBub25jZQ=="
	for host, want := range map[string]string{
		"first.localhost":         "first",
		"first.127.0.0.1.nip.io":  "first",
		"first.dev.example.com":   "first",
		"second.localhost":        "second",
		"second.127.0.0.1.nip.io": "second",
		"second.dev.example.com":  "second",
	} {
		connection, err := net.DialTimeout("tcp", proxyServer.Listener.Addr().String(), time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if err := connection.SetDeadline(time.Now().Add(time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, err := fmt.Fprintf(connection, "GET /livereload HTTP/1.1\r\nHost: %s\r\nConnection: Upgrade\r\nUpgrade: websocket\r\nSec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", host, websocketKey); err != nil {
			t.Fatal(err)
		}

		reader := bufio.NewReader(connection)
		request, err := http.NewRequest(http.MethodGet, "http://"+host+"/livereload", nil)
		if err != nil {
			t.Fatal(err)
		}
		response, err := http.ReadResponse(reader, request)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != http.StatusSwitchingProtocols {
			t.Fatalf("%s status = %s", host, response.Status)
		}

		frameHeader := make([]byte, 2)
		if _, err := io.ReadFull(reader, frameHeader); err != nil {
			t.Fatal(err)
		}
		if frameHeader[0] != 0x81 || int(frameHeader[1]) != len(want) {
			t.Fatalf("%s frame header = %#v", host, frameHeader)
		}
		payload := make([]byte, len(want))
		if _, err := io.ReadFull(reader, payload); err != nil {
			t.Fatal(err)
		}
		if got := string(payload); got != want {
			t.Fatalf("%s reached backend %q, want %q", host, got, want)
		}
		if err := connection.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestServeUsesProvidedListener(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("served through inherited listener"))
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	backendPort, err := strconv.Atoi(backendURL.Port())
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings := proxySettings()
	if err := (&config.Registry{Sites: []config.Site{{Slug: "acme", Port: backendPort}}}).Save(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(settings)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	request, err := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Host = "acme.localhost"
	response, err := (&http.Client{Timeout: time.Second}).Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = response.Body.Close() }()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if got := string(body); got != "served through inherited listener" {
		t.Fatalf("response = %q", got)
	}

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve returned %v", err)
	}
}

func TestServeReloadsRegistryWithoutLAN(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("beta route"))
	}))
	defer backend.Close()
	backendURL, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	backendPort, err := strconv.Atoi(backendURL.Port())
	if err != nil {
		t.Fatal(err)
	}

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings := proxySettings()
	registry := &config.Registry{Sites: []config.Site{{Slug: "alpha", Port: 4001}}}
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := New(settings)
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()

	time.Sleep(20 * time.Millisecond)
	registry.Sites = append(registry.Sites, config.Site{Slug: "beta", Port: backendPort})
	if err := registry.Save(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		request, err := http.NewRequest(http.MethodGet, "http://"+listener.Addr().String()+"/", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Host = "beta.localhost"
		response, err := (&http.Client{Timeout: time.Second}).Do(request)
		if err != nil {
			t.Fatal(err)
		}
		body, readErr := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if readErr != nil {
			t.Fatal(readErr)
		}
		if string(body) == "beta route" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("new route did not load; last response = %q", body)
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	if err := <-done; err != nil {
		t.Fatalf("Serve returned %v", err)
	}
}

func TestReloadKeepsLastValidRoutesAfterCollision(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	settings := proxySettings()
	reg := &config.Registry{Sites: []config.Site{{Slug: "acme", Port: 4001}}}
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	server := New(settings)
	initial := server.routes["acme.localhost"]
	if initial == nil {
		t.Fatal("initial route missing")
	}

	time.Sleep(10 * time.Millisecond)
	reg.Sites = append(reg.Sites, config.Site{Slug: "acme", Port: 4002})
	if err := reg.Save(); err != nil {
		t.Fatal(err)
	}
	server.reload()
	if got := server.routes["acme.localhost"]; got != initial {
		t.Fatal("failed registry reload replaced the last valid route table")
	}
}

func TestLandingRendersSiteLink(t *testing.T) {
	settings := proxySettings()
	settings.Hostnames.TLDsFallback = []string{"127.0.0.1.nip.io", "dev.example.com"}
	s := &Server{
		settings: settings,
		sites:    []config.Site{{Slug: "acme", HostPrefix: "auth", Port: 4001}},
	}
	rec := httptest.NewRecorder()
	s.landing(rec, httptest.NewRequest("GET", "http://localhost/", nil))
	body := rec.Body.String()
	if !strings.Contains(body, `href="http://auth.acme.localhost:48080/"`) {
		t.Error("expected primary site link in landing page")
	}
	for _, suffix := range settings.Hostnames.TLDsFallback {
		if strings.Contains(body, suffix) {
			t.Errorf("landing page contains fallback suffix %q: %s", suffix, body)
		}
	}
}

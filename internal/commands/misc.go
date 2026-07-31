package commands

import (
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/launcher"
	"github.com/reidransom/servd/internal/netcheck"
	"github.com/reidransom/servd/internal/tui"
	"github.com/spf13/cobra"
)

func newTUICmd() *cobra.Command {
	return &cobra.Command{
		Use:   "tui",
		Short: "Launch the interactive dashboard",
		RunE: func(cmd *cobra.Command, args []string) error {
			return tui.Run()
		},
	}
}

// newLaunchersCmd prints the effective launcher rules — the user's
// launchers.toml entries followed by the surviving built-ins — in the same
// TOML format, so any rule can be copied into launchers.toml and tweaked.
func newLaunchersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "launchers",
		Short: "Print the effective launcher rules (launchers.toml + built-ins)",
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := launcher.MarshalRules(launcher.EffectiveRules())
			if err != nil {
				return err
			}
			fmt.Print(string(out))
			return nil
		},
	}
}

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check tools, ports and hostname resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := app.Load()
			if err != nil {
				return err
			}
			ok := true

			fmt.Println("Launcher tools:")
			for _, bin := range launcher.Tools(launcher.EffectiveRules()) {
				if p, err := exec.LookPath(bin); err == nil {
					fmt.Printf("  ✓ %-7s %s\n", bin, p)
				} else {
					fmt.Printf("  · %-7s (not installed)\n", bin)
				}
			}

			fmt.Println("Proxy port:")
			if netcheck.PortFree(settings.BindHost, settings.Hostnames.HTTPPort) {
				fmt.Printf("  ✓ :%d is free\n", settings.Hostnames.HTTPPort)
			} else {
				fmt.Printf("  i :%d is in use (proxy may already be running)\n", settings.Hostnames.HTTPPort)
			}

			fmt.Println("Assigned ports:")
			conflicts := 0
			for _, s := range reg.Sites {
				if !netcheck.PortFree(settings.BindHost, s.Port) {
					conflicts++
				}
			}
			fmt.Printf("  %d site(s) registered, %d port(s) currently bound\n", len(reg.Sites), conflicts)

			if settings.Hostnames.NipIO {
				fmt.Println("nip.io resolution:")
				host := "test." + settings.Hostnames.NipIOSuffix
				if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
					fmt.Printf("  ✓ %s -> %v\n", host, addrs)
				} else {
					ok = false
					fmt.Printf("  ✗ could not resolve %s (check internet/DNS): %v\n", host, err)
				}
			}
			fmt.Printf("Primary hostname routing is configured for %v; verify DNS or hosts setup for custom TLDs.\n", settings.Hostnames.TLDs)

			if ok {
				fmt.Println("\nAll essential checks passed.")
			} else {
				fmt.Println("\nSome checks failed (see ✗ above).")
			}
			return nil
		},
	}
}

// newStaticCmd is the hidden built-in static file server used by the "static"
// launcher fallback. It serves --dir on --host:--port, refusing dot-prefixed
// paths (.env, .git, …) so a project dir doesn't leak secrets.
func newStaticCmd() *cobra.Command {
	var host, dir string
	var port int
	c := &cobra.Command{
		Use:    "__static",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			addr := net.JoinHostPort(host, strconv.Itoa(port))
			srv := &http.Server{
				Addr:         addr,
				Handler:      http.FileServer(dotHidingFS{http.Dir(dir)}),
				ReadTimeout:  30 * time.Second,
				WriteTimeout: 30 * time.Second,
			}
			return srv.ListenAndServe()
		},
	}
	c.Flags().StringVar(&host, "host", "127.0.0.1", "bind host")
	c.Flags().IntVar(&port, "port", 0, "listen port")
	c.Flags().StringVar(&dir, "dir", ".", "directory to serve")
	return c
}

// dotHidingFS wraps an http.FileSystem and hides dot-prefixed files and
// directories from both direct requests and directory listings.
type dotHidingFS struct{ fs http.FileSystem }

// containsDotSegment reports whether any path segment starts with a dot.
// http.FileServer cleans the URL path before calling Open, and the
// http.FileSystem API always uses forward slashes.
func containsDotSegment(name string) bool {
	for _, part := range strings.Split(name, "/") {
		if strings.HasPrefix(part, ".") {
			return true
		}
	}
	return false
}

func (d dotHidingFS) Open(name string) (http.File, error) {
	if containsDotSegment(name) {
		return nil, fs.ErrPermission // FileServer renders 403
	}
	f, err := d.fs.Open(name)
	if err != nil {
		return nil, err
	}
	return dotHidingFile{f}, nil
}

// dotHidingFile filters dotfiles out of directory listings.
type dotHidingFile struct{ http.File }

func (f dotHidingFile) Readdir(count int) ([]fs.FileInfo, error) {
	entries, err := f.File.Readdir(count)
	out := entries[:0]
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e)
		}
	}
	return out, err
}

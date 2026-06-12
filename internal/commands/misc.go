package commands

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"time"

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

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check tools, ports and nip.io resolution",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := load()
			if err != nil {
				return err
			}
			ok := true

			fmt.Println("Detector tools:")
			for _, bin := range []string{"jigyll", "jekyll", "hugo", "node", "npm", "just", "make"} {
				if p, err := exec.LookPath(bin); err == nil {
					fmt.Printf("  ✓ %-7s %s\n", bin, p)
				} else {
					fmt.Printf("  · %-7s (not installed)\n", bin)
				}
			}

			fmt.Println("Proxy port:")
			if portFree(settings.BindHost, settings.ProxyPort) {
				fmt.Printf("  ✓ :%d is free\n", settings.ProxyPort)
			} else {
				fmt.Printf("  i :%d is in use (proxy may already be running)\n", settings.ProxyPort)
			}

			fmt.Println("Assigned ports:")
			conflicts := 0
			for _, s := range reg.Sites {
				if !portFree(settings.BindHost, s.Port) {
					conflicts++
				}
			}
			fmt.Printf("  %d site(s) registered, %d port(s) currently bound\n", len(reg.Sites), conflicts)

			fmt.Println("nip.io resolution:")
			host := "test." + settings.DomainSuffix
			if addrs, err := net.LookupHost(host); err == nil && len(addrs) > 0 {
				fmt.Printf("  ✓ %s -> %v\n", host, addrs)
			} else {
				ok = false
				fmt.Printf("  ✗ could not resolve %s (check internet/DNS): %v\n", host, err)
			}

			if ok {
				fmt.Println("\nAll essential checks passed.")
			} else {
				fmt.Println("\nSome checks failed (see ✗ above).")
			}
			return nil
		},
	}
}

func portFree(host string, port int) bool {
	ln, err := net.Listen("tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		return false
	}
	ln.Close()
	return true
}

// newStaticCmd is the hidden built-in static file server used by the "static"
// launcher fallback. It serves --dir on --host:--port.
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
				Handler:      http.FileServer(http.Dir(dir)),
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

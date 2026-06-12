package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "up [slug...]",
		Short: "Start one or more sites (use --all for every site)",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, st, err := load()
			if err != nil {
				return err
			}
			sites, err := selectSites(reg, args, all, true)
			if err != nil {
				return err
			}
			if all && len(sites) == 0 {
				fmt.Println("No enabled sites to start (all are disabled).")
				return nil
			}
			for _, s := range sites {
				if err := supervisor.Start(s, settings, st); err != nil {
					fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					continue
				}
				fmt.Printf("  %-20s started on :%d\n", s.Slug, s.Port)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "start every registered site")
	return c
}

func newDownCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "down [slug...]",
		Short: "Stop one or more sites (use --all for every site)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg, st, err := load()
			if err != nil {
				return err
			}
			// down --all stops every site (including disabled-but-running ones).
			sites, err := selectSites(reg, args, all, false)
			if err != nil {
				return err
			}
			for _, s := range sites {
				if err := supervisor.Stop(s.Slug, st); err != nil {
					fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					continue
				}
				fmt.Printf("  %-20s stopped\n", s.Slug)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "stop every registered site")
	return c
}

func newRestartCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "restart [slug...]",
		Short: "Restart one or more sites",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, st, err := load()
			if err != nil {
				return err
			}
			sites, err := selectSites(reg, args, all, true)
			if err != nil {
				return err
			}
			for _, s := range sites {
				if err := supervisor.Restart(s, settings, st); err != nil {
					fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					continue
				}
				fmt.Printf("  %-20s restarted on :%d\n", s.Slug, s.Port)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "restart every registered site")
	return c
}

func newLogsCmd() *cobra.Command {
	var follow bool
	c := &cobra.Command{
		Use:   "logs <slug>",
		Short: "Show a site's server output",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg, _, err := load()
			if err != nil {
				return err
			}
			if reg.Find(args[0]) == nil {
				return fmt.Errorf("unknown site %q", args[0])
			}
			path := supervisor.LogPath(args[0])
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("no logs yet for %q", args[0])
			}
			defer f.Close()
			if _, err := io.Copy(os.Stdout, f); err != nil {
				return err
			}
			if !follow {
				return nil
			}
			// Tail: keep reading appended bytes.
			r := bufio.NewReader(f)
			for {
				line, err := r.ReadString('\n')
				if len(line) > 0 {
					fmt.Print(line)
				}
				if err == io.EOF {
					time.Sleep(300 * time.Millisecond)
					continue
				}
				if err != nil {
					return err
				}
			}
		},
	}
	c.Flags().BoolVarP(&follow, "follow", "f", false, "follow the log (like tail -f)")
	return c
}

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open <slug>",
		Short: "Open a site's nip.io URL in the browser",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := load()
			if err != nil {
				return err
			}
			s := reg.Find(args[0])
			if s == nil {
				return fmt.Errorf("unknown site %q", args[0])
			}
			url := siteURL(*s, settings)
			fmt.Println(url)
			return openBrowser(url)
		},
	}
}

// openBrowser opens a URL in the default browser (best effort).
func openBrowser(url string) error {
	var bin string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		bin, args = "open", []string{url}
	case "windows":
		bin, args = "cmd", []string{"/c", "start", url}
	default:
		bin, args = "xdg-open", []string{url}
	}
	return exec.Command(bin, args...).Start()
}

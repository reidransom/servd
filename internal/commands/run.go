package commands

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/reidransom/servd/internal/app"
	"github.com/reidransom/servd/internal/config"
	"github.com/reidransom/servd/internal/proxy"
	"github.com/reidransom/servd/internal/state"
	"github.com/reidransom/servd/internal/supervisor"
	"github.com/spf13/cobra"
)

func newUpCmd() *cobra.Command {
	var all, wait, jsonOut bool
	var timeout time.Duration
	c := &cobra.Command{
		Use:   "up [slug|path...]",
		Short: "Start one or more sites (use --all for every site)",
		Long:  "Start one or more sites by slug or registered root directory path. Slugs take precedence over directory names. Without targets, use the registered current directory if it contains .servd.toml. Use --all for every site.",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, reg, _, err := app.Load()
			if err != nil {
				return err
			}
			sites, err := selectSites(reg, args, all)
			if err != nil {
				return err
			}
			if all && len(sites) == 0 && !jsonOut {
				fmt.Println("No sites registered.")
				return nil
			}
			type upResult struct {
				siteInfo
				Error string `json:"error,omitempty"`
			}
			results := make([]upResult, 0, len(sites))
			failed := 0
			for _, s := range sites {
				err := supervisor.Start(s, settings)
				if err == nil && wait {
					err = supervisor.WaitReady(s, settings, timeout)
				}
				if jsonOut {
					// Reload state so the result carries the fresh pid/status.
					st, lerr := state.Load()
					if lerr != nil {
						return lerr
					}
					r := upResult{siteInfo: newSiteInfo(settings, s, st)}
					if err != nil {
						r.Error = err.Error()
					}
					results = append(results, r)
				}
				switch {
				case err != nil:
					failed++
					if !jsonOut {
						fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					}
				case jsonOut:
				case wait:
					fmt.Printf("  %-20s ready on :%d\n", s.Slug, s.Port)
				default:
					fmt.Printf("  %-20s started on :%d\n", s.Slug, s.Port)
				}
			}
			if jsonOut {
				if err := printJSON(results); err != nil {
					return err
				}
			}
			if failed > 0 {
				return fmt.Errorf("%d site(s) failed to start", failed)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&all, "all", false, "start every registered site")
	c.Flags().BoolVar(&wait, "wait", false, "block until each server accepts connections (exit non-zero on failure)")
	c.Flags().DurationVar(&timeout, "timeout", 30*time.Second, "per-site readiness timeout (with --wait)")
	c.Flags().BoolVar(&jsonOut, "json", false, "emit machine-readable JSON results")
	return c
}

func newDownCmd() *cobra.Command {
	var all bool
	c := &cobra.Command{
		Use:   "down [slug|path...]",
		Short: "Stop one or more sites (use --all for every site)",
		Long:  "Stop one or more sites by slug or registered root directory path. Slugs take precedence over directory names. Without targets, use the registered current directory if it contains .servd.toml. Use --all for every site.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg, _, err := app.Load()
			if err != nil {
				return err
			}
			sites, err := selectSites(reg, args, all)
			if err != nil {
				return err
			}
			failed := 0
			for _, s := range sites {
				if err := supervisor.Stop(s.Slug); err != nil {
					failed++
					fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					continue
				}
				fmt.Printf("  %-20s stopped\n", s.Slug)
			}
			if failed > 0 {
				return fmt.Errorf("%d site(s) failed to stop", failed)
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
		Use:   "restart [slug|path...]",
		Short: "Restart one or more sites",
		Long:  "Restart one or more sites by slug or registered root directory path. Slugs take precedence over directory names. Without targets, use the registered current directory if it contains .servd.toml. Use --all for every site.",
		RunE: func(cmd *cobra.Command, args []string) error {
			settings, err := config.LoadSettings()
			if err != nil {
				return err
			}
			reg, err := config.LoadRegistry()
			if err != nil {
				return err
			}
			sites, err := selectSites(reg, args, all)
			if err != nil {
				return err
			}
			failed := 0
			for _, s := range sites {
				if err := supervisor.Restart(s, settings); err != nil {
					failed++
					fmt.Printf("  %-20s ERROR: %v\n", s.Slug, err)
					continue
				}
				fmt.Printf("  %-20s restarted on :%d\n", s.Slug, s.Port)
			}
			if failed > 0 {
				return fmt.Errorf("%d site(s) failed to restart", failed)
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
		Use:   "logs [slug|path]",
		Short: "Show a site's server output",
		Long:  "Show a site's server output by slug or registered root directory path. Slugs take precedence over directory names. Without a target, use the registered current directory if it contains .servd.toml.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, reg, _, err := app.Load()
			if err != nil {
				return err
			}
			s, err := selectSite(cmd, reg, args)
			if err != nil {
				return err
			}
			path := supervisor.LogPath(s.Slug)
			f, err := os.Open(path)
			if err != nil {
				return fmt.Errorf("no logs yet for %q", s.Slug)
			}
			defer func() { _ = f.Close() }()
			if _, err := io.Copy(os.Stdout, f); err != nil {
				return err
			}
			if !follow {
				return nil
			}
			// Tail: keep reading appended bytes, reopening if the file is
			// replaced and rewinding if it is truncated in place.
			r := bufio.NewReader(f)
			for {
				line, err := r.ReadString('\n')
				if len(line) > 0 {
					fmt.Print(line)
				}
				if err == nil {
					continue
				}
				if err != io.EOF {
					return err
				}

				// Logical read offset = underlying position minus unread buffer.
				offset, serr := f.Seek(0, io.SeekCurrent)
				if serr == nil {
					offset -= int64(r.Buffered())
				}
				fi, ferr := f.Stat()
				di, derr := os.Stat(path)
				switch {
				case ferr == nil && derr == nil && !os.SameFile(fi, di):
					// Rotated: a new file lives at path; follow it from the top.
					if nf, oerr := os.Open(path); oerr == nil {
						_ = f.Close()
						f = nf
						r = bufio.NewReader(f)
					}
				case serr == nil && derr == nil && di.Size() < offset:
					// Truncated in place: rewind.
					if _, err := f.Seek(0, io.SeekStart); err != nil {
						return err
					}
					r = bufio.NewReader(f)
				}
				time.Sleep(300 * time.Millisecond)
			}
		},
	}
	c.Flags().BoolVarP(&follow, "follow", "f", false, "follow the log (like tail -f)")
	return c
}

func selectedSiteURL(cmd *cobra.Command, args []string) (string, error) {
	settings, reg, st, err := app.Load()
	if err != nil {
		return "", err
	}
	site, err := selectSite(cmd, reg, args)
	if err != nil {
		return "", err
	}
	return proxy.EffectiveSettings(settings, st).SiteURL(*site), nil
}

func newCopyCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "copy [slug|path]",
		Short: "Copy a site's primary URL to the clipboard",
		Long:  "Copy a site's primary URL to the clipboard by slug or registered root directory path. Slugs take precedence over directory names. Without a target, use the registered current directory if it contains .servd.toml.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := selectedSiteURL(cmd, args)
			if err != nil {
				return err
			}
			return app.WriteClipboard(cmd.OutOrStdout(), url)
		},
	}
}

func newOpenCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "open [slug|path]",
		Short: "Open a site's primary URL in the browser",
		Long:  "Open a site's primary URL in the browser by slug or registered root directory path. Slugs take precedence over directory names. Without a target, use the registered current directory if it contains .servd.toml.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			url, err := selectedSiteURL(cmd, args)
			if err != nil {
				return err
			}
			fmt.Println(url)
			return app.OpenBrowser(url)
		},
	}
}

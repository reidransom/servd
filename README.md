# servd

[![CI](https://github.com/reidransom/servd/actions/workflows/ci.yml/badge.svg)](https://github.com/reidransom/servd/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/reidransom/servd)](https://goreportcard.com/report/github.com/reidransom/servd)
[![Go Reference](https://pkg.go.dev/badge/github.com/reidransom/servd.svg)](https://pkg.go.dev/github.com/reidransom/servd)
![Go Version](https://img.shields.io/github/go-mod/go-version/reidransom/servd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run and manage many local dev servers at once.

`servd` discovers your web projects, runs each one's dev server on a stable
port, and reverse-proxies them as friendly [nip.io](https://nip.io) subdomains — so a folder of
client sites becomes `http://acme.127.0.0.1.nip.io:8080/`,
`http://blog.127.0.0.1.nip.io:8080/`, and so on, all managed from one CLI or an
interactive TUI.

It is **not tied to any one generator.**
[jigyll](https://github.com/reidransom/jigyll)/[Jekyll](https://jekyllrb.com)
is just one of several detectors; [Node](https://nodejs.org),
[Hugo](https://gohugo.io), [just](https://github.com/casey/just)/[make](https://www.gnu.org/software/make/)
recipes, plain static dirs, and any project with a `Procfile` work too.

## Install

```sh
go install github.com/reidransom/servd/cmd/servd@latest
```

## Quick start

```sh
servd scan ~/clients      # discover projects, assign ports + slugs
servd enable acme blog    # pick which sites you're working on (see below)
servd up --all            # start every *enabled* dev server
servd proxy up            # start the nip.io reverse proxy on :8080
servd                     # open the interactive dashboard (TUI)
```

By default, **scanned sites are registered disabled** — `up --all` only starts
sites you've explicitly `servd enable`d. This keeps a big folder of clients from
all spinning up at once. To start a single site regardless, name it:
`servd up acme`. To make new sites enabled by default, set
`default_enabled = true` in `config.toml` (or use `servd add --enable`).

Then visit `http://<slug>.127.0.0.1.nip.io:8080/` for any site, or
`http://127.0.0.1:8080/` for a landing page listing them all. nip.io resolves
`*.127.0.0.1.nip.io` to 127.0.0.1 with zero DNS setup.

## How a project is launched

For each site, servd resolves a launch command in this order (first match wins):

1. **Manual override** — a command pinned with `servd add --cmd "…"`.
2. **`Procfile`** (or `Procfile.dev`) — the `web:` process, run with `$PORT`
   and `$HOST` exported ([foreman](https://github.com/ddollar/foreman)/[Heroku
   convention](https://devcenter.heroku.com/articles/procfile)). The universal
   escape hatch.
3. **Auto-detection:**
   | Detector | Trigger | Command |
   |---|---|---|
   | jigyll / Jekyll | `_config.yml` | `jigyll serve -s . -H {host} -P {port} -w` |
   | Hugo | `hugo.toml` / content dir | `hugo serve --bind {host} -p {port}` |
   | Node | `package.json` dev/serve/start script | `npm run <script>` (+`--port` for [vite](https://vite.dev)/[next](https://nextjs.org)/[astro](https://astro.build)) |
   | just | `justfile` with `serve` recipe | `just serve` |
   | make | `Makefile` with `serve` target | `make serve PORT={port}` |
   | static | an `index.html` | built-in file server (no dependency) |

`PORT` and `HOST` are always exported to the child, and `{port}`/`{host}`
placeholders are substituted — so Procfiles and detected commands share one
port-injection mechanism. Run `servd which <slug>` to see exactly what a project
resolves to.

## Commands

| Command | Purpose |
|---|---|
| `servd scan [dir]` | discover servable projects under `dir` (or the configured projects dir) |
| `servd add <path> [--slug] [--port] [--cmd]` | register one project |
| `servd rm <slug>` | stop and unregister a site |
| `servd which <slug>` | show the resolved launch command |
| `servd status` (alias `ls`) | table of every site with live status |
| `servd up [slug…] [--all]` | start sites (`--all` skips disabled ones) |
| `servd down [slug…] [--all]` | stop sites (`--all` stops everything) |
| `servd restart [slug…] [--all]` | restart sites (`--all` skips disabled ones) |
| `servd enable <slug…>` | enable sites so `up --all` starts them |
| `servd disable <slug…>` | disable sites so `up --all` skips them (still startable by name) |
| `servd logs <slug> [-f]` | show / follow a site's server output |
| `servd open <slug>` | open the nip.io URL in a browser |
| `servd proxy up\|down\|status` | manage the background reverse proxy |
| `servd proxy` | run the proxy in the foreground |
| `servd doctor` | check tools, ports and nip.io resolution |
| `servd` / `servd tui` | interactive dashboard |

The dashboard is a split view: the site list on the left, and a live tail of
the highlighted site's log on the right, led by the `$ command` that started
(or would start) it. Moving the selection switches the log panel; `tab` moves
focus to the log so `↑/↓` scroll it (scroll up to pause the tail, scroll back to
the bottom to resume following).

### TUI keys

`↑/↓` move · `tab` focus list/log · `s` start · `x` stop · `r` restart ·
`a` start-all · `X` stop-all · `e` enable/disable · `o` open · `p` toggle proxy ·
`S` rescan · `A` add a site (type a path, `tab` completes) · `q` quit

## Files

- `~/.config/servd/config.toml` — settings (`projects_dir`, `port_range_start`,
  `proxy_port`, `domain_suffix`, `bind_host`, `default_enabled`)
- `~/.config/servd/sites.toml` — the site registry
- `~/.local/state/servd/state.json` — live pids/ports (self-healing)
- `~/.local/state/servd/logs/<slug>.log` — per-site server output

Servers are launched **detached** in their own process groups, so they keep
running after the CLI or TUI exits; `servd down` signals the whole group.
The reverse proxy passes through websocket upgrades, so live-reload / HMR works.

## Development

```sh
go build ./... && go vet ./... && go test ./... -race
```

## Proxy and the Host header

The proxy rewrites the `Host` header to the backend's own address
(e.g. `127.0.0.1:4001`), so dev servers with host allowlists (Vite 5+, Next,
[Rails](https://rubyonrails.org) host authorization) accept proxied requests
out of the box. The original
host is still available to the backend via `X-Forwarded-Host` / `X-Forwarded-Proto`.

If a site builds absolute URLs from `Host` and needs the nip.io address
instead, set `preserve_host = true` on its `[[site]]` entry in `sites.toml` —
and add the nip.io hostname to that dev server's own allowed-hosts setting.

---

[MIT licensed](LICENSE). Maintained by [r2ware](https://r2ware.dev).

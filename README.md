# servd

[![CI](https://github.com/reidransom/servd/actions/workflows/ci.yml/badge.svg)](https://github.com/reidransom/servd/actions/workflows/ci.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/reidransom/servd)](https://goreportcard.com/report/github.com/reidransom/servd)
[![Go Reference](https://pkg.go.dev/badge/github.com/reidransom/servd.svg)](https://pkg.go.dev/github.com/reidransom/servd)
![Go Version](https://img.shields.io/github/go-mod/go-version/reidransom/servd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run and manage many local dev servers at once.

`servd` runs registered web projects on stable backend ports and reverse-proxies
them by exact hostname. A folder of client sites becomes
`http://acme.localhost/`, `http://blog.localhost/`, and so on, all managed from
one CLI or interactive TUI. Optional [nip.io](https://nip.io) hostnames remain
available for compatibility.

It is **not tied to any one generator.**
[jigyll](https://github.com/reidransom/jigyll)/[Jekyll](https://jekyllrb.com)
is just one of several detectors; [Node](https://nodejs.org),
[Hugo](https://gohugo.io), [just](https://github.com/casey/just)/[make](https://www.gnu.org/software/make/)
recipes, plain static dirs, and any project with a `Procfile` work too.

## Install

### Homebrew — macOS and Linux

```sh
brew install reidransom/tap/servd
```

### Scoop — Windows

```powershell
scoop bucket add reidransom https://github.com/reidransom/scoop-bucket
scoop install reidransom/servd
```

### Release archive

Download the archive for your operating system and architecture, plus
`checksums.txt`, from the
[latest GitHub release](https://github.com/reidransom/servd/releases/latest).
Windows archives are ZIP files; macOS and Linux archives are tarballs. Verify
the archive before extracting it:

```sh
# macOS or Linux: compare this hash with the matching checksums.txt entry
shasum -a 256 servd_Darwin_arm64.tar.gz
```

```powershell
# Windows: compare this hash with the matching checksums.txt entry
Get-FileHash .\servd_Windows_x86_64.zip -Algorithm SHA256
```

Each archive contains `servd` (`servd.exe` on Windows), `README.md`, and
`LICENSE`.

### Go install

With the Go version declared in `go.mod` or newer:

```sh
go install github.com/reidransom/servd/cmd/servd@latest
```

Every installation method can report its exact version without reading config
or starting the TUI:

```sh
servd version
```

## Quick start

```sh
servd add ~/clients/acme  # detect the launcher, assign a port + slug
servd up --all            # start every registered dev server
servd proxy up            # start the hostname reverse proxy
servd                     # open the interactive dashboard (TUI)
```

Every entry in `sites.toml` is managed by servd and participates in
`up --all` and `restart --all`. To start a single registered site, name it:
`servd up acme`.

Then visit `http://<slug>.localhost/` for any site, or `http://127.0.0.1/`
for a landing page listing them all. `.localhost` resolves to loopback without
DNS setup.

### Proxy port selection

When `config.toml` is absent, `servd proxy up` first tries `127.0.0.1:80`.
If that requires permission, it asks `sudo` only to bind the listener, then
starts the HTTP proxy as the invoking user. If elevation is declined or port
80 is already occupied, servd falls back to `127.0.0.1:8080` for that run; it
does not write that fallback into configuration. A noninteractive command
attempts passwordless elevation and falls back immediately.

Set an explicit port to make the choice strict:

```toml
[hostnames]
http_port = 8080
```

An explicit privileged port fails when it cannot be acquired; it never silently
falls back. While running, status, site URLs, the TUI, and browser-open actions
use the proxy's recorded runtime port. Route changes in `sites.toml` reload
without restarting the proxy.

## How a project is launched

For each site, servd resolves a launch command in this order (first match wins):

1. **Manual override** — a command pinned at registration, either quoted
   (`servd add . --cmd "…"`) or unquoted after `--`
   (`servd add . -- bundle exec middleman serve -p {port}`).
2. **`.servd.toml`** in the project directory — a one-line
   `cmd = "bundle exec middleman serve -p {port}"` lets any project declare how
   to serve itself, next to its code.
3. **`Procfile`** (or `Procfile.dev`) — the `web:` process, run with `$PORT`
   and `$HOST` exported ([foreman](https://github.com/ddollar/foreman)/[Heroku
   convention](https://devcenter.heroku.com/articles/procfile)). The universal
   escape hatch.
4. **Launcher rules** — declarative match-and-run rules, tried in order. Your
   own rules from `~/.config/servd/launchers.toml` come first, then the
   built-in defaults:
   | Rule | Trigger | Command |
   |---|---|---|
   | jigyll / Jekyll | `_config.yml` | `jigyll serve -s . -H {host} -P {port} -w` |
   | Hugo | `hugo.toml` / content dir | `hugo serve --bind {host} -p {port}` |
   | Node | `package.json` dev/serve/start script | `npm run <script>` (+`--port` for [vite](https://vite.dev)/[next](https://nextjs.org)/[astro](https://astro.build)) |
   | just | `justfile` with `serve` recipe | `just serve` |
   | make | `Makefile` with `serve` target | `make serve PORT={port}` |
   | static | an `index.html` | built-in file server (no dependency) |

`PORT` and `HOST` are always exported to the child, and `{port}`/`{host}`
placeholders are substituted — so Procfiles and rule commands share one
port-injection mechanism. Run `servd which <slug>` to see exactly what a project
resolves to.

### Custom launcher rules

The built-in table above is data, not code — every rule is defined in the same
TOML format you can write yourself. `servd launchers` prints the effective rule
set; copy any rule into `~/.config/servd/launchers.toml` to change it (same
`name` replaces the built-in, `disabled = true` turns it off), or add rules for
tools servd has never heard of:

```toml
[[launcher]]
name = "middleman"
matches = { file = "Gemfile", regex = "middleman" }   # file content regex
bin = "middleman"                                     # required on PATH
cmd = "bundle exec middleman serve -p {port}"

[[launcher]]
name = "static"        # replace the built-in fallback file server
exists = ["index.html"]
bin = "python3"
cmd = "python3 -m http.server -b {host} {port}"
```

User rules are always tried before the built-ins, in file order. A rule matches
a directory when **all** of its predicates hold; list values match **any-of**:

| Predicate | Meaning |
|---|---|
| `exists = ["hugo.*"]` | any file matches (globs ok) |
| `dirs = ["content"]` | any directory matches (globs ok) |
| `bin = "hugo"` | binary is on `PATH` |
| `recipe = { files = ["justfile"], target = "serve" }` | make/just file declares the target |
| `matches = { file = "Gemfile", regex = "…" }` | file content matches the regex |
| `npm_script = ["dev", "serve"]` | `package.json` has one of these scripts |

In `cmd`, `{port}`/`{host}` are substituted at launch, `{script}` is the
matched `npm_script` entry, and `{self}` is the servd binary itself. For dev
servers that ignore `$PORT`, `port_flag = " -- --port {port}"` is appended when
any `port_flag_deps` entry appears in `package.json` dependencies.

## Commands

| Command | Purpose |
|---|---|
| `servd add <path> [--slug] [--port] [--cmd] [-- <command>…]` | register one project |
| `servd rm <slug>` | stop and unregister a site |
| `servd which <slug>` | show the resolved launch command |
| `servd launchers` | print the effective launcher rules (yours + built-ins) |
| `servd status [slug]` (alias `ls`) | table of every site, or one named site, with live status (`--json` for machines) |
| `servd up [slug…] [--all]` | start sites (`--all` starts every registered site; `--wait`/`--json` for scripts) |
| `servd down [slug…] [--all]` | stop sites (`--all` stops every registered site) |
| `servd restart [slug…] [--all]` | restart sites (`--all` restarts every registered site) |
| `servd logs <slug> [-f]` | show / follow a site's server output |
| `servd open <slug>` | open the nip.io URL in a browser |
| `servd proxy up\|down\|status` | manage the background reverse proxy |
| `servd proxy` | run the proxy in the foreground |
| `servd doctor` | check tools, ports and nip.io resolution |
| `servd version` / `servd --version` | report version, commit, and build date |
| `servd` / `servd tui` | interactive dashboard |

The dashboard is a split view: the site list on the left, and a live tail of
the highlighted site's log on the right, led by the `$ command` that started
(or would start) it. Moving the selection switches the log panel; `tab` moves
focus to the log so `↑/↓` scroll it (scroll up to pause the tail, scroll back to
the bottom to resume following).

Site glyphs are `○` stopped, `◐` starting, `●` running, and red `✕` error.
Select an error row to see its concise reason while the site log remains visible.

### TUI keys

`↑/↓` move · `tab` focus list/log · `s` start/stop · `r` restart ·
`S` start/stop all · `a` add a site (type a path, `tab` completes) · `o` open · `p` toggle proxy ·
`h` show/hide this key help · `q` quit

## Agents and scripts

Coding agents (and shell scripts) shouldn't have to parse tables or babysit
long-running processes. Two flags give them a structured interface:

```sh
servd status --json            # everything an agent needs to discover servers
servd status acme --json       # status for one known server
```

`status --json` prints one object: `proxy` (`running`, `accepting`, `pid`,
`port`, `url`) plus a `sites` array (containing only the requested site when a
slug is supplied) where each site carries `slug`, `path`, `port`, `url` (through the proxy), `direct_url` (straight to the dev server),
`launcher`, and `status` (`stopped` | `starting` | `running` | `error`).
Error records also carry a concise `error` reason; live records carry `pid`,
`cmd`, `log`, `started_at`, and `uptime_seconds`. Match your project by `path`
to find its slug, then hit `direct_url` (or `url` if the proxy is accepting).

`up --wait` polls until the server actually accepts connections (default
`--timeout 30s`), and exits non-zero — with the log tail in the error — if the
process dies or never binds. Failed launches remain in `error` until a
successful start or `servd down` clears the runtime attempt. Path and launcher
errors clear automatically when their source is fixed. With `--json`
the per-site results (including any `error`) go to stdout as an array. Servers
are already detached by default, so `up` never needs backgrounding tricks; a
second `up` on a running site is a no-op.

Use these instead of reading `state.json` directly — the file's format is
internal and may change.

## Files

- `~/.config/servd/config.toml` — settings (`port_range_start`, `proxy_port`,
  `domain_suffix`, `bind_host`)
- `~/.config/servd/sites.toml` — the site registry
- `~/.config/servd/launchers.toml` — your launcher rules (optional, see above)
- `<project>/.servd.toml` — per-project launch command (optional)
- `~/.local/state/servd/state.json` — latest supervised runtime attempts
- `~/.local/state/servd/logs/<slug>.log` — per-site server output

Compatibility: legacy `enabled` site keys and `default_enabled` settings keys
are ignored and may be deleted.

Servers are launched **detached** in their own process groups, so they keep
running after the CLI or TUI exits; `servd down` signals the whole group.
The reverse proxy passes through websocket upgrades, so live-reload / HMR works.

## Platform behavior

- LAN mDNS publishing is available on macOS and Linux. Windows reports it as
  unsupported in `servd doctor`; regular loopback and nip.io routing still
  work.
- Hosts-file synchronization requires an elevated terminal. The hosts file is
  `/etc/hosts` on macOS and Linux, and
  `%SystemRoot%\System32\drivers\etc\hosts` on Windows.
- Launcher tools such as Node, Hugo, jigyll, Jekyll, `just`, and `make` are
  optional host dependencies. `servd doctor` reports which capabilities are
  available.
- User-authored commands run through `sh` on macOS and Linux and through
  `cmd.exe` on Windows. Commands that depend on POSIX shell syntax are not
  portable to Windows.

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

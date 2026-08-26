# servd

[![CI](https://github.com/reidransom/servd/actions/workflows/ci.yml/badge.svg)](https://github.com/reidransom/servd/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/reidransom/servd.svg)](https://pkg.go.dev/github.com/reidransom/servd)
![Go Version](https://img.shields.io/github/go-mod/go-version/reidransom/servd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run and manage many local dev servers at once.

`servd` runs registered web projects on stable backend ports and reverse-proxies
them by exact hostname. A folder of client sites becomes
`http://acme.localhost/`, `http://blog.localhost/`, and so on, all managed from
one CLI or interactive TUI. Optional [nip.io](https://nip.io) hostnames remain
available for compatibility.

Each registered project supplies its command explicitly, either at registration
or in the repository's root `.servd.toml`. That keeps command selection local
and deterministic, regardless of project files, installed tools, or global
configuration.

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

Create a repository command:

```toml
# ~/clients/acme/.servd.toml
cmd = "npm run dev"
```

Then register and run it:

```sh
servd add ~/clients/acme  # assign a port + slug
servd up --all            # start every registered dev server
servd proxy up            # start the hostname reverse proxy
servd                     # open the interactive dashboard (TUI)
```

Alternatively, supply a command only for this registration:

```sh
servd add ~/clients/acme -- npm run dev
```

Every entry in `sites.toml` is managed by servd and participates in
`up --all` and `restart --all`. To start a single registered site, name it:
`servd up acme`.

Then visit `http://<slug>.localhost/` for any site, or `http://127.0.0.1/`
for a landing page listing them all. `.localhost` resolves to loopback without
DNS setup.

### Proxy port selection

When `config.toml` is absent, `servd proxy up` first tries `127.0.0.1:80`.
If that requires permission in an interactive terminal, it asks whether to use
`port-less mode (requires root password)` before invoking `sudo`. Answering no
falls back to `127.0.0.1:8080` without requesting a password. If the user
confirms, `sudo` binds only the listener and the HTTP proxy runs as the invoking
user. A noninteractive command attempts passwordless elevation and falls back
immediately. Port 80 conflicts also fall back to port 8080. The fallback applies
only to that run and is not written to configuration.

Set an explicit port to make the choice strict:

```toml
[hostnames]
http_port = 8080
```

An explicit privileged port fails when it cannot be acquired; it never silently
falls back. While running, status, site URLs, the TUI, and browser-open actions
use the proxy's recorded runtime port. Route changes in `sites.toml` reload
without restarting the proxy.

## Commands

Each site's next command comes from exactly one of these sources:

1. an explicit command stored with that site's registration; or
2. a nonblank top-level `cmd` in `<registered-root>/.servd.toml`.

An explicit command takes precedence and Servd does not read `.servd.toml` for
that site. Otherwise, Servd reads only the registered root's `.servd.toml`;
it never searches parent directories or infers a command from Procfiles,
framework files, package scripts, recipes, directory contents, installed
tools, or global rules. A missing, unreadable, malformed, or invalid repository
configuration leaves the site in error. `servd which <slug>` shows the source
(`explicit` or `.servd.toml`) and the resolved command for the next start.

Repository commands use a one-field file at the registered root:

```toml
cmd = "bundle exec middleman serve --bind {host} --port {port}"
```

`{host}` and `{port}` are substituted literally before execution. Servd also
exports `HOST` and `PORT`, so ordinary environment expansion works too:

```toml
cmd = "bundle exec rackup --host $HOST --port $PORT"
```

Register with either supported form:

```text
servd add <path>
servd add <path> -- <command>...
```

The path must be an existing directory. Path-only registration requires a valid
root `.servd.toml`. With `--`, exactly one path must precede it and at least one
command argument must follow it. The trailing values are an argument vector:
Servd applies platform-specific shell quoting and stores the resulting explicit
command. They are not shell syntax, so `&&` after `--` is passed as an argument.
To use shell operators, invoke a shell explicitly, for example:

```sh
servd add . -- sh -c 'first && second'
```

`add` only registers new paths; it does not update an existing registration.
Replace an explicit command by removing and re-adding the site:

```sh
servd rm acme
servd add ~/clients/acme -- npm run dev
```

To remove an explicit override and return to the repository command:

```sh
servd rm acme
servd add ~/clients/acme
```

Changes to a valid `.servd.toml` affect the next start without restarting a
healthy process. Fixing an invalid next command also leaves a running process
alone; restart is the explicit cutover operation.

### Static files

`servd static` is a foreground static HTTP server; it neither registers nor
supervises a site:

```text
servd static [--host <host>] [--port <port>] [--dir <directory>]
```

It accepts no positional arguments, and `--dir` defaults to the current working
directory. Listener settings take precedence in this order: explicit `--host`
and `--port` flags, `HOST` and `PORT` environment variables, then
`127.0.0.1` and `8080`. It does not load `config.toml`. The host must be
nonempty; the port must be an integer from 1 through 65535; and the selected
root must be a readable directory. Invalid inputs fail before opening a
listener.

Static serving is never selected automatically. Choose it in the repository:

```toml
cmd = "servd static"
```

or as an explicit command:

```sh
servd add ./public -- servd static
```

The server recursively serves files within its root and directory-local
`index.html` files. Missing routes and directories without an index return
404; it does not list directories or provide SPA fallback. A path with any
dot-prefixed segment returns 403, including `.well-known`. Symlinks are served
only when their resolved target stays within the resolved root and has no
dot-prefixed segment relative to that root.

### TUI and bulk operations

The TUI add form accepts only a repository path. It requires a valid root
`.servd.toml`; when configuration is invalid, the form stays open and displays
the error. Adding an explicit command is a CLI-only workflow.

Command-resolution failures are isolated to their site. `up --all`,
`restart --all`, and `down --all` attempt every selected site, report every
failure, and exit nonzero if any operation fails. The TUI does the same for
bulk actions and reports both counts, for example `Started 4 sites; 2 failed`.
An invalid site has error status and a red `✕` glyph; an already-running invalid
site remains stoppable.

### Migration

This is a breaking cutover: there is no automatic conversion of prior command
selection. Existing registrations with a nonblank explicit `cmd` continue to
work. A registration without one uses a valid root `.servd.toml` or becomes an
error; the recovery is to create that file or remove and re-add the site with
an explicit command.

Repository-owned command:

```toml
# .servd.toml
cmd = "npm run dev"
```

One-site explicit command:

```sh
servd rm acme
servd add ~/clients/acme -- npm run dev
```

A former Procfile entry:

```text
web: bundle exec middleman serve -p $PORT
```

becomes:

```toml
cmd = "bundle exec middleman serve -p $PORT"
```

A former static detection becomes:

```toml
cmd = "servd static"
```

Convert former custom launcher rules to an explicit command for each
registration or commit a `.servd.toml` to each affected repository. An existing
`launchers.toml` is left untouched and silently ignored. A manually persisted
`servd __static` command is not rewritten; replace it, or re-register the site,
with `servd static`.

### Command reference

| Command | Purpose |
|---|---|
| `servd add <path> [--slug] [--port] [-- <command>…]` | register one project with a repository or explicit command |
| `servd rm <slug>` | stop and unregister a site |
| `servd which <slug>` | show the source and resolved command for the next start |
| `servd static [--host <host>] [--port <port>] [--dir <directory>]` | run the foreground static server |
| `servd status [slug]` (alias `ls`) | table of every site, or one named site, with live status (`--json` for machines) |
| `servd up [slug…] [--all]` | start sites (`--all` starts every registered site; `--wait`/`--json` for scripts) |
| `servd down [slug…] [--all]` | stop sites (`--all` stops every registered site) |
| `servd restart [slug…] [--all]` | restart sites (`--all` restarts every registered site) |
| `servd logs <slug> [-f]` | show / follow a site's server output |
| `servd open <slug>` | open the nip.io URL in a browser |
| `servd proxy up\|down\|status` | manage the background reverse proxy |
| `servd proxy` | run the proxy in the foreground |
| `servd doctor` | check settings, ports, and nip.io resolution |
| `servd version` / `servd --version` | report version, commit, and build date |
| `servd` / `servd tui` | interactive dashboard |

### Dashboard
The dashboard is a split view: the site list on the left, and a live tail of
the highlighted site's log on the right, led by the `$ command` that started
(or would start) it. Moving the selection switches the log panel; `tab` moves
focus to the log so `↑/↓` scroll it (scroll up to pause the tail, scroll back to
the bottom to resume following).

Site glyphs are `○` stopped, `◐` starting, `●` running, and red `✕` error.
Select an error row to see its concise reason while the site log remains visible.

### TUI keys

`↑/↓` move · `tab` focus list/log · `s` start/stop · `r` rename · `R` restart ·
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
slug is supplied) where each site carries `slug`, `path`, `port`, `url` (through
the proxy), `direct_url` (straight to the dev server), and `status` (`stopped`
| `starting` | `running` | `error`). Error records also carry a concise `error`
reason; live records carry `pid`, `cmd`, `log`, `started_at`, and
`uptime_seconds`. Match your project by `path` to find its slug, then hit
`direct_url` (or `url` if the proxy is accepting).

`up --wait` polls until the server actually accepts connections (default
`--timeout 30s`), and exits non-zero — with the log tail in the error — if the
process dies or never binds. Failed launches remain in `error` until a
successful start or `servd down` clears the runtime attempt. Path and
command-resolution errors clear automatically when their source is fixed. With
`--json` the per-site results (including any `error`) go to stdout as an array.
Servers are already detached by default, so `up` never needs backgrounding
tricks; a second `up` on a running site is a no-op.

Use these instead of reading `state.json` directly — the file's format is
internal and may change.

## Files

- `~/.config/servd/config.toml` — settings (`port_range_start`, `proxy_port`,
  `domain_suffix`, `bind_host`)
- `~/.config/servd/sites.toml` — the site registry and site-specific explicit
  commands
- `<project>/.servd.toml` — the repository command at the registered root
  when no explicit command is stored
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
- `servd doctor` checks Servd settings, ports, and nip.io resolution; command
  selection does not depend on framework-specific tools.
- Repository and explicit commands run from the registered repository through
  `sh` on macOS and Linux and through `cmd.exe` on Windows. Commands that
  depend on POSIX shell syntax are not portable to Windows.

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

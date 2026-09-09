# servd

[![CI](https://github.com/reidransom/servd/actions/workflows/ci.yml/badge.svg)](https://github.com/reidransom/servd/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/reidransom/servd.svg)](https://pkg.go.dev/github.com/reidransom/servd)
![Go Version](https://img.shields.io/github/go-mod/go-version/reidransom/servd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run and manage many local dev servers at once.

`servd` runs registered web projects on stable backend ports and reverse-proxies
them by exact hostname. A folder of client sites becomes
`http://acme.localhost/`, `http://blog.localhost/`, and so on, all managed from
one CLI or interactive TUI. Additional hostnames require explicit fallback
suffixes; none are enabled by default.

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

## Hostname configuration

Each site has one primary hostname. The default hostname settings are:

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = []
enable_mdns = false
```

Omitting a key uses its default. `tlds` must be a nonempty string;
`tlds_fallback` must be a list of nonempty strings. Multi-label suffixes are
valid, but uppercase letters, surrounding whitespace and invalid DNS labels
are rejected rather than repaired. Each complete site hostname, including its
stored worktree prefix, must fit the DNS length limit.

To accept explicit alternatives for the same backend:

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = ["127.0.0.1.nip.io", "dev.example.com"]
http_port = 8080
```

For `acme`, the primary URL is `http://acme.localhost:8080/`. Both
`acme.127.0.0.1.nip.io` and `acme.dev.example.com` are exact aliases.
Fallbacks are not redirects or DNS failover, and servd never tries them
automatically. The landing page, `servd open`, status tables and TUI links
always use the primary. Duplicate fallback suffixes and a fallback equal to
the effective primary create no additional routes or URLs. Saving settings
preserves the declared list and its order.

You supply DNS or hosts entries for fallbacks. Automatic hosts synchronization
and `servd hosts sync` manage only primary names, using `127.0.0.1`.
They never map fallback names to loopback. `hosts_mode` remains `auto`,
`always` or `never`; `never` disables automatic sync, not explicit hosts
commands. Safari may need primary `.localhost` hosts entries in `auto` mode.

`enable_mdns` defaults to `false`. Set it to `true`, or pass
`servd proxy --enable-mdns`, to select `.local` as the effective primary and
publish that name through mDNS. Explicit fallbacks remain routable, but mDNS
publishing adds no other aliases and does not change backend binding or grant
network access. The declared primary suffix remains saved for use when mDNS
is disabled. `lan_ip` remains the optional LAN address advertised through mDNS;
otherwise servd detects that address.

For remote access, a primary such as `tlds = "100.101.102.103.nip.io"`
with `tlds_fallback = ["localhost"]` makes remote links primary while keeping
an explicit local alias. Replace that address with the server's Tailscale IP.
This does not configure DNS, Tailscale or a public listener. See the
[Tailscale setup](docs/tailscale-integration.md).

### Migrating the old hostname schema

This is a breaking change. Edit `config.toml` manually; servd rejects the old
hostname keys and array-valued `tlds`, even alongside new settings. It never
rewrites the file or silently enables a previously disabled alternative.

| Old setting | Manual replacement |
| --- | --- |
| `tlds = ["localhost"]` | `tlds = "localhost"`, with no fallbacks unless wanted |
| `tlds = ["dev.example.com", "localhost"]` | `tlds = "dev.example.com"` and, if wanted, `tlds_fallback = ["localhost"]` |
| `nip_io = false` | Delete the key; there is no replacement |
| `nip_io = true` with the default suffix | Delete the key; add `"127.0.0.1.nip.io"` to `tlds_fallback` only if wanted |
| `nip_io_suffix = "100.101.102.103.nip.io"` | Delete the key; explicitly choose that suffix as primary or fallback |
| No config file | Keep the default `.localhost` primary; the implicit nip.io route disappears |
| `lan = true` or `lan = false` | Rename to `enable_mdns`, keeping the boolean value; `false` may be omitted |

An empty primary array remains invalid. For longer arrays, choose the first
entry as the scalar primary and retain remaining entries as fallbacks only
when intended. Names moved into `tlds_fallback` remain routes but are no
longer owned by servd's managed hosts block. Inspect stale entries with
`servd hosts status` before running `servd hosts sync`, which replaces the
managed block with current primary names. Preserve any needed fallback
entries separately with their intended addresses.

Rename the `--lan` CLI flag to `--enable-mdns`. The old flag and config key
are rejected rather than retained as aliases.

Legacy `proxy_port` and `hostnames.sync_hosts` migration still works.
An explicit legacy `domain_suffix` becomes the scalar primary only when
`hostnames.tlds` is absent, including when that suffix is a nip.io domain.
It never creates an implicit fallback.

Update scripts for scalar JSON `proxy.tlds`, plural `fallback_urls` and
`fallback_url_patterns`, and the removal of `proxy.nip_io`. Restart an
already-running proxy after editing hostname settings:

```sh
servd proxy down
servd proxy up
```

Do not restart backends or change their bind addresses solely for this schema
change.

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
| `servd open <slug>` | open the primary URL in a browser |
| `servd proxy up\|down\|status` | manage the background reverse proxy |
| `servd proxy` | run the proxy in the foreground |
| `servd doctor` | check settings, ports, primary resolution and configured fallback names |
| `servd version` / `servd --version` | report version, commit, and build date |
| `servd` / `servd tui` | interactive dashboard |

### Dashboard
The dashboard is a split view: the proxy followed by registered sites on the
left, and a live tail of the selected server's log on the right. The proxy is
selected initially, including when no sites are registered. Its pane is labeled
`proxy log`; site panes show the next `$ command`. The footer shows the proxy's
landing URL or the selected site's primary URL, using the active proxy port.

Moving the selection switches the log panel; `tab` moves focus to the log so
`↑/↓` scroll it. Scroll up to pause the tail, then back to the bottom to resume.
The proxy scrolls with the sites rather than staying pinned above them.

Site glyphs are `○` stopped, `◐` starting, `●` running, and red `✕` error.
Select an error row to see its concise reason while the site log remains visible.

Drag with the left mouse button to highlight text; releasing automatically copies
it to the system clipboard. No Shift key or terminal copy-on-select setting is
needed. Clipboard delivery uses OSC 52, supported by Ghostty and other modern
terminals. Selections begun inside a pane stay within that pane, excluding its
borders and the neighboring pane.

The highlighted view stays stable while logs and statuses refresh underneath.
The next click, keypress (including `esc`), wheel scroll, or resize clears the
selection and shows the latest content. A click without dragging still selects
a server or focuses a pane; wheel scrolling works as before.

### TUI keys

`↑/↓` move · `tab` focus list/log · `s` start/stop selected server ·
`r` rename site · `R` restart site · `d` remove site ·
`S` start/stop all sites · `a` add a site (type a path, `tab` completes) ·
`o` open selected server's URL · `h` show/hide this key help · `q` quit

Select the proxy and press `s` to start or stop it. The global `p` shortcut has
been removed. Rename, restart, and remove are site-only actions; `S` never
starts or stops the proxy. The help bar reflects the current selection.

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

`proxy.primary_url_pattern` uses the primary suffix. `proxy.tlds` is a string,
and `proxy.tlds_fallback` is the declared ordered array, including redundant
entries, or `[]` when empty. `proxy.fallback_url_patterns` and each site's
`fallback_urls` contain only distinct effective alternatives in declaration
order and are omitted when empty. Site `url` remains primary-only. All proxy
URLs and patterns use the active listener port, including a first-run fallback
from port 80 to 8080. The old `fallback_url`, `fallback_url_pattern` and
`nip_io` fields are absent.

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

- `~/.config/servd/config.toml` — settings such as `port_range_start`,
  `bind_host` and the `[hostnames]` table
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
  unsupported in `servd doctor`; regular primary and explicit fallback routing
  still work.
- Hosts-file synchronization requires an elevated terminal. The hosts file is
  `/etc/hosts` on macOS and Linux, and
  `%SystemRoot%\System32\drivers\etc\hosts` on Windows.
- `servd doctor` checks settings, ports and primary resolution. It also resolves
  actual registered fallback names when configured, without assuming wildcard
  DNS. Fallback-resolution failures are advisory, and remote addresses are
  valid. Command selection does not depend on framework-specific tools.
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

If a site builds absolute URLs from `Host` and needs the original routed
hostname, set `preserve_host = true` on its `[[site]]` entry in `sites.toml`
and add every hostname it accepts to that dev server's allowed-hosts setting.

---

[MIT licensed](LICENSE). Maintained by [r2ware](https://r2ware.dev).

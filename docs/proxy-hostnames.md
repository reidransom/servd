---
title: Proxy ports and hostnames
permalink: /proxy-hostnames/
---

The proxy accepts exact configured hostnames and sends every matching request to that site's backend. Start it in the foreground with `servd proxy`, or manage the background proxy with `servd proxy up`, `down`, and `status`.

## Choose a port

With no `config.toml`, the first background start tries port 80. In an interactive terminal it offers listener-only elevation; accepting runs the listener through `sudo` while the proxy remains the invoking user. Refusing falls back to 8080. Noninteractive starts attempt passwordless elevation and fall back immediately; a port-80 conflict also selects 8080. That fallback is runtime-only and is never saved.

An explicit `[hostnames] http_port` is strict. A privileged or occupied configured port fails instead of falling back. Once running, `status`, generated URLs, the dashboard, browser opening, and route reloads use the recorded active runtime port.

## Configure exact names

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = []
enable_mdns = false
```

`tlds` is one nonempty primary suffix; `tlds_fallback` is a list of nonempty suffixes. Multi-label suffixes are valid. Configuration rejects wrong scalar/list types, uppercase values, surrounding whitespace, invalid DNS labels, and complete site names that exceed DNS length limits, including stored worktree prefixes. Errors propagate rather than silently removing an invalid route.

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = ["127.0.0.1.nip.io", "dev.example.com"]
http_port = 8080
```

For `acme`, the primary is `acme.localhost`; the listed suffixes are exact aliases for the same backend. They are not redirects, DNS failover, or URLs servd attempts automatically. Declared fallback order is preserved on save. Effective routes and fallback URL arrays deduplicate repeated suffixes and a suffix equal to the effective primary.

The landing page, `servd open`, status tables, dashboard links, and browser actions use the primary hostname only. Fallback routes work at the active proxy port; structured output exposes plural fallback URL fields where applicable.

## Hosts-file ownership

Automatic synchronization and `servd hosts sync` own only primary names in a managed block mapped to loopback. `hosts_mode` is `auto`, `always`, or `never`; `never` disables automatic synchronization but not explicit hosts commands. Safari can need primary `.localhost` entries even in auto mode. Inspect stale managed lines with `servd hosts status` and remove only servd's block with `servd hosts clean`.

Fallback names are your DNS or hosts-file responsibility. They may deliberately resolve elsewhere and servd never writes them to loopback.

## mDNS

`enable_mdns = true` or `servd proxy --enable-mdns` uses `.local` as the effective primary and advertises that primary on the LAN. Explicit fallback routes remain. mDNS does not change backend binding, publish arbitrary fallbacks, or turn on network exposure. macOS and Linux support publishing; Windows reports it as unsupported in `servd doctor`. `lan_ip` chooses the advertised LAN address when set; otherwise servd detects one.

## Backend request contract

By default, the proxy rewrites `Host` to the backend address so common dev-server host allowlists accept proxied traffic. The original routed host and scheme remain in `X-Forwarded-Host` and `X-Forwarded-Proto`, and the proxy supports WebSocket upgrades. Set `preserve_host = true` on a site only when its backend needs the routed name; then configure that backend to allow every hostname it accepts.

## Migrate old hostname settings

Edit `config.toml` manually, then restart the proxy. Backends do not need to restart or rebind.

| Old setting | Replacement |
| --- | --- |
| `tlds = ["localhost"]` | `tlds = "localhost"` |
| multiple `tlds` entries | first becomes primary; retain deliberate remaining entries in `tlds_fallback` |
| `nip_io` | delete it; add an explicit fallback only if wanted |
| `nip_io_suffix` | delete it; choose it as a primary or fallback deliberately |
| `lan` / `--lan` | `enable_mdns` / `--enable-mdns` |
| no configuration | default `.localhost` primary; no implicit nip.io route |

Removed hostname keys and array-valued `tlds` fail with migration guidance. Legacy `proxy_port` and `hostnames.sync_hosts` remain supported. A legacy `domain_suffix` becomes the primary only when new `hostnames.tlds` is absent; it never creates an implicit fallback. Update scripts for scalar `proxy.tlds`, plural `fallback_urls` and `fallback_url_patterns`, and removed provider-specific fields.

```sh
servd hosts status
servd proxy down
servd proxy up
```

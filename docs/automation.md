---
title: Automation and JSON
permalink: /automation/
---

Automation should consume `servd status --json` and `servd up --wait --json`, not human tables or internal runtime files.

## Status object

`status --json` emits a `proxy` object and `sites` array. The proxy includes `running`, `accepting`, `pid`, `port`, `url`, scalar `tlds`, declared `tlds_fallback`, primary URL pattern, and effective `fallback_url_patterns`. Each site includes `slug`, `path`, backend `port`, primary `url`, `direct_url`, `status`, and when relevant `error`, `pid`, runtime `cmd`, `log`, `started_at`, and `uptime_seconds`.

Match a registered project by `path`, not a formatted table. A current-directory selection or explicit slug narrows the site array. Primary `url` is scalar; `fallback_urls` is an ordered effective array and is omitted when empty. Declared fallback configuration preserves duplicates, while effective fallback arrays deduplicate and exclude the primary. All proxy URLs use the active port. Retired singular fallback and provider-specific fields are absent.

## Wait for readiness

```sh
servd up acme --wait --timeout 45s --json
```

`--wait` polls until a backend accepts connections. The default per-site timeout is 30 seconds; `--timeout` overrides it. Process death or bind timeout exits nonzero and includes the log tail. JSON writes a per-site result array and exits nonzero for any failure. Launch errors remain available until a successful start or `servd down`; corrected path or command configuration recovers automatically. Starting an already running site is a no-op.

## Runtime behavior

Sites run detached in process groups, so they survive CLI and dashboard exit. `down` signals the whole group. The proxy forwards WebSocket upgrades for live reload and HMR. See [Commands and static serving](../commands/) for command resolution and [Proxy ports and hostnames](../proxy-hostnames/) for URL fields.

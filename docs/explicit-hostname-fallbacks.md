# Explicit hostname fallbacks

Status: Implemented and locally verified, 2026-09-08. This document retains the original implementation requirements.

## Decision

Keep the config key `hostnames.tlds`, but change its value from a list to one string. Add `hostnames.tlds_fallback` as an optional list with an empty default. Remove `nip_io` and `nip_io_suffix` entirely from the active configuration and output model.

There is one primary hostname per site. Every alternative hostname must come from an explicitly configured fallback suffix. A nip.io domain is an ordinary suffix with no provider-specific switch, default, URL helper or diagnostic.

This is a breaking change. Do not implement it as a compatibility layer that continues accepting the old hostname schema.

## Configuration contract

The default behavior is equivalent to:

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = []
```

Omitting the table or either key uses its default. In particular, omitting `tlds_fallback` never creates a nip.io hostname.

Explicit local alternatives:

```toml
[hostnames]
tlds = "localhost"
tlds_fallback = ["127.0.0.1.nip.io", "dev.example.com"]
```

For site `acme`, the primary is `acme.localhost`. The other two names are accepted aliases for the same backend. A fallback is not a redirect, a DNS failover mechanism or another backend. Servd never tries fallback URLs automatically when the primary fails.

A Tailscale-oriented example:

```toml
bind_host = "127.0.0.1"

[hostnames]
tlds = "100.101.102.103.nip.io"
tlds_fallback = ["localhost"]
http_port = 8080
hosts_mode = "never"
enable_mdns = false
```

Replace the example address with the server's Tailscale IP. Primary links use the remote suffix; `acme.localhost` remains an explicitly requested local alias. This config does not configure DNS, Tailscale or a public listener. Those remain separate setup steps.

### Validation and ordering

- `tlds` must be a nonempty string. `tlds_fallback` must be a list of nonempty strings. Reject an array for `tlds` and a string for `tlds_fallback`.
- Apply the existing `hostnames.ValidateTLD` rules to every supplied suffix. Multi-label suffixes remain valid. Do not introduce new case folding or whitespace repair for configuration values.
- Validate the complete generated hostname for each site and suffix, including a stored worktree prefix and the DNS length limit. An invalid primary or fallback must produce an error, not disappear from the route list or promote another suffix to primary.
- Preserve fallback declaration order. When deriving effective fallbacks, omit repeated suffixes and any suffix equal to the effective primary. Keep the declared config list intact when saving; redundant entries do not create duplicate routes or fallback URLs.
- Compose primary and fallback names from the same stored site identity. Preserve the current slug and worktree-prefix rules. No wildcard matching, suffix guessing or routing by the leftmost label.

## Routing and generated URLs

The route table contains the primary hostname followed by the distinct explicit fallback hostnames. All names for a site share its existing backend reverse proxy. Retain exact Host matching, request-host normalization, cross-site collision rejection, unknown-host landing behavior, backend Host rewriting and WebSocket handling.

`SiteURL`, `PrimaryURLPattern`, the landing page, `servd open`, status tables and TUI links use only the primary suffix. Fallback configuration never changes those links. HTTP scheme and effective proxy-port handling remain unchanged.

In [internal/config/config.go](../internal/config/config.go):

- Replace the list-valued Go `TLDs` field with a scalar `TLD`, retaining `toml:"tlds"`. Add `TLDsFallback []string` with `toml:"tlds_fallback"` and matching presence-aware loader fields.
- Remove `NipIO`, `NipIOSuffix`, `NipIOHostname`, and their defaults and serialization paths.
- Keep `PrimaryHostname` as the single-primary operation. Remove `PrimaryHostnames`; its callers no longer need a per-site primary list.
- Replace singular `FallbackURL` and `FallbackURLPattern` with `FallbackURLs` and `FallbackURLPatterns`. Derive them from the same effective fallback list used by `RouteHostnames`, rather than duplicating suffix rules in callers.
- Keep composition and validation inside the config/hostname modules. Consumers must not join suffixes or decide which fallback counts as primary.

The former [ParseHostnames](../internal/hostnames/hostnames.go) skipped invalid suffix-specific results and could return a later hostname when an earlier result was invalid. That parser and its implementation-only helpers and tests have been removed. The primary/fallback implementation uses single-hostname composition and propagates each error.

## CLI and JSON contract

Update [internal/commands/info.go](../internal/commands/info.go) without retaining singular aliases:

| Record | Change |
| --- | --- |
| Site | Keep `url` as the primary URL. Replace `fallback_url` with `fallback_urls`, an ordered array omitted when empty. |
| Proxy | Change `tlds` from an array to a string. Add `tlds_fallback`, an array that is `[]` when empty and reflects the declared config list. |
| Proxy | Replace `fallback_url_pattern` with `fallback_url_patterns`, an ordered array omitted when empty. |
| Proxy | Remove `nip_io`. |

Keep unrelated status, health, runtime and direct-URL fields unchanged. Fallback URLs and patterns contain only distinct effective alternatives and use the active listener port, including first-run fallback-port behavior.

Update both foreground and background startup output in [internal/commands/proxy.go](../internal/commands/proxy.go) to print every configured fallback pattern with provider-neutral wording. Print no fallback section when none exists.

Update `servd open` help in [internal/commands/run.go](../internal/commands/run.go) to say primary URL, not nip.io URL.

Replace the nip.io-specific check in [internal/commands/misc.go](../internal/commands/misc.go) with generic diagnostics for explicitly configured fallback hostnames of registered sites. Do not probe a synthetic `test.<suffix>` name, which incorrectly assumes wildcard DNS. With no registered sites or no fallbacks, skip those lookups. Keep optional fallback-resolution failures advisory and do not require fallback addresses to be loopback. Preserve unrelated primary-resolution diagnostics in this change.

## Hosts-file and mDNS behavior

Hosts synchronization continues to own primary names only. Adapt [internal/commands/hosts.go](../internal/commands/hosts.go) to collect one primary hostname per site and [internal/hostsfile/hostsfile.go](../internal/hostsfile/hostsfile.go) to evaluate a scalar primary suffix. Preserve sorting, deduplication across registry entries, Safari handling, `hosts_mode` behavior and explicit hosts commands.

Do not automatically write fallback names to `127.0.0.1`. A fallback may deliberately resolve to another address, as in the Tailscale example. Users supply DNS or hosts entries for fallbacks themselves.

`hostnames.enable_mdns` defaults to `false`. Enabling it, or passing `--enable-mdns`, selects the effective primary suffix `local`, without changing backend binding. Keep explicitly configured fallbacks routable, but do not invent additional aliases. In [internal/proxy/mdns.go](../internal/proxy/mdns.go), publish only the single primary `.local` name per site through mDNS. Do not attempt to publish arbitrary fallback suffixes through mDNS. The old `lan` config key and `--lan` flag have been replaced, not retained as aliases; preserve the boolean value when renaming the key.

This makes migration from a multi-primary array observable: entries moved into `tlds_fallback` remain routes but are no longer owned by primary hosts-file synchronization. Explain that distinction and inspect stale managed entries during migration rather than silently preserving the old ownership model.

## Breaking migration

Reject removed hostname keys with an actionable error, even when `nip_io = false` or a new configuration key is also present. Do not silently ignore them, automatically turn `nip_io = true` into a fallback, rewrite the user's file, or accept both array and string forms of `tlds`.

The loader otherwise ignores unknown fields. A targeted removed-key check for `hostnames.nip_io` and `hostnames.nip_io_suffix` prevents deletion from silently discarding a user's intended routes. Removed-key detection is an error path only, not a second accepted settings model. Unrelated unknown-key handling is unchanged.

Document manual migration:

| Old setting | New setting |
| --- | --- |
| `tlds = ["localhost"]` | `tlds = "localhost"`, no fallbacks unless explicitly wanted |
| `tlds = ["dev.example.com", "localhost"]` | `tlds = "dev.example.com"`, `tlds_fallback = ["localhost"]` |
| `nip_io = false` | Delete the key. It has no replacement. |
| `nip_io = true` with the default suffix | Delete the key. Explicitly add `"127.0.0.1.nip.io"` to `tlds_fallback` only if that route is wanted. |
| `nip_io_suffix = "100.101.102.103.nip.io"` | Delete the key. Explicitly choose that suffix as primary or as a fallback. A previously disabled fallback does not become enabled during migration. |
| No config file | Keep the default primary `.localhost` route; the old implicit nip.io route disappears. |

An old empty primary array remains invalid. For a multi-primary array, the first entry becomes the scalar primary; remaining entries become explicit fallbacks only when the user chooses to retain them. Update scripts for the JSON type change and plural fallback fields in the same release.

Retain unrelated legacy migrations for `proxy_port` and `sync_hosts`. Retain explicit legacy `domain_suffix` translation to the scalar primary when `hostnames.tlds` is absent, but remove the hardcoded `127.0.0.1.nip.io` exception. An explicitly supplied domain suffix is an ordinary primary suffix, never permission to add an implicit fallback. An explicit new `tlds` still takes precedence. Do not broaden this change into removal of all historical config migrations.

Restart an already-running proxy after migrating its hostname configuration. Do not restart backends or change their bind addresses solely for this schema change.

## Implementation sequence

1. **Change the hostname model.** Update config fields, defaults, load/save behavior, removed-key errors, suffix validation and primary/fallback helpers. Adapt legacy scalar-suffix precedence without retaining provider-specific logic.
2. **Migrate every consumer.** Update routing, LAN publication, hosts synchronization, CLI output, doctor and JSON serialization. Remove obsolete helpers after their callers are gone. Search source, tests and documentation for the retired fields and singular fallback interfaces; use LSP references and renames if a Go language server is available.
3. **Verify behavior.** Update existing behavioral coverage, run focused package tests, then exercise the real foreground CLI and HTTP proxy using isolated config and state. Run the repository's build, vet and full test commands once after the changes settle.
4. **Update shipped documentation.** After the smoke check passes, update README configuration, `open`/`doctor` descriptions, JSON documentation and migration guidance. Rewrite the setup examples in [tailscale-integration.md](tailscale-integration.md) for scalar `tlds` and explicit `tlds_fallback`. Preserve its old experiment as dated historical evidence, or replace it only with a rerun against the new implementation. Remove the throwaway smoke program. Do not create a new changelog convention solely for this change.

### Verification requirements

Prefer updating existing tests in the affected packages. Keep new regression tests only where they defend behavior that could plausibly break.

- A default site routes through its primary hostname, while the formerly implicit nip.io hostname takes the unknown-host path.
- Two explicit fallback suffixes route to the same backend, including a worktree-prefixed site. Unconfigured aliases remain unknown, and the landing link remains primary-only.
- Duplicate fallback declarations and a fallback equal to the primary do not produce duplicate URLs or routes. Cross-site collisions still fail route construction.
- Invalid primary and fallback suffixes, wrong TOML types, complete-hostname overflow and removed keys return errors. A valid fallback must not conceal an invalid primary or another invalid fallback.
- Load/save/load preserves scalar primary and explicit fallback choices, including an empty list. Legacy `domain_suffix` precedence produces no implicit fallback.
- JSON exposes the new scalar and list shapes, supports multiple fallback URLs at the active proxy port, and omits retired fields. Avoid tests that merely compare copied Go fields.
- Hosts sync manages only the primary. LAN publishes only the primary `.local` name while retaining explicit fallback routes.
- Doctor checks actual configured fallback names without assuming wildcard DNS or loopback addresses. Use controlled resolution results, not public DNS in permanent tests.

For the smoke check, start a disposable backend and the actual foreground servd proxy with temporary XDG directories and an unprivileged port. Send requests with primary, fallback, prefixed and unconfigured Host headers; inspect the backend responses, landing links, `servd status --json` and startup output. Stop both processes and remove temporary files. No Tailscale connection or public DNS dependency is required to prove this config/routing change.

Existing tests in `internal/config`, `internal/proxy`, `internal/commands`, `internal/hostnames` and `internal/hostsfile` need contract updates. Check `internal/tui` for fixtures or output assertions affected by the change. Delete tests for removed multi-primary or implicit-provider behavior rather than preserving that behavior through new aliases.

## Completion criteria

- One scalar primary suffix and an explicit, empty-by-default fallback list are the only active hostname configuration model.
- No nip.io-specific behavior remains in runtime routing, defaults, serialization, URL helpers or diagnostics. The domain may still appear in user examples, explicit test configurations and removed-key migration errors.
- CLI, JSON, routing, LAN and hosts-file behavior agree on primary versus fallback ownership.
- Old hostname configuration fails with migration guidance instead of being silently accepted or partially ignored.
- Documentation describes the implemented contract, and focused tests plus a real CLI/proxy smoke check demonstrate it.

Verification completed with focused package tests, `go build ./...`, `go vet ./...`, and `go test ./... -race`. A real foreground CLI/proxy smoke check exercised primary, two explicit fallbacks, worktree-prefixed and unknown Host headers, primary-only landing links, JSON shapes and startup output. A second run with fallbacks omitted confirmed the former implicit nip.io route takes the unknown-host path. Temporary processes were stopped. The Tailscale note now uses the implemented schema and preserves its earlier experiment as dated historical evidence.

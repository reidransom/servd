# Tailscale access to servd

Status: Setup updated for explicit hostname fallbacks, 2026-09-08. No live Tailscale configuration changes. The earlier experiment is retained below as historical evidence.

## Recommendation

Keep servd and its backends on `127.0.0.1`. Use an external **raw TCP Tailscale Serve forwarder** into the proxy, then give clients DNS or hosts entries for servd's exact site hostnames. This needs no servd code changes and does not deliberately expose backend ports or the LAN interface.

For a few computers, use explicit hosts entries. For phones or many changing sites, use a wildcard under a domain you control. An IP-encoded `nip.io` suffix is the shortest setup if depending on public DNS is acceptable.

```text
Browser: http://acme.servd.test:8080/
  -> DNS/hosts: servd machine's Tailscale IP
  -> Tailscale Serve TCP :8080
  -> servd 127.0.0.1:8080, Host: acme.servd.test:8080
  -> site's loopback backend
```

[Tailscale documents raw TCP forwarding][serve-cli]. Its implementation copies bytes in both directions without parsing HTTP, so the incoming Host and WebSocket upgrade reach servd unchanged. Do not enable PROXY protocol: it prepends bytes that servd's ordinary HTTP listener does not consume. [TCP implementation][serve-source]

## Existing servd behavior

- `bind_host` defaults to `127.0.0.1`. The proxy uses it both for its listener and its backend dial address. Launch commands receive it through `{host}` and `HOST`. Changing it is therefore not a proxy-only operation. Explicit project commands can still bind somewhere else. [Settings][config], [proxy][proxy], [launcher](../internal/launcher/launcher.go), [supervisor](../internal/supervisor/supervisor.go)
- Default primary names are `<slug>.localhost`, or `<prefix>.<slug>.localhost` for a stored worktree prefix. There are no default fallbacks. `.localhost` points clients back to themselves and is unsuitable for remote access. An explicitly configured `127.0.0.1.nip.io` suffix also resolves to the client's loopback address. [Hostname generation][config], [RFC 6761, section 6.3][special-names]
- `hostnames.tlds` is one primary suffix string, and `hostnames.tlds_fallback` is an optional list of explicit aliases. Both accept multi-label suffixes. Only the primary controls landing-page links, CLI output and `servd open`. Incoming request hosts are case-folded, with an optional port and trailing root dot removed; configuration suffixes are validated without case folding or whitespace repair. There is no arbitrary-subdomain fallback. A request for the machine's MagicDNS name or an unconfigured `acme.machine.tailnet.ts.net` gets the site index, not the `acme` backend. [Settings][config], [routing and landing page][proxy], [open command](../internal/commands/run.go)
- Generated URLs always use HTTP and `hostnames.http_port`. The settings default is `8080`; without a config file, startup prefers port `80` and can fall back to `8080`. Use the actual active port, not an assumed default. Setting `hostnames.https = true` currently fails validation. [Settings][config], [app loading](../internal/app/app.go), [proxy startup](../internal/proxy/control.go)
- After selecting a site, servd rewrites backend Host to the backend address by default and supplies `X-Forwarded-Host`, `X-Forwarded-Proto` and `X-Forwarded-For`. A site's `preserve_host = true` keeps the original Host but requires the backend to allow that hostname. The Go reverse proxy supports HTTP/1.1 WebSocket upgrades. [Proxy][proxy]
- `hostnames.enable_mdns = true` or `--enable-mdns` selects `.local` as the effective primary and publishes only that name through mDNS. Explicit fallbacks remain routes. It does not change `bind_host`. LAN discovery is not tailnet DNS, and Tailscale's mDNS support request remains open. Leave `enable_mdns` false or omitted for this setup. [EnableMDNS][config], [publisher](../internal/proxy/mdns.go), [Tailscale mDNS issue][mdns]

## Usable setup

The example assumes an existing registered site `acme`, another site `blog` with stored prefix `auth`, and Tailscale installed and connected on the server and clients. Replace `100.101.102.103` with the server's output from `tailscale ip -4`. Run Tailscale commands with the required local permissions, using `sudo` if needed.

### 1. Choose a remote hostname suffix

Merge this into `~/.config/servd/config.toml`, or its `XDG_CONFIG_HOME` equivalent. Preserve unrelated settings and avoid duplicate TOML tables.

```toml
bind_host = "127.0.0.1"

[hostnames]
tlds = "servd.test"
tlds_fallback = ["localhost"]
http_port = 8080
https = false
hosts_mode = "never"
enable_mdns = false
```

`servd.test` is a private example namespace. `.test` is reserved for testing; it needs your explicit hosts entries or private DNS. `localhost` remains an explicitly requested local-only alias, not a second primary. Fallbacks are not redirects or automatic URL failover. [RFC 6761, sections 6.2 and 6.3][special-names]

On each remote Linux client, add the exact registered names to `/etc/hosts`:

```text
100.101.102.103 acme.servd.test auth.blog.servd.test
```

Hosts files contain explicit names, not wildcard patterns. Include each worktree-prefixed name and update the entries when sites change. For use on the servd machine itself, map these names to `127.0.0.1` there. [hosts file format][hosts-man]

`hosts_mode = "never"` disables automatic hosts synchronization when starting the proxy. It does not remove old entries or disable an explicit `servd hosts sync` command. Hosts synchronization now owns only primary names and writes `127.0.0.1`, not the Tailscale IP. It never writes fallback names or configures remote clients. Inspect stale managed entries with `servd hosts status` when migrating an old multi-primary list. Names moved to `tlds_fallback` lose managed-block ownership; supply their DNS or hosts entries yourself. [Hosts implementation](../internal/hostsfile/hostsfile.go), [hosts commands](../internal/commands/hosts.go)

### 2. Start servd and the TCP forwarder

Restart an existing proxy after changing hostname settings. This briefly interrupts its requests; backends do not need new bind addresses.

```sh
servd proxy down
servd proxy up
servd up acme

# Inspect existing Serve configuration before choosing a port.
tailscale serve status
tailscale serve --bg --tcp=8080 tcp://127.0.0.1:8080
tailscale serve status
```

Use an unoccupied Serve port. The loopback proxy and Tailscale endpoint can use the same port because they use different addresses. Keeping both at `8080` also keeps generated links correct. `--bg` persists the forwarding configuration across Tailscale restarts and machine reboots; it does not start servd for you. To remove only this forwarder later:

```sh
tailscale serve --bg --tcp=8080 off
```

Do not use `tailscale serve reset` to remove one entry from a machine that serves other applications. [Current CLI syntax and lifecycle][serve-cli]

### 3. Connect from another node

Open `http://acme.servd.test:8080/`. For a routing check before installing DNS or hosts entries:

```sh
curl --noproxy '*' \
  --resolve acme.servd.test:8080:100.101.102.103 \
  http://acme.servd.test:8080/
```

This checks the address and Host separately from DNS. A bare `http://100.101.102.103:8080/` should show the index, whose links now use `servd.test`. A backend must be running for its site request to succeed. These are setup commands, not a claim that a cross-node check has been run here.

## DNS alternatives

MagicDNS registers machine names such as `workstation.tailnet-name.ts.net`. It does **not** create `*.workstation.tailnet-name.ts.net`, and Tailscale explicitly disallows adding arbitrary MagicDNS records. Choosing a servd TLD that resembles that name does not create DNS records. A CNAME or hosts entry can resolve an alias, but the browser still sends the alias as Host, so it must also be a configured servd route. [MagicDNS][magicdns], [DNS configuration][dns]

For a shared, maintained setup, replace `servd.test` with `dev.example.com`, under a domain you control, and publish:

```text
*.dev.example.com.  300  IN  A  100.101.102.103
```

Use DNS-only records, not a public CDN proxy. This can cover `acme.dev.example.com` and nested names such as `auth.blog.dev.example.com`; explicit records or delegations can interrupt wildcard matching. Public DNS reveals the names and address, but does not grant network access to that tailnet IP. [Wildcard DNS behavior][wildcard], [Tailscale public DNS guidance][dns]

If those names should stay in private DNS, configure a reachable resolver for the suffix and add it as a restricted nameserver in Tailscale's DNS settings. Clients must accept Tailscale DNS settings, and policy must permit access to the resolver. For an existing dnsmasq resolver, the relevant suffix rules are:

```ini
address=/servd.test/100.101.102.103
local=/servd.test/
```

This is resolver configuration, not a complete DNS-server installation. Split DNS distributes where to ask; it does not host the records itself. [Restricted nameservers][dns], [dnsmasq address/local options][dnsmasq]

For the least DNS administration, replace only the `tlds` line in the setup with:

```toml
tlds = "100.101.102.103.nip.io"
```

Keep `tlds_fallback = ["localhost"]` for the explicit local alias. The primary suffix uses nip.io's public IP-to-DNS service; servd treats it like any other suffix. Generated URLs become `http://acme.100.101.102.103.nip.io:8080/`, including correct worktree-prefixed URLs. Alternatively, keep `tlds = "localhost"` and set `tlds_fallback = ["100.101.102.103.nip.io"]` to accept remote aliases without changing primary links. Delete any old `nip_io` or `nip_io_suffix` keys; the loader rejects them, even when disabled. [nip.io operator documentation][nip], [servd hostname generation][config], [migration guide](../README.md#migrating-the-old-hostname-schema)

The nip.io option depends on a third party and the client's resolver. Public DNS services can disclose queries and addresses, and filtering or rebinding protection may reject answers under local resolver policy. Whether a particular resolver blocks this Tailscale address is unverified; do not disable protection globally to make a demo work. Use hosts or controlled DNS when this matters. [DNS filtering controls][dnsmasq]

## Why not the other approaches?

| Approach | Decision for this repo |
| --- | --- |
| External raw TCP Serve into loopback | Recommended. Keeps backend binding unchanged, preserves site Host and upgrades, and needs only existing configuration plus DNS. |
| Set `bind_host` to the Tailscale IP | Possible on a host with an OS-visible Tailscale interface, but also changes backend dials, `{host}` and `HOST`. Backends honoring those settings become tailnet listeners; hardcoded loopback backends stop matching the dial address. Local `.localhost` access to the proxy also stops working. Do not use `0.0.0.0` as a shortcut. [servd sources][proxy], [Tailscale networking modes][userspace] |
| HTTP/HTTPS Serve, such as `tailscale serve --bg --https=443 http://127.0.0.1:8080` | Not a transparent replacement for multi-site Host routing. Current HTTP Serve preserves outbound Host, but first selects its own web handler using the configured node hostname for HTTP or TLS SNI for HTTPS. Browsing the node's normal HTTPS URL therefore reaches servd with the node Host and gets its index. Arbitrary site aliases are not automatically registered Serve web hosts or covered by its certificate. [Handler selection and Host forwarding][serve-source], [Serve limitations][serve] |
| Embed `tsnet` | Adds a dependency, node identity, authentication and persistent node state to solve a problem the existing daemon already solves. Useful only if servd should itself join the tailnet without a separately installed daemon. One embedded node still does not provide arbitrary per-site MagicDNS names. [tsnet][tsnet], [MagicDNS][magicdns] |

HTTP Serve's problem here is not that it always rewrites Host to `127.0.0.1`. Current source explicitly retains the incoming Host for normal network backends. Nor does mounting `/acme` make servd route by path. Its routing contract is hostname-based. [Serve source][serve-source], [servd proxy][proxy]

## Ports, HTTPS and WebSockets

Raw `--tcp` does not add TLS. HTTP and WebSocket bytes travel through Tailscale's encrypted node-to-node connection, but browsers still see HTTP rather than an HTTPS secure context. Some browser APIs and secure-cookie behavior therefore differ from local `.localhost` development. [Tailscale HTTPS explanation][https]

For clean port-80 links, both the active servd `http_port` and the external forwarding port should be `80`. Forwarding external `80` to local `8080` transports requests but leaves generated URLs advertising `:8080`; servd has no separate external URL port setting. [URL generation][config]

Tailscale's automatic certificates cover the node's tailnet DNS name, not your arbitrary per-site aliases. `--tls-terminated-tcp=443` retains the HTTP bytes after termination but does not solve this certificate-name mismatch. If browser-trusted HTTPS for many site names is required, use a separate Host-preserving TLS proxy with certificates for those names. servd would also need deliberate external-scheme/port and trusted-forwarded-header handling for correct generated links and backend scheme reporting. Its current rewrite derives forwarded values from the immediate incoming request, so an upstream `X-Forwarded-Proto: https` is not automatically retained. That is additional work, not part of this HTTP recommendation. [Certificates][https], [TCP TLS option][serve-cli], [servd rewrite][proxy]

WebSockets should traverse the raw TCP hop and servd's upgrade-capable reverse proxy. An application's HMR client may still advertise a hardcoded localhost hostname or backend port. Configure that application to use the browser-facing hostname and proxy port rather than exposing extra backend ports. Actual browser/HMR behavior has not been exercised here.

## Security scope

Serve is tailnet-only and respects the tailnet's ACLs or grants. Funnel intentionally publishes a service to the internet; do not enable it for this setup. The same port cannot be private Serve and public Funnel at once. Inspect existing configuration before making changes. [Serve access controls and Funnel distinction][serve]

Grant only the intended clients access to the external TCP port. For example, merge an entry like this into the existing policy's `grants` array, replacing the user and server IP:

```json
{
  "src": ["alice@example.com"],
  "dst": ["100.101.102.103"],
  "ip": ["tcp:8080"]
}
```

This is an allow rule, not a restriction that overrides broader existing grants or ACLs. New tailnets normally allow all devices to communicate. Review existing allow-all access before treating this as limited sharing. [Grants syntax][grants], [default policy][acls]

Access to this single port gives access to every registered site routed by that proxy, plus its index listing. Network policy cannot distinguish the HTTP Host values on the same destination port. Do not share it with someone who should see only one site. Raw TCP forwarding does not add authenticated Tailscale HTTP identity headers; do not trust client-supplied identity headers in an application. Backends remain subject to their own authentication and development-server risks. [Network grant scope][grants], [TCP implementation][serve-source], [servd index][proxy]

## Evidence and remaining uncertainty

### Historical experiment, 2026-09-08, before the schema change

The following experiment used the retired array-valued schema. It is historical evidence, not a configuration example for the current release.

Source inspection and an isolated local experiment support the recommendation. The experiment loaded the real servd proxy handler with `bind_host = "127.0.0.1"`, configured URL port `8080`, `tlds = ["100.101.102.103.nip.io", "localhost"]` and `nip_io = false`. The HTTP server, backend and byte-copy TCP relay used ephemeral loopback ports. The two remote-suffix site names and `acme.localhost` reached a real local backend with the original `X-Forwarded-Host`. Machine and unconfigured subdomain requests returned the index, with remote-first-suffix links on port `8080`. The throwaway program was removed.

That experiment was not Tailscale Serve, public DNS, a browser, an ACL check or a second-node connection. No cross-node outcome is claimed. Actual deployment still depends on the installed client version, running daemon, port availability, policy, resolver and backend URL/HMR behavior.

### Current hostname verification, 2026-09-08

An isolated smoke check ran the actual foreground servd CLI and a disposable loopback backend with temporary XDG config and state. Scalar primary `localhost` and explicit fallback suffixes `127.0.0.1.nip.io` and `dev.example.com` routed both ordinary and worktree-prefixed sites to that backend. Unconfigured names returned the landing page, whose links used only the primary. JSON preserved the declared fallback list but emitted distinct alternatives on the active unprivileged port. With fallbacks omitted, the former implicit nip.io names returned the landing page instead. This check did not run Tailscale or public DNS and does not replace the historical TCP-relay experiment.

There is a documentation discrepancy worth preserving. The general Serve guide says HTTPS certificates are required; the current CLI source only runs the certificate-enable flow for HTTPS mode, not raw TCP. Raw `--tcp` also does not terminate TLS. This recommendation follows the documented TCP mode and source, not a blanket assumption that every Serve mode needs a certificate. The inspected upstream revision was `a8b023c063b608fcead5446f3d885c4fc847c944`; it may be newer than an installed stable client. [General guide][serve], [CLI feature gate][serve-cli-source]

## Optional product integration

No product integration is required. If setup friction warrants one, the smallest useful addition is an explicit opt-in helper that reports the actual proxy port, prints the matching TCP Serve command and remote site URLs, and explains DNS and access scope. It should not silently change tailnet policy, enable Funnel, reset existing Serve configuration or change backend binding.

If native binding is later preferred, first separate proxy listener addresses from backend binding. Keep loopback as the default and add an explicit Tailscale-IP listener without changing backend targets. That is a separate feature proposal, not an existing setting and not a prerequisite for the no-code setup.

[config]: ../internal/config/config.go
[proxy]: ../internal/proxy/proxy.go
[serve-cli]: https://tailscale.com/docs/reference/tailscale-cli/serve
[serve]: https://tailscale.com/docs/features/tailscale-serve
[serve-source]: https://github.com/tailscale/tailscale/blob/a8b023c063b608fcead5446f3d885c4fc847c944/ipn/ipnlocal/serve.go
[serve-cli-source]: https://github.com/tailscale/tailscale/blob/a8b023c063b608fcead5446f3d885c4fc847c944/cmd/tailscale/cli/serve_v2.go#L468-L482
[magicdns]: https://tailscale.com/docs/features/magicdns
[dns]: https://tailscale.com/docs/reference/dns-in-tailscale
[https]: https://tailscale.com/docs/how-to/set-up-https-certificates
[tsnet]: https://tailscale.com/docs/features/tsnet
[userspace]: https://tailscale.com/docs/concepts/userspace-networking
[grants]: https://tailscale.com/docs/features/access-control/grants
[acls]: https://tailscale.com/docs/features/access-control/acls
[mdns]: https://github.com/tailscale/tailscale/issues/1013
[nip]: https://nip.io/
[wildcard]: https://developers.cloudflare.com/dns/manage-dns-records/reference/wildcard-dns-records/
[dnsmasq]: https://thekelleys.org.uk/dnsmasq/docs/dnsmasq-man.html
[hosts-man]: https://man7.org/linux/man-pages/man5/hosts.5.html
[special-names]: https://www.rfc-editor.org/rfc/rfc6761#section-6

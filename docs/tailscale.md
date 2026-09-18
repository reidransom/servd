---
title: Tailscale access
permalink: /tailscale/
---

Keep servd and its backends on loopback. Use a raw TCP Tailscale Serve forwarder to the active proxy port; it preserves HTTP hostnames and WebSocket upgrades without exposing backend ports or rebinding servd.

## Recommended workflow

1. Install and connect Tailscale on the servd machine and client.
2. Choose a remote primary suffix and keep explicit local aliases only when wanted:

   ```toml
   bind_host = "127.0.0.1"

   [hostnames]
   tlds = "servd.test"
   tlds_fallback = ["localhost"]
   http_port = 8080
   hosts_mode = "never"
   enable_mdns = false
   ```

3. Restart the proxy, start sites, inspect existing Serve configuration, and forward the matching port:

   ```sh
   servd proxy down
   servd proxy up
   servd up acme
   tailscale serve status
   tailscale serve --bg --tcp=8080 tcp://127.0.0.1:8080
   tailscale serve status
   ```

4. On a remote client, map exact configured names—including worktree prefixes—to the server's Tailscale IP:

   ```text
   100.101.102.103 acme.servd.test auth.blog.servd.test
   ```

5. Confirm address and Host routing independently of DNS:

   ```sh
   curl --noproxy '*' --resolve acme.servd.test:8080:100.101.102.103 http://acme.servd.test:8080/
   ```

Remove only this endpoint with `tailscale serve --bg --tcp=8080 off`; do not use `tailscale serve reset` on a shared machine.

## Naming choices

`.localhost` and loopback-encoded DNS resolve on the client, not the server. A bare Tailscale IP reaches the proxy index because servd routes exact hosts. MagicDNS creates node names, not arbitrary `acme.<node>.tailnet.ts.net` records.

| Choice | Tradeoff |
| --- | --- |
| Exact hosts entries | Private and simple for few clients; update every ordinary and worktree-prefixed name. |
| Controlled wildcard DNS | Scales under a domain you control; public DNS exposes names and address. |
| Split DNS | Keeps names private; needs a reachable resolver and accepted Tailscale DNS settings. |
| IP-encoded suffix | Minimal DNS administration; depends on a third party, client resolver policy, and rebinding protection. |

A scalar primary controls generated links. Fallbacks remain configured aliases, not provider-specific switches or automatic remote failover. See [Proxy ports and hostnames](../proxy-hostnames/).

## What raw TCP does not add

Raw TCP Serve is different from HTTP/HTTPS Serve, TLS termination, MagicDNS, native tailnet binding, and embedded tailnet nodes. It does not add browser-trusted TLS or a secure context. To keep port-80 links, the external Serve port and servd active port must both be 80; forwarding external 80 to local 8080 leaves generated URLs at `:8080`. Certificates for a node's tailnet name do not cover arbitrary site aliases. servd derives forwarded scheme from its incoming HTTP request, so upstream TLS is not automatically represented.

WebSocket bytes pass through, but application HMR clients may need their browser-facing hostname and proxy port configured.

## Security scope

Serve is tailnet-only and subject to grants or ACLs. Do not enable Funnel. Access to one proxy port exposes every registered site and the index; policy cannot distinguish HTTP hostnames on that port. Existing allow-all policies can make a new endpoint broader than intended. Raw TCP forwarding supplies no trustworthy user identity header—applications must enforce their own authentication.

## Historical evidence

A 2026-09-08 local byte-relay experiment used the retired array-valued hostname schema and showed host preservation through a loopback relay. It was not a Tailscale Serve run, browser check, cross-node test, DNS result, policy evaluation, or HMR proof. Current setup depends on installed client behavior, daemon state, ports, policy, resolver, and application configuration.

Read Tailscale's [Serve CLI reference](https://tailscale.com/docs/reference/tailscale-cli/serve), [Serve overview](https://tailscale.com/docs/features/tailscale-serve), and [DNS documentation](https://tailscale.com/docs/reference/dns-in-tailscale) for current provider behavior.

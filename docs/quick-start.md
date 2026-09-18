---
title: Quick start
permalink: /quick-start/
---

This walkthrough registers one repository, starts its backend and the hostname proxy, then opens the dashboard.

## Create a repository command

At the registered repository root, create `.servd.toml`:

```toml
# ~/clients/acme/.servd.toml
cmd = "npm run dev"
```

The registry records the path, slug, port, and any explicit command. The repository command remains owned by the repository and is resolved when servd starts the site.

## Register and start the site

```sh
servd add ~/clients/acme  # assigns a stable slug and backend port
servd up --all            # starts every registered site
servd proxy up            # starts the hostname proxy
servd                     # opens the interactive dashboard
```

A one-registration explicit command is a CLI-only alternative to `.servd.toml`:

```sh
servd add ~/clients/acme -- npm run dev
```

Visit the site's primary URL at `http://acme.localhost/`. Visit the proxy landing page at `http://127.0.0.1/` to see every registered site. The primary suffix is `.localhost` by default, and no additional fallback hostname is enabled unless you configure one.

## QR codes are local text

The landing page and dashboard can display a QR code for a primary URL. It is generated locally and includes the active proxy port. The scanning device must be able to reach the proxy and resolve the hostname: `.localhost` and loopback addresses refer to the scanning device, not the computer running servd. Showing a code neither configures DNS nor exposes servd to a network.

Detailed command and hostname guides are available in the full documentation.

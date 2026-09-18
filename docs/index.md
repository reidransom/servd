---
title: servd
layout: splash
permalink: /
hero:
  tagline: Stable local development servers, routed by exact hostname.
  actions:
    - text: Quick start
      link: /quick-start/
      variant: primary
    - text: Install servd
      link: /installation/
      variant: minimal
---

servd runs registered web projects on stable backend ports and routes them through one local proxy. A project becomes a predictable URL such as `http://acme.localhost/`, managed from the CLI or interactive dashboard.

## A small, explicit model

- A **registered project** has a stable slug, backend port, and directory.
- The proxy routes each request by its **exact hostname** to that project's backend.
- A project starts from an **explicit command** stored at registration or a **repository command** in its root `.servd.toml`. servd does not infer commands from framework files, package scripts, installed tools, or parent directories.
- The default primary hostname is `<slug>.localhost`. `.localhost` resolves to loopback without DNS setup. There are no implicit fallback hostnames; configure every additional alias deliberately.

Start with [installation](./installation/) and follow the [quick start](./quick-start/) to register a project, run it, and open its primary URL.

---
title: Files and platform behavior
permalink: /files-platform/
---

## Public files

- `~/.config/servd/config.toml` — settings such as `bind_host`, port range, and `[hostnames]`.
- `~/.config/servd/sites.toml` — registered projects and explicit commands.
- `<project>/.servd.toml` — repository command at the registered root.
- `~/.local/state/servd/state.json` — internal latest runtime attempts.
- `~/.local/state/servd/logs/<slug>.log` — site output.

XDG environment variables select their platform-specific locations. Edit settings, registry, and repository command deliberately. Do not make `state.json` a public integration contract; use [Automation and JSON](../automation/) instead. Legacy `enabled` site and `default_enabled` setting keys are ignored and may be deleted.

## Platform notes

mDNS publishing is available on macOS and Linux; Windows reports it unsupported in `servd doctor`. Hosts-file synchronization needs elevation and uses `/etc/hosts` on macOS/Linux or `%SystemRoot%\System32\drivers\etc\hosts` on Windows. `doctor` checks settings, ports, primary resolution, and configured fallback names; fallback-resolution failures are advisory and may resolve remotely.

Commands run from the registered root through `sh` on macOS/Linux and `cmd.exe` on Windows. POSIX quoting, expansion, and operators are not portable to Windows.

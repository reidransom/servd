---
title: CLI reference
permalink: /cli-reference/
---

## Commands

| Command | Purpose |
| --- | --- |
| `servd add <path> [--slug] [--port] [--host-prefix] [-- <command>…]` | Register one project. `--no-worktree-prefix` disables detected linked-worktree prefixing. |
| `servd rm [slug]` | Stop and unregister a site; project files remain. |
| `servd which [slug]` | Show the selected source and resolved command. |
| `servd static [--host] [--port] [--dir]` | Run the foreground static server. |
| `servd status [slug]` / `servd ls` | Show one selected site or all sites; `--json` emits machine output. |
| `servd up [slug…]` | Start selected sites; `--all` selects every site, `--wait` waits for readiness, `--timeout` sets per-site wait time, and `--json` emits results. |
| `servd down [slug…]` | Stop selected sites; `--all` selects every site. |
| `servd restart [slug…]` | Restart selected sites; `--all` selects every site. |
| `servd logs [slug] [-f]` | Read or follow site output. |
| `servd open [slug]` | Open the selected site's primary URL. |
| `servd proxy` | Run the proxy in the foreground; `--enable-mdns` selects `.local` for this run. |
| `servd proxy up`, `down`, `status` | Manage or inspect the background proxy. |
| `servd hosts sync`, `status`, `clean` | Manage only servd's hosts-file block. |
| `servd doctor` | Check settings, ports, and hostname resolution. |
| `servd version` / `servd --version` | Print version, commit, and build date. |
| `servd` / `servd tui` | Open the interactive dashboard. |

Target selection and primary URLs are described in [Commands and static serving](../commands/). `open`, dashboard links, and ordinary status URLs always use a site's primary hostname; configured fallback aliases remain routable but are not automatically chosen.

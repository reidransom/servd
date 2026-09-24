---
title: Commands and static serving
permalink: /commands/
---

A registered project starts from exactly one command. The public terms are **explicit command**, **repository command**, **resolved command**, and **runtime command**. servd does not use a launcher model.

## Choose a command

An explicit command is stored with a registration. A repository command is the nonblank top-level `cmd` string in `<registered-root>/.servd.toml`. For each next start, servd checks that the registered path exists and is a directory, then selects a nonblank explicit command without reading `.servd.toml`; otherwise it reads only that root file.

```toml
cmd = "bundle exec middleman serve --bind {host} --port {port}"
```

A missing file means no repository command. An unreadable or malformed file, missing or non-string `cmd`, and empty or whitespace-only `cmd` are errors. Unknown fields are ignored. servd never searches parents or infers commands from Procfiles, framework files, package scripts, recipes, directory contents, installed tools, `PATH`, or global rules.

The resolved command substitutes literal `{host}` and `{port}` and exports `HOST` and `PORT`. macOS and Linux execute through `sh`; Windows executes through `cmd.exe`. The runtime command is the concrete command retained for the latest supervised attempt, not desired configuration.

## Register deliberately

```text
servd add <path>
servd add <path> -- <command>...
```

`<path>` must exist and be a directory. Path-only registration requires a valid repository command. The second form needs exactly one path before `--` and at least one argument after it; trailing values are an argument vector, so platform-specific shell quoting preserves them as arguments. `&&` after `--` is not shell syntax. Invoke a shell explicitly when needed:

```sh
servd add . -- sh -c 'first && second'
```

servd assigns the slug and a stable backend port. The dashboard registration form is path-only and keeps its form open with the repository-command error when invalid.

`add` never updates a registration. Replace an explicit command by removing and re-adding; return command ownership to `.servd.toml` by re-adding without trailing command arguments:

```sh
servd rm acme
servd add ~/clients/acme -- npm run dev

servd rm acme
servd add ~/clients/acme
```

A valid `.servd.toml` edit affects the next start but does not restart a healthy process. Use `servd restart acme` as the deliberate cutover. Invalid next configuration leaves an existing process running but reports the site as an error; it remains stoppable.

## Select a target

`up`, `down`, `restart`, `status` (`ls`), `logs`, `open`, `copy`, `which`, and `rm` accept a site slug or a registered root directory path. Lookup checks the exact slug first, then the absolute, cleaned path relative to your working directory. An exact registered path wins over filesystem-identity matches (such as symlinks); ambiguous identity matches require a slug.

```sh
servd status ./docs/
servd restart api ./docs/
servd copy /absolute/project/path
```

A matching slug takes precedence over a relative directory name: `servd which docs` selects the slug `docs`, while `servd which ./docs/` selects the registered directory. Explicit paths do not require `.servd.toml`, never search parents or descendants, and never register a site automatically. An exact registered path remains selectable even if the directory no longer exists, allowing stale registrations to be removed.

Without a target, commands select only a project registered at the current directory when `.servd.toml` is directly present—never a parent. An unregistered current directory containing that file errors with an `add .` hint. Without an inferred target, `status` lists all sites; other site commands require explicit targets or their existing `--all` option.

`up`, `down`, and `restart` accept mixed slug/path lists. All targets must resolve before any site is started, stopped, or restarted. `--all` selects all registered sites, bypasses cwd inference, and cannot be combined with positional targets. Other site commands accept at most one target.

`servd copy` writes the selected site's primary URL to the system clipboard through the terminal's OSC 52 protocol. It uses the live proxy's effective port, emits no separate success text, and does not open a browser.

After target selection, command-resolution and operation errors are isolated per site. Bulk operations attempt all selected sites, report every failure, and exit nonzero if any fail. `status` can show all rows despite an error, while a targeted invalid site exits nonzero. `down` still stops a running site whose next command is invalid.

`servd rm ./docs/` stops and unregisters the selected site; it does not delete the directory or its files. Selection for stopping, log access, and removal does not depend on a valid repository command.

## Serve static files

`servd static` is a foreground HTTP server. It does not register, supervise, detach, or load `config.toml`.

```text
servd static [--host <host>] [--port <port>] [--dir <directory>]
```

It accepts no positional arguments. `--dir` defaults to the current directory. Listener values come from flags, then `HOST`/`PORT`, then `127.0.0.1` and `8080`; host must be nonempty, port must be 1–65535, and root must be readable directory. Invalid input fails before listening.

Static serving is never auto-selected. Set `cmd = "servd static"` in `.servd.toml` or register `servd add ./public -- servd static`. The server serves files and directory-local `index.html` below the resolved root; missing paths and directories without an index are 404. It has no directory listings or SPA fallback. Any dot-prefixed path segment is 403, including `.well-known`; symlinks must resolve inside the root and may not expose a hidden segment.

## Migration

Existing nonblank registration commands remain explicit. Otherwise create a root `.servd.toml` or remove and re-add with an explicit command. Convert a Procfile entry such as `web: bundle exec middleman serve -p $PORT` to `cmd = "bundle exec middleman serve -p $PORT"`; replace former static detection with `cmd = "servd static"`. `launchers.toml` and former custom rules are ignored. Replace any persisted `servd __static` command with `servd static`.

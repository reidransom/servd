# Explicit launch commands

Status: Accepted design; not yet implemented.

This document specifies the breaking removal of Servd's launcher-discovery system. Until the implementation lands, the README describes the behavior of the current release.

## Problem

Servd currently chooses commands from persisted overrides, `.servd.toml`, Procfiles, user-defined launcher rules, and embedded rules for specific tools and project layouts. Resolution depends on project files, installed binaries, rule order, and global customization. The resulting launcher code, configuration, status metadata, tests, and documentation are larger and less predictable than the behavior warrants.

Servd will stop guessing how to run a repository. A registered site must obtain its command from one of two explicit sources:

1. a command supplied while registering that site; or
2. that repository's root `.servd.toml`.

## Goals

- Make command selection explicit, local, and deterministic.
- Remove Procfile handling, launcher rules, built-in framework detection, and global launcher customization.
- Preserve Servd's process supervision, stable port allocation, hostname behavior, shell execution, and host/port injection.
- Isolate invalid sites so they do not prevent unrelated sites from running.
- Preserve an explicit, dependency-free static server without selecting it automatically.
- Give existing auto-discovered sites actionable migration errors.

## Non-goals

- Removing `config.toml`, `sites.toml`, normal CLI flags, or runtime environment variables.
- Removing slug inference, worktree hostname-prefix detection, or port allocation.
- Searching parent directories for repository configuration.
- Automatically converting launcher labels or arbitrary shell commands.
- Adding command-update subcommands or making `servd add` update an existing registration.
- Adding SPA fallback, caching policy, CORS behavior, or framework-specific behavior to the static server.

## Terminology

- **Explicit command**: a command supplied by `servd add <path> -- <command>...` and stored on that site's `sites.toml` entry.
- **Repository command**: the nonblank top-level `cmd` string in `<registered-root>/.servd.toml`.
- **Resolved command**: the selected command after `{host}` and `{port}` substitution.
- **Runtime command**: the concrete command recorded in `state.json` for the latest supervised launch attempt. It is historical runtime state, not desired configuration.

The public term **launcher** is removed. Command sources are named `explicit` and `.servd.toml`.

## Command resolution

Servd MUST resolve a site's next command in this order:

1. Verify that the registered path exists and is a directory.
2. If the site's `sites.toml` `cmd` is nonblank after trimming whitespace, select it with source `explicit`. Do not read `.servd.toml`.
3. Otherwise, inspect exactly `<registered-root>/.servd.toml`:
   - a missing file means no repository command;
   - an unreadable file is an error;
   - malformed TOML is an error;
   - a missing, non-string, empty, or whitespace-only `cmd` is an error;
   - unknown fields are ignored;
   - a valid `cmd` is selected with source `.servd.toml`.
4. If neither source provides a command, return a missing-command error.

Servd MUST NOT inspect parent directories, Procfiles, package scripts, recipes, framework files, directory contents, installed tools, `PATH`, `launchers.toml`, or embedded launcher rules while choosing a command.

A blank legacy `sites.toml` `cmd` is treated as absent, allowing `.servd.toml` to resolve. An explicit command has complete precedence: a malformed lower-priority `.servd.toml` does not affect that site.

Repository commands are resolved again whenever start, restart, status evaluation, `which`, or the TUI command preview needs the next command. A valid edit changes the next launch without restarting the current process. An invalid edit marks the site as an error but does not terminate an already-running process.

## Repository configuration

`.servd.toml` remains a one-field repository-root file:

```toml
cmd = "npm run dev"
```

The command is a shell command string. Existing `{host}` and `{port}` substitution remains available:

```toml
cmd = "bundle exec middleman serve --bind {host} --port {port}"
```

Servd also exports `HOST` and `PORT` to every child process. A project can therefore prefer ordinary environment expansion:

```toml
cmd = "bundle exec rackup --host $HOST --port $PORT"
```

On Windows, commands continue to run through `cmd.exe`; on macOS and Linux, they continue to run through `sh`.

## Registration

### Supported forms

```text
servd add <path>
servd add <path> -- <command>...
```

`<path>` is a repository path, not a requested site name. Existing slug inference remains responsible for the site name. Worktree hostname-prefix detection and unused-port allocation also remain unchanged.

`servd add <path>` succeeds only when `<path>/.servd.toml` contains a valid repository command.

`servd add <path> -- <command>...`:

- requires exactly one repository path before `--`;
- requires at least one command argument after `--`;
- validates that the path exists and is a directory;
- shell-quotes and joins the trailing argument vector using the existing platform-specific behavior;
- stores the resulting command in that site's `sites.toml` entry;
- gives that explicit command precedence over `.servd.toml`.

There is no unnamed root form such as `servd -- <command>`. The `--cmd` flag is removed without an alias or deprecation path.

Arguments after `--` remain an argument vector, not shell syntax. For example, `&&` after `--` is quoted as an argument. Users who need shell operators must explicitly invoke a shell, such as `servd add . -- sh -c 'first && second'` on systems with `sh`.

### Persistence

The explicit command remains beside the site's path, slug, and port in `sites.toml`. It MUST NOT move to `state.json`: runtime entries are deleted by `down`, overwritten by launches and failures, and represent concrete launch history rather than durable intent.

A successful add reports:

- the registered slug and URL;
- source `explicit` or `.servd.toml`; and
- the resolved command.

### Changing commands

`add` remains registration-only and MUST reject an already-registered path. No command-update or command-unset interface is added.

To replace an explicit command:

```sh
servd rm <slug>
servd add <path> -- <new-command>...
```

To remove an explicit override and return to `.servd.toml`:

```sh
servd rm <slug>
servd add <path>
```

### TUI registration

The TUI add form remains path-only. It has no command field and performs no
discovery. Registration succeeds only when the selected repository has a valid
root `.servd.toml`; otherwise the form remains open and shows the repository
configuration error. Supplying an explicit command is a CLI-only workflow.

## Execution

The existing execution contract remains:

- run the command with the registered repository as its working directory;
- execute through the platform shell;
- export `HOST` and `PORT`;
- substitute literal `{host}` and `{port}` before execution;
- preserve platform-specific quoting for commands supplied after `--`;
- detach supervised sites into their existing process-group lifecycle.

Path validation occurs on every resolution, including sites with explicit commands. A missing or non-directory working directory is always an error.

A valid command change never restarts a healthy process automatically. `servd which` shows what the next start will use. Restart is the explicit cutover operation.

## Static server

The hidden `servd __static` command becomes the public, documented foreground command `servd static`. `__static` is removed without an alias.

Static serving is never a fallback. A repository must select it explicitly:

```toml
cmd = "servd static"
```

or:

```sh
servd add ./public -- servd static
```

### Interface

```text
servd static [--host <host>] [--port <port>] [--dir <directory>]
```

Positional arguments are rejected. `--dir` defaults to the current working directory.

Listener settings use this precedence:

1. explicit `--host` and `--port` flags;
2. `HOST` and `PORT` environment variables;
3. standalone defaults `127.0.0.1` and `8080`.

The command MUST NOT load `config.toml`. When supervised, the existing exported `HOST` and `PORT` make `servd static` bind the site's assigned address without extra flags.

The host must be nonempty. The port must be an integer from 1 through 65535. The root must exist, be readable, and be a directory. Invalid arguments or inputs fail before opening the listener and produce a nonzero exit.

`servd static` remains a foreground HTTP server. It does not register, supervise, detach, or mutate a site.

### Serving and containment

The server:

- recursively serves files beneath the selected root;
- serves directory-local `index.html` files;
- returns 404 for missing routes and directories without an index;
- does not generate directory listings;
- does not perform SPA fallback;
- returns 403 for any requested path containing a dot-prefixed segment;
- follows a symlink only when its canonical target remains beneath the canonical root and contains no dot-prefixed segment relative to that root.

Containment MUST be enforced against resolved targets, not only lexical request paths. A visible symlink cannot expose a file outside the root or alias a hidden file. The blanket dot-segment prohibition includes `.well-known`.

## CLI surface

The command surface changes as follows:

- remove `servd launchers`;
- remove `servd add --cmd`;
- add public `servd static`;
- remove hidden `servd __static`;
- keep `servd which <slug>`;
- keep `servd doctor`, but remove checks derived from launcher rules or framework tools.

`servd which <slug>` prints both the public source name and resolved command. Its human output is:

```text
source: explicit
command: npm run dev
```

or:

```text
source: .servd.toml
command: npm run dev
```

Resolution errors produce a nonzero exit.

## Status and lifecycle isolation

A command-resolution failure belongs to one site. It MUST NOT abort evaluation or lifecycle work for unrelated sites.

- An invalid site has `error` status and the red `✕` TUI glyph.
- A running process is not stopped merely because its next-launch configuration becomes missing or invalid.
- An invalid running site still reports `error`; `down` remains functional.
- Restarting such a site explicitly stops the old process and then fails to start the invalid next command.
- `servd up <invalid-site>` and `servd restart <invalid-site>` exit nonzero.
- `servd up --all`, `restart --all`, and `down --all` attempt every selected site, report every operation failure, and exit nonzero if any operation failed.
- Plain `servd status` renders all rows and exits successfully when collection succeeds, even when some sites have `error` status.
- `servd status <invalid-site>` renders the error and exits nonzero.
- Structured status retains per-site `status` and `error` fields.

The TUI applies the same isolation. Bulk actions attempt every site and report both counts, for example `Started 4 sites; 2 failed`. Each failed row retains its own error. A running site whose configuration became invalid must still be stoppable from the TUI.

A missing-command error for an existing registration gives both recovery paths:

```text
no command configured for /path/to/site
create /path/to/site/.servd.toml with a nonblank cmd, or run:
  servd rm site-slug
  servd add /path/to/site -- <command>
```

CLI, TUI, and JSON use the same underlying error; each surface may adapt presentation without changing its meaning.

## Configuration and persisted schema

These files remain supported:

- `$XDG_CONFIG_HOME/servd/config.toml`, or its existing fallback, for Servd settings;
- `$XDG_CONFIG_HOME/servd/sites.toml` for registered sites and site-specific explicit commands;
- `<registered-root>/.servd.toml` for the repository command;
- `$XDG_STATE_HOME/servd/state.json`, or its existing fallback, for latest runtime attempts;
- the existing state log directory.

`launchers.toml` support is removed completely. Servd MUST NOT discover, read, parse, warn about, modify, migrate, or delete an existing file at that path.

The persisted `launcher` field is removed from the site model. Old `launcher` keys are tolerated when reading an existing registry and discarded the next time Servd rewrites it. The label is never used to reconstruct a command.

The public status JSON `launcher` field is removed without a replacement `command_source` field. Runtime `cmd` remains available under its existing live-entry rules. `servd which` is the source-inspection interface.

Launcher labels are also removed from tables, add output, the TUI footer, and the proxy landing page. No replacement launcher metadata is introduced.

## Removed behavior and code

The implementation removes:

- `Procfile.dev` and `Procfile` resolution;
- user launcher rule loading, matching, merging, disabling, and serialization;
- every embedded Jigyll, Jekyll, Hugo, Node, Just, Make, and static detector;
- file, directory, recipe, package-script, dependency, content-regex, binary, and `PATH` predicates used for command selection;
- launcher-tool extraction from `doctor`;
- `{script}`, `{self}`, and rule-specific port-flag expansion;
- the `servd launchers` command;
- launcher metadata persistence and presentation.

Platform shell quoting used by trailing `--` commands remains. Host/port substitution and the reduced two-source resolver remain, but implementation and public names should use command/source vocabulary rather than framework launcher vocabulary.

## Migration

This is a clean breaking cutover. There is no compatibility resolver, warning period, command alias, or automatic launcher conversion.

### Existing explicit sites

Sites whose `sites.toml` entries already contain a nonblank `cmd` continue to use that command. No data migration is needed.

A manually persisted command containing `servd __static` is not rewritten. Replace or re-register it with `servd static`.

### Existing auto-discovered sites

A site with no explicit command:

1. uses a valid root `.servd.toml` if present;
2. otherwise becomes `error` with the missing-command remediation.

The old launcher label is insufficient to reconstruct commands safely and is ignored. Servd never materializes commands from Procfiles, embedded rules, custom rules, project files, or installed tools during migration.

A process already running when the new version starts is left running. If its next command cannot resolve, status shows `error` until configuration is fixed or the process is stopped.

### Common conversions

Repository-owned command:

```toml
# .servd.toml
cmd = "npm run dev"
```

One-site explicit command:

```sh
servd rm acme
servd add ~/clients/acme -- npm run dev
```

Former Procfile:

```text
web: bundle exec middleman serve -p $PORT
```

becomes:

```toml
cmd = "bundle exec middleman serve -p $PORT"
```

Former static detection:

```toml
cmd = "servd static"
```

Custom launcher rules must be converted to an explicit command for each registration or checked into each affected repository as `.servd.toml`.

An existing `launchers.toml` is left untouched and silently ignored.

## Documentation cutover

The implementation change and public README update land together. The updated README must:

- replace the detection-based introduction and quick start;
- replace the launch precedence and custom-rule sections with the two-source contract;
- document `.servd.toml`, trailing `--`, precedence, and remove/re-add workflows;
- document `servd static`, including environment/flag precedence and containment limits;
- remove `--cmd`, `launchers`, Procfile, framework detector, and launcher-tool claims;
- update TUI add behavior and bulk failure reporting;
- remove `launcher` from the agent JSON contract;
- remove `launchers.toml` from the Files list without claiming it is deleted;
- retain platform shell behavior and clarify argument-vector quoting after `--`;
- include the migration examples above.

Until that implementation lands, the current README remains authoritative for released behavior and links to this accepted future specification.

## Acceptance criteria

The cutover is complete only when all of the following are observable:

1. Registration succeeds with a valid `.servd.toml` or a trailing explicit command and fails without either.
2. Explicit commands win without parsing an invalid lower-priority `.servd.toml`.
3. Malformed, unreadable, missing-command, and blank-command repository files produce distinct nonzero errors.
4. No Procfile, framework file, package script, recipe, binary, user rule, or embedded rule can select a command.
5. `launchers.toml` is neither read nor modified.
6. Existing explicit `sites.toml` commands still launch; launcher-only registrations fail with remediation unless `.servd.toml` is valid.
7. One invalid site does not prevent unrelated status evaluation, start, restart, or shutdown work.
8. Status, JSON, TUI, proxy output, and persistence expose no launcher field or label.
9. `servd static` works both directly and under supervision, rejects invalid inputs, emits no directory listings, and confines resolved file targets to its root.
10. `servd launchers`, `--cmd`, and `servd __static` are unavailable.
11. `servd which` reports `explicit` or `.servd.toml` plus the resolved command.
12. README, help output, migration guidance, and behavioral tests describe the same contract.
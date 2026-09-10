# Proxy server in the sidebar

Status: Implemented, reviewed, and verified on 2026-09-09. This document retains the implementation requirements and records the approved lifecycle correction.

## Decision

Add a selectable `servd` row before the registered sites in the left sidebar. Use the existing table, selection styling, and status-glyph column. Selecting the proxy shows its log in the right pane and its landing URL in the footer.

Move proxy status out of the title row rather than displaying it twice. Keep the title and both pane borders.

Interaction choices:

- The proxy is the first row and the initial selection, whether running or stopped.
- It participates in normal table scrolling. This change does not introduce a separate sticky section.
- With no registered sites, the proxy remains selectable. Show the add-site hint below it without making the hint selectable.
- Site order, site status evaluation, and site lifecycle behavior stay unchanged.

Example layout, with the proxy selected:

```text
servd
╭──────────────────────╮╭──────────────────────────────────╮
│ ● servd              ││ proxy log                   LIVE │
│ ○ acme               ││ <existing proxy log output>      │
│ ● blog               ││                                  │
╰──────────────────────╯╰──────────────────────────────────╯
→ http://servd.localhost:8080/
```

The example URL is illustrative. Render the effective bind host and listener port, not a hard-coded address.

## Implementation baseline

- `internal/tui/statuses.go` already reads `proxy.Running` and applies `proxy.EffectiveSettings`, but builds table rows only for registered sites.
- `internal/tui/model.go` maps table rows through `slugs`, resolves selection through `selectedSite`, and tails `supervisor.LogPath(m.logSlug)`.
- The `p` handler already starts and stops the proxy asynchronously, respects `busy`, and refreshes state after completion.
- `internal/proxy/control.go` defines the reserved runtime key `proxy.Slug`, records the proxy log path, and provides the lifecycle operations. No new proxy backend is needed.
- `View` currently shows proxy status and its landing URL in the title. Its empty-registry branch replaces the table with an add-site hint.

## Implementation sequence

### 1. Add the proxy row and distinguish it from sites

In `internal/tui/statuses.go`:

- Prepend a plain-text row with label `servd` and glyph `●` when `proxy.Running` is true, otherwise `○`.
- Include `proxy.Slug` in the row-key list before the site slugs. Keep the visible label separate from the reserved runtime key.
- Continue evaluating only registered sites with `supervisor.Evaluate`. Do not insert a synthetic site into the registry or site-status map.
- Preserve the existing live-port calculation and background refresh cadence. Do not add a second health poll or new proxy health states.

In `internal/tui/model.go`:

- Centralize retrieval of the selected row key. Make `selectedSite` explicitly return nil for the proxy row.
- Use the selected key for log selection rather than deriving it exclusively from `selectedSite`.
- Preserve selection by key across status refreshes. If the selected site disappears, clamp to a surviving nearby row, falling back to the proxy when no sites remain. Keep successful rename selection on the renamed site.
- Synchronize the log selection immediately after applying a new row snapshot so the highlighted row and log pane cannot disagree until the next tick.

Reuse the existing row-key slice and reserved proxy key. Do not introduce a generic server interface or change persisted configuration. The model retains the complete rows and keys while the table renders a bounded visible window. Its explicit row offset is shared by navigation and mouse hit-testing because Bubbles does not expose the table's internal scroll offset.

### 2. Render proxy logs and details

In `internal/tui/model.go`:

- Reuse `supervisor.LogPath(proxy.Slug)` and the existing viewport loading, follow, and pause behavior.
- Give the proxy pane a descriptive `proxy log` header. Do not call `launcher.Resolve` for it or display an invented site launch command.
- Keep the existing command header and resolution errors for site selections.
- When proxy logs do not exist, show a proxy-specific empty-log message. Keep old logs readable when the proxy is stopped.
- Share one private landing-URL helper across the proxy footer, browser-open, clipboard, and QR actions. Use `Settings.SiteURL` with the `servd` label to follow the effective primary suffix and runtime port, including the trailing slash, instead of displaying the listener's bind address. DNS must resolve this hostname to the proxy.
- Show the landing URL for a selected proxy even while stopped. Its row glyph communicates that the listener is not running.
- Remove the old title-row proxy indicator. Preserve transient action errors in the footer.

### 3. Route shortcuts safely

| Key | Proxy selected | Site selected |
| --- | --- | --- |
| `s` | Start or stop the proxy | Existing site start/stop |
| `o` | Open the proxy landing URL | Open the site's primary URL |
| `p` | No action | No action |
| `r`, `R`, `d` | No action | Existing rename, restart, remove |
| `S` | Start/stop all registered sites only | Same |
| `a` | Open the add-site form | Same |

Move the existing proxy-toggle implementation into `s` handling for the proxy row and remove the global `p` handler entirely. Preserve the asynchronous action flow, busy guard, error reporting, and post-action refresh. Do not retain `p` as an alias or add proxy restart in this change.

Make the help bar selection-aware so it does not advertise rename, restart, or removal for the proxy. Remove `p` from the help bar and README key reference; selecting the proxy and pressing `s` is the dashboard's proxy start/stop workflow. Opening a URL must report a browser-launch error rather than claiming success when the launch fails.

### 4. Keep navigation and layout correct

In `internal/tui/model.go`:

- Render the table even when the registry is empty. Fit the nonselectable add-site hint within the sidebar's content height.
- Keep the existing sidebar width, one-space glyph separation, and ANSI-safe post-truncation glyph styling.
- Verify keyboard navigation and mouse-wheel movement between proxy and site rows.
- Update mouse hit-testing from the actual rendered content origin. `firstRowY` currently describes a table header that `sidebarTableView` removes; do not blindly add one to that constant.
- Account for the sidebar border, the table's scroll offset, and non-row areas when translating a click to a selection. A click on the proxy must never select or operate on the first site.
- Keep both panes aligned after resize and help toggles. Ignore clicks beneath either modal.

### 5. Verify behavior and update documentation

Update affected tests in `internal/tui/model_test.go` and `internal/tui/statuses_test.go`:

- Site action tests must explicitly select their target site instead of relying on row zero.
- Replace the title-location proxy assertion with coverage of the selected proxy's landing URL and effective runtime port. Do not retain exact old title wording as a contract.
- Adapt status-row and error-glyph checks to locate the intended site rather than assuming its numeric index.

Keep focused regression coverage for these plausible failures:

- An empty registry hides the proxy or prevents selection.
- Refresh, removal, or rename switches the selected row without switching its logs and details.
- A site-only shortcut mutates proxy state or a neighboring site while the proxy is selected.
- Bulk site operations include the proxy.
- The removed `p` shortcut still starts or stops the proxy from either pane or selection.
- Clicking after scrolling selects a different row from the one displayed under the pointer.

Run `go test ./internal/tui`, then `go test ./...` and `go build ./...` once the changes are integrated.

Launch the newly built TUI in a real terminal with isolated `XDG_CONFIG_HOME` and `XDG_STATE_HOME`, a loopback bind address, and an available unprivileged port. Exercise:

1. No sites, proxy stopped: the proxy row and add-site hint are visible.
2. Start through `s`, observe the running glyph, fetch the displayed landing URL, and inspect actual proxy logs.
3. Stop through `s` with the proxy selected, observe the stopped glyph, and confirm site state did not change.
4. Add sites and switch selection with keys and mouse. Confirm logs, headers, footer URLs, and site-only actions follow the correct target.
5. Use enough sites to scroll; click visible rows after scrolling, resize the terminal, and toggle help.
6. Confirm proxy selection cannot rename, restart, or remove a site, and that `S` leaves the proxy untouched.
7. Press `p` with both proxy and site selections, from each pane, and confirm it does not change proxy state.

After the smoke check, stop temporary processes and remove temporary files. Update the dashboard description and TUI key reference in `README.md`. Record the actual verification results in this plan when implemented.

## Out of scope

No changes to proxy routing, hostname configuration, CLI commands, registry format, or automatic proxy startup. No sticky subsection, proxy restart shortcut, new log transport, or new health model. The only process-supervision change is the approved worker-reaping fix below.

## Approved lifecycle correction

The terminal smoke test exposed an existing defect in `internal/proxy/worker_unix.go`: `spawnWorker` released its child without waiting for it. A proxy stopped by its still-running TUI parent became a zombie. `StopBackground` reported that it survived forced termination and retained stale running state.

The user approved including the reaping fix. `spawnWorker` now waits asynchronously for the child, matching the existing site-supervisor approach. Its detached process group still survives the parent exiting first. No lifecycle interface or routing behavior changes.

`TestBackgroundProxyStopsWhileParentRemainsAlive` exercises the real inherited-listener worker through `StartBackground` and `StopBackground`. It failed before the fix with the forced-termination error and passed afterward.

## Verification results

- Focused TUI regressions passed for empty-registry proxy selection, runtime-port URLs, refresh/removal selection, sorted rename selection, bind errors, disabled actions, browser-launch errors, and scrolled mouse clicks.
- A real TUI ran in an isolated tmux terminal with loopback binding and an unprivileged proxy port. Proxy start, HTTP 200 from the displayed landing URL, stop, and restart passed in the same persistent dashboard.
- Actual proxy reload errors appeared in the log pane and remained readable after stopping. Site logs paused while new output arrived and resumed following when scrolled to the bottom.
- Both add-site modals and site start, restart, rename, and removal were exercised. Rename preserved selection after sorting; removal selected a surviving row. Mouse clicks beneath the rename modal had no effect.
- `S` started and stopped the two fixture sites without changing the proxy PID. `p` did nothing with proxy/site selections and either pane focused. Proxy `r`, `R`, and `d` stayed inert.
- With 26 registered sites, keyboard scrolling, mouse selection after scrolling, and wheel movement across proxy/site rows passed. Pane alignment held at 120×24 and 80×16 with help shown and hidden.
- An isolated `xdg-open` recorder received the exact proxy and site URLs. This verified browser dispatch without opening the user's browser.
- Sequential coding-standards and specification reviews completed. Review corrections removed wording-only lifecycle assertions, exercised command-cache refresh through the rendered dashboard, and fixed rename selection when a periodic refresh arrives before action completion.
- Final validation passed: `go test -race ./...`, `go vet ./...`, and `go build ./...`. The final built executable also passed proxy start/HTTP/stop and running-site rename/stop checks.
- All smoke processes were stopped, the tmux session exited, and temporary binaries, configuration, sites, logs, and browser-dispatch fixtures were removed.

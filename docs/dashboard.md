---
title: Dashboard and TUI
permalink: /dashboard/
---

Run `servd` or `servd tui` to open the dashboard. It is a split view: the left pane lists the proxy first, then registered sites; the right pane follows the selected server log. The proxy is selected initially, including with an empty registry. Site rows retain registry ordering and all selectable rows scroll together.

## Read rows and logs

The proxy row is labeled `servd`. Site glyphs are `○` stopped, `◐` starting, `●` running, and red `✕` error; select an error row to read its concise reason. The selected proxy shows a `proxy log` header; a site shows its next command. Existing logs remain readable after a process stops.

The footer is a URL, not the listener address: for the proxy it is the landing URL, and for a site it is that site's primary URL. Both use the effective primary suffix and active proxy port.

Moving selection changes the live log. Focus the log with `tab`; scrolling away from its bottom pauses following, and returning to the bottom resumes it.

## Keyboard reference

| Key | Action |
| --- | --- |
| `↑` / `↓` | Move selection or scroll the focused log |
| `tab` | Move focus between list and log |
| `s` | Start or stop the selected site; with proxy selected, start or stop the proxy |
| `r`, `R`, `d` | Rename, restart, or remove the selected site only |
| `S` | Start or stop all sites; never the proxy |
| `a` | Add a repository path |
| `o` / `c` | Open or copy the selected primary or landing URL |
| `Q` | Display a QR code for the selected URL |
| `h` | Show or hide help |
| `q` | Quit |

The former global `p` proxy shortcut is removed. Proxy-only and site-only actions remain inert for the wrong row.

## Mouse and selection

Click a row to select it; wheel scrolling works in scrolled lists and panes. Drag in a pane to highlight text; releasing sends the selection through OSC 52 clipboard delivery. Selection is contained by its pane and clears on the next interaction or resize. Modals shield the content beneath them.

## QR codes

`Q` creates a QR code locally for the selected proxy landing URL or site's primary URL, fixed while the overlay is open. `Esc`, `Q`, or `q` closes it; `Ctrl-C` quits the dashboard. servd reports minimum terminal dimensions instead of clipping a code.

The QR code includes the active proxy port but does not configure DNS or expose the proxy. A scanning device must reach the proxy and resolve the URL's hostname; its own `.localhost` and loopback addresses are not the servd machine.

For primary suffix, fallback routing, and remote-access implications, see [Proxy ports and hostnames](../proxy-hostnames/).

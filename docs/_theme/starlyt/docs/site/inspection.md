---
title: Manual inspection
permalink: /inspection/
---

Use this site as a real reader would. These suggestions help you find useful examples; they are not a mandatory test matrix, an automated suite, or a request to capture a set of reference screenshots.

## Start with the installed consumer

Follow the repository README to create the documentation consumer through Jigyll's theme installer, copy the maintained documentation content, and serve it. Edit source pages under `docs/site/`, not the disposable consumer.

The installed theme is a committed snapshot. Update its Git checkout after committing theme changes, then restart the preview. This avoids making a direct source build look like proof that consumer installation works.

## Read at different widths

Start with a wide window, then narrow it. The sidebar becomes Menu below 50rem; the right-hand outline becomes an **On this page** disclosure below 72rem. With the default 16px browser font these correspond to 800 and 1152 CSS pixels.

You do not need an exhaustive viewport matrix. Look at transitions that matter to the change: a long navigation label, the [Markdown table and image](../markdown/), or the [long code line](../code/). The article should remain readable, and content that scrolls locally should not force the whole page sideways.

## Try navigation with the keyboard

Reload and press Tab to reveal **Skip to content**. Activate it, then explore the article's links. Visible focus should tell you where the keyboard will act.

At a narrow width, open Menu. Use Tab and Shift-Tab, open and close a group, then press Escape. Focus should return to the menu button. Try resizing while the menu is open: after returning to desktop layout, the overlay should not leave reading content inert.

The Guides → Reference group and the deliberately long label for this page are live navigation examples. Collapse a group and follow another page to inspect session persistence without inventing another fixture.

## Follow headings

Open [Headings and table of contents](../headings/). It has repeated Review headings, an explicit HTML heading ID, and a deeper heading outside the H2/H3 outline.

Follow the two Review entries separately. Try an anchor URL, reload at that fragment, and scroll through the article. Watch where the heading lands relative to the fixed header and which section is indicated in the outline.

On a narrow window, open the outline disclosure and choose a section. Escape and clicking outside provide other ways to close it. These observations concern visible behavior, not the enhancement's internal fields.

## Copy real examples

Open [Code blocks](../code/) and copy the Python example into a plain text editor. Look at indentation, blank lines, and whether the copied text excludes the button's label. Scroll the long JSON line and reach its copy control with the keyboard.

Copying needs JavaScript and browser Clipboard API support. HTTPS or a trustworthy local origin is appropriate. If permissions are denied, feedback should report failure rather than success. Where the API is missing entirely, normal text selection remains available without a misleading copy button.

## Try search

Open Search or press Control+K (Command+K on Apple devices), then confirm the input receives focus. Search a title, a heading, and a code term. A heading result links directly to its existing fragment. Press Escape to return focus to Search.

Temporarily block `assets/search-data.json` in developer tools to see the visible failure state, then use Retry after unblocking it. With JavaScript disabled, Search remains disabled and cannot receive keyboard focus.

## Change color preferences

Use the selector to choose Light, then navigate to another page. Try Dark and Auto too. With Auto selected, change the system color preference or use browser developer tools to emulate it; the effective palette should follow the system rather than retain an explicit choice.

Inspect text, code, focus indicators, and native controls in both palettes. The example image owns its colors and does not recolor merely because the page palette changes.

## Inspect the fallback when useful

Disable JavaScript in the browser and reload. Read the article, follow links, and use native navigation disclosures. The palette is dark; generated TOC, heading-anchor controls, copying, and mode selection are absent. Re-enable JavaScript afterward.

If storage is unavailable, reading should still work. A selector choice made in memory need not survive navigation. You do not need to reproduce every storage or permission condition for every documentation edit.

## Check a deployment prefix when relevant

The [deployment guide](../deployment/) shows how to build and mount the same consumer at `/docs/` without changing generated files. When changing links or deployment configuration, follow a sidebar link, an article link, and the image at that prefix as well as checking theme assets load.

## Describe an observation precisely

When reporting a problem, include the page route, what you did, what you expected, what happened, and relevant browser/window or permission settings. A screenshot can help explain a visual issue, but no screenshot baseline or evidence-report format is required.

Manual inspection can reveal defects. It cannot establish complete visual parity or interaction parity with stock Starlight, and it does not extend support to every browser or physical device.

---
title: Color modes and fallback behavior
permalink: /color-modes/
---

The **Select theme** control offers Dark, Light, and Auto. On a wide screen it is in the header; on a narrow screen it moves into Menu. It is the same control in both places, not two independently stored preferences.

## Choose a preference

| Preference | Effective palette | Persistence |
| --- | --- | --- |
| Dark | Dark regardless of the system setting | Saved locally when storage is available |
| Light | Light regardless of the system setting | Saved locally when storage is available |
| Auto | Follows the system color preference | Clears the explicit stored choice |

Try Light and then follow [Code blocks](../code/). Your explicit preference should survive navigation. Switch back to Auto when you want the system preference to decide.

### Auto follows live changes

With Auto selected, changing the operating system's appearance should change the site's effective palette without a reload. A browser's developer tools may also let you emulate `prefers-color-scheme` for manual inspection. With Dark or Light selected, the explicit choice wins over the system setting.

Auto is a preference, not a third palette. The document ultimately uses either the dark or light colors.

## Read without storage

The stored preference uses the `starlight-theme` local-storage key. Invalid values fall back to Auto. If storage access is unavailable, the page still initializes from the system preference and lets you change its in-memory preference.

An in-memory choice is not a promise of persistence after navigation or reload. This distinction matters in restricted browsing contexts: a readable page and working selector do not imply that the browser permitted a saved setting.

## Read without JavaScript

JavaScript-disabled reading uses the default **dark** palette. There is no mode selector, generated table of contents, added heading-anchor control, or copy button. These controls are absent instead of being presented as nonfunctional widgets.

The article, links, images, server-highlighted code, and native navigation disclosures remain. The mobile menu relies on native popover support; JavaScript adds focus handling and persistence to that native behavior.

You can disable JavaScript in browser settings and reload to inspect this fallback. Re-enable it afterward to restore the enhancements. This is a suggested manual check, not a maintained automated test.

## Initialization and responsiveness

The color-mode asset executes in the document head before styles and body content, so the effective palette can be set early. Once the DOM is ready, it initializes the selector and places it in the header or menu at the 50rem navigation transition.

Palette changes affect theme colors, not the content of author-supplied images. See [Markdown](../markdown/) for an image that retains its own colors in both modes.

For source ownership, see [Theme internals](../internals/). For ways to inspect keyboard focus and mobile layout, see [Manual inspection](../inspection/).

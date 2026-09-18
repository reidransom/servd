---
title: Navigation
permalink: /navigation/
---

The sidebar is an ordered list owned by the consumer. This site's **Guides → Reference** group demonstrates nested navigation, and the long manual-inspection label demonstrates wrapping without a separate fixture page.

## Define links and groups

Each entry has a text `label` and either a root-relative `link` or a nested `items` list. Do not supply both. For example:

```yaml
navigation:
  - label: Overview
    link: /
  - label: Guides
    items:
      - label: Getting started
        link: /getting-started/
      - label: Reference
        items:
          - label: Markdown
            link: /markdown/
          - label: Code blocks
            link: /code/
```

These routes are real pages in this documentation site. Follow [Markdown](../markdown/) or [Code blocks](../code/) and look for the corresponding current-page indication in the sidebar.

### Match the public route

Use the page's exact permalink, including its trailing slash. Navigation compares the page URL with the configured link to mark the current page. A navigation entry does not create its destination page.

Navigation links exclude `baseurl`: configure `/markdown/`, not `/docs/markdown/`, even when deploying at `/docs`. The theme adds the prefix when rendering the link. Article links use a different authoring convention: make them relative to the page's public route.

### Keep labels readable

Labels are escaped text, not HTML. Their order comes from configuration, not alphabetical sorting or filename order. Long labels can wrap. The manual-inspection entry is deliberately verbose so that this behavior is visible in normal use.

### Add a link badge

Link entries may add a `badge` mapping with escaped plain-text `text`, a documented badge `variant`, and a documented `size`. Badge text participates in the link name unless an `aria_label` already contains equivalent context; in that case the redundant badge is hidden from assistive technology. Group labels do not receive badges. See the [badge guide](../badges/) for the complete interface.

## Use groups

Select a group heading to collapse or expand it. Groups are native disclosures and initially open. With JavaScript, their state and the sidebar scroll position are saved for the browser session. Changing the navigation structure invalidates that saved arrangement.

Try closing **Reference**, navigating to another page, and opening it again. If session storage is unavailable, the disclosures still work; persistence is an enhancement, not a prerequisite for navigation.

## Use the mobile menu

Below 50rem—800 CSS pixels with the default 16px font setting—the sidebar becomes a **Menu** popover. The color selector moves into the same menu instead of creating a second selector.

Open Menu, follow a link, and reopen it. With JavaScript enabled, opening the menu makes covered content inert and moves focus into navigation. Escape closes the menu and returns focus to its button. Tab and Shift-Tab cycle through the open menu controls and its button.

### Resize while open

Try widening the window while the menu is open. At the desktop transition, the overlay closes and the normal sidebar takes over. Narrow the window again: the article should remain available, without a stale overlay blocking it.

The menu uses the browser's native popover support. Without JavaScript, native links, disclosures, and popover navigation remain; the added focus handling and persistence are not available.

## Skip repeated navigation

Reload a page and press Tab to reveal **Skip to content**. Activate it to move to the main reading area instead of traversing every navigation entry. Keep using Tab to explore visible focus indicators and heading links.

For suggestions rather than a mandatory test sequence, see [Manual inspection](../inspection/).

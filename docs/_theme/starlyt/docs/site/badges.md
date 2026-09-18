---
title: Badges
permalink: /badges/
---

Badges are escaped, noninteractive plain-text labels. Their text and shape remain meaningful without relying on color.

## Variants

{% include components/badge.html text="Default" variant="default" size="medium" %} {% include components/badge.html text="Note" variant="note" size="medium" %} {% include components/badge.html text="Tip" variant="tip" size="medium" %} {% include components/badge.html text="Caution" variant="caution" size="medium" %} {% include components/badge.html text="Danger" variant="danger" size="medium" %} {% include components/badge.html text="Success" variant="success" size="medium" %}

Each variant uses a fixed decorative icon from the frozen public registry: `star`, `information`, `rocket`, `warning`, `error`, or `approve-check-circle`. The escaped text and badge shape still carry the label when icons or color are unavailable.

Use the include directly in prose. `text` is escaped plain text; raw HTML and Markdown are not accepted:

```liquid
{% raw %}{% include components/badge.html text="Beta!" variant="caution" size="small" %}{% endraw %}
```

Punctuation and markup-like text stay literal: {% include components/badge.html text="Ready: <strong>yes</strong> & safe!" variant="success" size="medium" %}

## Sizes

{% include components/badge.html text="Small" variant="default" size="small" %} {% include components/badge.html text="Medium" variant="default" size="medium" %} {% include components/badge.html text="Large" variant="default" size="large" %}

An invalid variant or size visibly falls back to `default` and `medium`: {% include components/badge.html text="Fallback" variant="unknown" size="huge" %}

A deliberately long badge wraps instead of widening the page: {% include components/badge.html text="A long release-status label with punctuation: alpha, beta, and release candidate!" variant="note" size="large" %}

## Release status
{% include components/badge.html text="Beta!" variant="caution" size="small" %}

Place a heading badge as a standalone include on the source line immediately after the heading. The badge is styled beside the authored heading text while remaining outside its semantic label. The heading ID, anchor, and table-of-contents label remain **Release status**.

## Navigation badges

A link entry accepts one optional `badge` mapping with the same `text`, `variant`, and `size` values:

```yaml
navigation:
  - label: Badges
    link: /badges/
    badge:
      text: New!
      variant: success
      size: large
```

Without `aria_label`, visible badge text participates in the link's accessible name. If an explicit `aria_label` already includes equivalent context, Starlyt uses that name and hides the redundant badge text from assistive technology:

```yaml
- label: Tabs
  link: /tabs/
  aria_label: Tabs, stable
  badge:
    text: Stable
    variant: note
    size: small
```

The visible link label and its badge remain separate from the destination page title. Group labels do not accept badges. Badges are static labels and add no keyboard stop.

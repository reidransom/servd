---
title: Link buttons
permalink: /link-buttons/
---

Link buttons are visually prominent anchors for navigation. They do not represent state-changing button actions.

## Variants

{% include components/link-button.html text="Primary action" href="/getting-started/" %}
{% include components/link-button.html text="Secondary action" href="/deployment/" variant="secondary" %}
{% include components/link-button.html text="Minimal action" href="#destinations" variant="minimal" %}

`variant` accepts `primary`, `secondary`, or `minimal` and defaults to `primary`:

```liquid
{% raw %}{% include components/link-button.html
  text="Get started"
  href="/getting-started/"
  variant="primary"
%}{% endraw %}
```

`text` and `href` are required. Text is escaped plain text, not Markdown or raw HTML. Missing required values and unsupported destinations omit the link.

## Icons

Set `icon` to a name from the [public icon registry](/icons/). `icon_position` accepts `leading` or `trailing` and defaults to `leading`. Icons are decorative; the escaped text remains the link's complete accessible name. An unknown icon is omitted without removing the link text.

{% include components/link-button.html text="Leading rocket" href="/pages/" variant="primary" icon="rocket" %}
{% include components/link-button.html text="Trailing arrow" href="/cards/" variant="secondary" icon="right-arrow" icon_position="trailing" %}
{% include components/link-button.html text="Unknown icon keeps this text" href="/badges/" variant="minimal" icon="not-in-the-registry" %}

Invalid variants fall back to `primary`, and invalid icon positions fall back to `leading`:

{% include components/link-button.html text="Fallback options with literal <markup> & text" href="/tabs/" variant="loud" icon="star" icon_position="middle" %}

## Destinations and new windows

Root-relative internal paths pass through `relative_url`. Fragments, email, telephone, and absolute HTTP or HTTPS destinations remain unchanged. Other relative paths and schemes are unsupported and omitted.

{% include components/link-button.html text="Nested local destination" href="/guides/nested/example/" variant="secondary" %}
{% include components/link-button.html text="Page fragment" href="#variants" variant="minimal" %}
{% include components/link-button.html text="Email documentation" href="mailto:docs@example.com" variant="minimal" %}
{% include components/link-button.html text="Telephone support" href="tel:+15550100" variant="minimal" %}
{% include components/link-button.html text="External in this window" href="https://example.com/docs" variant="secondary" %}
{% include components/link-button.html text="Explicit new window" href="https://example.com/project" variant="secondary" icon="external" icon_position="trailing" new_window=true %}

`new_window=true` explicitly adds `target="_blank"` and `rel="noopener noreferrer"`. External URLs do not infer new-window behavior.

## Wrapping and omission fallbacks

Adjacent actions wrap when their combined width exceeds the content column:

{% capture adjacent_actions %}
{% include components/link-button.html text="A deliberately long primary action label that wraps without widening the page" href="/getting-started/" %}
{% include components/link-button.html text="A neighboring secondary action with enough words to wrap onto another row" href="/deployment/" variant="secondary" %}
{% include components/link-button.html text="Small final action" href="#icons" variant="minimal" %}
{% endcapture %}
<div class="hero-actions">{{ adjacent_actions | strip_newlines }}</div>

The following invalid fixtures intentionally render no links:

{% include components/link-button.html text="Missing destination" %}
{% include components/link-button.html href="/getting-started/" %}
{% include components/link-button.html text="Unsupported relative destination" href="relative/path" %}
{% include components/link-button.html text="Unsupported script destination" href="javascript:alert(1)" %}

---
title: Cards and grids
permalink: /cards/
---

Starlyt provides separate includes for rich-content cards, responsive card grids, and single-target link cards. Their inputs are trusted consumer content; they are not Astro components or an HTML-sanitization boundary.

## Rich cards

Capture Markdown, pass it to `card.html`, then capture the rendered cards for `card-grid.html`:

{% capture card_intro %}
A card body supports **Markdown**, [ordinary links](../getting-started/), and lists:

- one useful detail
- another useful detail
{% endcapture %}
{% capture card_code %}
Wide code scrolls inside the card rather than widening the page:

```json
{"component":"card","content":"a deliberately long value that remains locally scrollable inside a narrow responsive grid column"}
```
{% endcapture %}
{% capture card_short %}
Short content makes unequal card lengths visible without forcing filler copy.
{% endcapture %}
{% capture card_long %}
This deliberately long body demonstrates natural wrapping across several lines. Cards keep their content readable when neighboring cards contain different amounts of material.
{% endcapture %}
{% capture standard_cards %}
{% include components/card.html title="Rich Markdown body" icon="rocket" content=card_intro %}
{% include components/card.html title="A deliberately long card title that wraps safely" icon="not-in-the-frozen-registry" content=card_code %}
{% include components/card.html title="Short card" content=card_short %}
{% include components/card.html title="Unequal body length" content=card_long %}
{% endcapture %}
{% include components/card-grid.html content=standard_cards %}

The exact authoring shape is:

```liquid
{% raw %}{% capture body %}
Markdown with **formatting**, lists, links, and code.
{% endcapture %}
{% capture cards %}
{% include components/card.html title="Escaped plain-text title" icon="rocket" content=body %}
{% endcapture %}
{% include components/card-grid.html content=cards stagger=false %}{% endraw %}
```

`title` is required escaped plain text. `icon` optionally accepts a frozen public icon name; unknown names omit the SVG. `content` is passed through `markdownify` and therefore follows the consumer's trusted Markdown/raw-HTML policy. Cards add no navigation target of their own. A card inside a link card and nested card grids are unsupported.

## Staggered grid

Set the literal Boolean `stagger=true` to offset alternating cards at the 50rem grid transition. Omitted, false, and invalid values use the standard grid.

{% capture staggered_cards %}
{% include components/card.html title="First" content=card_short %}
{% include components/card.html title="Second, offset at wider sizes" content=card_long %}
{% include components/card.html title="Third" content=card_intro %}
{% include components/card.html title="Fourth, also offset" content=card_code %}
{% endcapture %}
{% include components/card-grid.html content=staggered_cards stagger=true %}

## Link cards

A link card is one anchor covering one card. Its title and optional description are escaped plain text; rich or interactive children are intentionally unsupported.

{% capture link_cards %}
{% include components/link-card.html title="Install Starlyt" href="/getting-started/" description="Follow the consumer installation path." %}
{% include components/link-card.html title="Read the deployment guide with a deliberately long title" href="/deployment/" %}
{% include components/link-card.html title="Email the documentation team" href="mailto:docs@example.com" description="Mail and telephone destinations remain unchanged." %}
{% include components/link-card.html title="Frozen Starlight source" href="https://github.com/withastro/starlight" description="Absolute HTTP(S) destinations remain external." %}
{% endcapture %}
{% include components/card-grid.html content=link_cards %}

```liquid
{% raw %}{% include components/link-card.html
  title="Installation"
  href="/getting-started/"
  description="Start with a working consumer."
%}{% endraw %}
```

Root-relative internal URLs pass through `relative_url`. Fragments, `mailto:`, `tel:`, and absolute HTTP(S) URLs remain unchanged. Missing required values and unsupported schemes omit the link card. Nested grids and interactive card children are not supported.

---
title: Callouts
permalink: /callouts/
---

Callouts present supplementary notes, tips, cautions, and dangers as labeled asides. They are static content, not alerts.

## Variants

{% capture note_body %}
Use a note for useful context that does not change the reader's task.
{% endcapture %}
{% include components/callout.html content=note_body %}

{% capture tip_body %}
A tip can contain [a useful link](/markdown/), a list, and inline `code`:

- Keep the advice specific.
- Keep the next action clear.
{% endcapture %}
{% include components/callout.html content=tip_body type="tip" %}

{% capture caution_body %}
Review this step before continuing.
{% endcapture %}
{% include components/callout.html content=caution_body type="caution" %}

{% capture danger_body %}
Do not share credentials or destructive commands without reviewing them.
{% endcapture %}
{% include components/callout.html content=danger_body type="danger" %}

`type` accepts `note`, `tip`, `caution`, or `danger`. An omitted or invalid type falls back to `note` so authored content is retained.

## Capture and include Markdown

Capture the Markdown body, then pass that capture as `content`. The include applies `markdownify`:

{% raw %}
````liquid
{% capture deployment_tip %}
Check the generated URL before deployment:

```sh
jigyll build --baseurl /docs
```

> A preview build should use the same hosting prefix.
{% endcapture %}
{% include components/callout.html content=deployment_tip type="tip" title="Before deployment" %}
````
{% endraw %}

{% capture deployment_tip %}
Check the generated URL before deployment:

```sh
jigyll build --baseurl /docs
```

> A preview build should use the same hosting prefix.
{% endcapture %}
{% include components/callout.html content=deployment_tip type="tip" title="Before deployment" %}

Paragraphs, links, lists, inline code, fenced code, and ordinary blockquotes are supported. Nested callouts are unsupported. An empty or whitespace-only capture produces no component.

## Titles and icons

Default titles are `Note`, `Tip`, `Caution`, and `Danger`. `title` replaces that label with escaped plain text; it does not accept Markdown or raw HTML. `icon` accepts a name from the [public icon registry](/icons/). An omitted or unknown icon uses the selected type's default.

{% capture custom_body %}
The custom title and icon do not change the aside semantics.
{% endcapture %}
{% include components/callout.html content=custom_body type="tip" title="Markup stays literal: <mark>safe</mark> & readable" icon="star" %}

{% capture fallback_icon_body %}
The unknown icon falls back to the caution warning icon.
{% endcapture %}
{% include components/callout.html content=fallback_icon_body type="caution" icon="not-in-the-registry" %}

{% capture invalid_type_body %}
This invalid type renders as a note with the note title and information icon.
{% endcapture %}
{% include components/callout.html content=invalid_type_body type="urgent" %}

{% capture long_title_body %}
The title and this deliberately wide code block remain bounded by the callout and scroll locally when needed.

```text
https://example.test/a-deliberately-long-unbroken-path/that-proves-wide-code-does-not-widen-the-page/beyond-the-viewport
```
{% endcapture %}
{% include components/callout.html content=long_title_body type="danger" title="A deliberately long danger title with markup-like <characters> and enough words to wrap at narrow widths" %}

{% capture empty_body %}   {% endcapture %}
{% include components/callout.html content=empty_body type="danger" title="This must not render" %}

## Ordinary blockquotes

Callouts do not rewrite blockquotes. A blockquote outside a captured callout keeps the theme's ordinary markup and presentation:

> This remains an ordinary blockquote, not a callout.

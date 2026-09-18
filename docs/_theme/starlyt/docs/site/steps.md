---
title: Steps
permalink: /steps/
---

Starlyt decorates one captured Markdown ordered list as instructional steps. The server output remains an ordinary ordered list, and the circles and guides are CSS decoration rather than copied or accessible text.

## Rich instructional steps

{% capture setup_steps %}
1. ### Prepare the project

   Read the [installation guide](../getting-started/) before changing files.

   - Keep source and generated output separate.
   - Commit the lockfile when dependencies change.

2. ### Build the documentation

   Run the build from the consumer directory:

   ```sh
   jigyll build --source documentation --destination documentation/_site --trace --future --unpublished --verbose-output-that-stays-inside-the-code-frame
   ```

3. Review the generated site and follow its links with a keyboard.
{% endcapture %}
{% include components/steps.html content=setup_steps %}

Capture one ordered list whose authored markers begin at `1.`, then pass it to the include:

```liquid
{% raw %}{% capture instructions %}
1. Prepare the source.
2. Build the site.
3. Review the output.
{% endcapture %}
{% include components/steps.html content=instructions %}{% endraw %}
```

Rich items may contain paragraphs, headings, links, ordinary nested lists, and fenced code. A nested ordered or unordered list keeps its native list semantics and does not become another Steps component. Nested Steps components are unsupported.

## Start at eight

Use the optional positive integer `start` parameter for visible numbering instead of changing the Markdown source markers. This fixture crosses from 9 to 10 without colliding with its content.

{% capture later_steps %}
1. Eighth step with short content.
2. Ninth step with enough content to wrap across lines at a narrow viewport while preserving the guide and first-content alignment.
3. Tenth step with a [focusable destination](../navigation/) and a two-digit circle.
{% endcapture %}
{% include components/steps.html content=later_steps start=8 %}

Invalid, zero, negative, fractional, or nonnumeric `start` values fall back to `1`.

## Malformed input fallback

Content that is not one outer ordered list renders as ordinary Markdown without step decoration:

{% capture malformed_steps %}
This paragraph is intentionally not an ordered list.

- It remains an ordinary unordered list.
- Its content stays readable.
{% endcapture %}
{% include components/steps.html content=malformed_steps start="invalid" %}

With styles unavailable, both valid examples remain ordered lists in source order with native markers. The invalid example remains an ordinary paragraph and unordered list. No JavaScript is required.

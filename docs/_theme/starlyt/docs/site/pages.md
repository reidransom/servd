---
title: Page layouts and heroes
permalink: /pages/
hero:
  title: Documentation can have a hero too
  tagline: Page layout and hero presentation are independent choices, even with a long explanation that wraps on narrow screens.
  image:
    src: /media/workflow.svg
    alt: A workflow diagram
    width: 960
    height: 480
  actions:
    - text: View the splash layout without a hero
      link: /wide-page/
      variant: secondary
---

Choose the shell with Jigyll's `layout` front matter. `default` keeps the navigation and heading table of contents. `splash` omits both sidebars and gives the article a wider content region. Either layout can render an optional hero.

## Add a hero

Hero text is escaped plain text. Article Markdown begins after the hero. If `hero.title` is absent, the hero uses `page.title`; an empty or absent tagline creates no gap.

```yaml
layout: splash
hero:
  tagline: A concise plain-text introduction.
  actions:
    - text: Get started
      link: /getting-started/
      variant: primary
    - text: Project source
      link: https://example.com/project
      variant: minimal
```

Action variants are `primary`, `secondary`, and `minimal`; invalid variants fall back to `primary`. Action text is plain text. Links must be root-relative internal paths or absolute HTTP(S) URLs. Malformed actions are omitted.

## Add an image

Use either one root-relative consumer image:

```yaml
hero:
  image:
    src: /media/diagram.svg
    alt: Architecture overview
    width: 960
    height: 480
```

Or provide a complete light/dark pair:

```yaml
hero:
  image:
    light: /media/diagram-light.svg
    dark: /media/diagram-dark.svg
    alt: Architecture overview
    width: 960
    height: 480
```

`alt`, `width`, and `height` are required. Alternative text may be empty only when the artwork is decorative. Image paths are root-relative without `baseurl`; Starlyt applies the deployment prefix. Incomplete pairs, nonpositive dimensions, and non-root-relative paths omit the image instead of emitting broken markup.

## Supported combinations

- `layout: default` without `hero`: the ordinary documentation shell used by most guides.
- `layout: default` with `hero`: this page.
- `layout: splash` without `hero`: the [wide-page fixture](../wide-page/).
- `layout: splash` with `hero`: the [documentation home page](../).

The [minimal hero fixture](../minimal-hero/) exercises an image-free hero and an empty tagline. Hero values do not accept Markdown, raw HTML, component slots, or arbitrary action attributes.

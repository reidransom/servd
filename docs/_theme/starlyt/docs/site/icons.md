---
title: Icons
permalink: /icons/
icon_label: "Starlyt <mark>safe</mark> & literal"
icon_names:
  - "add-document"
  - "alpine"
  - "analytics"
  - "apple"
  - "approve-check"
  - "approve-check-circle"
  - "astro"
  - "azureDevOps"
  - "backstage"
  - "bars"
  - "biome"
  - "bitbucket"
  - "blueSky"
  - "bun"
  - "chrome"
  - "clock"
  - "close"
  - "cloud-download"
  - "cloudflare"
  - "code-branch"
  - "codePen"
  - "codeberg"
  - "comment"
  - "comment-alt"
  - "confluence"
  - "database"
  - "deno"
  - "desktop"
  - "discord"
  - "discourse"
  - "document"
  - "down-arrow"
  - "down-caret"
  - "download"
  - "edge"
  - "email"
  - "error"
  - "external"
  - "facebook"
  - "farcaster"
  - "figma"
  - "firefox"
  - "forgejo"
  - "forward-slash"
  - "github"
  - "gitlab"
  - "gitter"
  - "goodreads"
  - "hackerOne"
  - "heart"
  - "homebrew"
  - "hypothesis"
  - "information"
  - "instagram"
  - "jetbrains"
  - "jira"
  - "jsr"
  - "laptop"
  - "left-arrow"
  - "left-caret"
  - "link"
  - "link-alt"
  - "linkedin"
  - "linux"
  - "list-format"
  - "magnifier"
  - "mastodon"
  - "matrix"
  - "mdx"
  - "microsoftTeams"
  - "mobile-android"
  - "moon"
  - "netlify"
  - "nix"
  - "node"
  - "nostr"
  - "notes"
  - "npm"
  - "npmx"
  - "open-book"
  - "openCollective"
  - "padlock"
  - "patreon"
  - "pen"
  - "pencil"
  - "phone"
  - "pinterest"
  - "pkl"
  - "pnpm"
  - "puzzle"
  - "question"
  - "question-circle"
  - "random"
  - "reddit"
  - "right-arrow"
  - "right-caret"
  - "rocket"
  - "rss"
  - "safari"
  - "server"
  - "setting"
  - "signal"
  - "sketch"
  - "slack"
  - "solidjs"
  - "sourcehut"
  - "stackOverflow"
  - "star"
  - "starlight"
  - "storybook"
  - "substack"
  - "sun"
  - "telegram"
  - "threads"
  - "tiktok"
  - "translate"
  - "twitch"
  - "twitter"
  - "up-arrow"
  - "up-caret"
  - "vercel"
  - "vim"
  - "vscode"
  - "warning"
  - "whatsApp"
  - "window"
  - "x.com"
  - "youtube"
  - "zed"
  - "zulip"
---

Starlyt ships the 130 names from stock Starlight commit `39d4e71f23b3fb6fde0e77eb983fcd38629b70b9`. This alphabetical inventory and its rendered fixture are generated from `tools/icons/icons.json`; run `python tools/icons/generate.py --check` after registry changes.

## Authoring contract

`icon.html` requires a supported `name`. `size` accepts `small`, `medium`, or `large` and defaults to `medium`. `color` accepts `currentColor`, `accent`, `blue`, `green`, `orange`, `purple`, `red`, or `gray` and defaults to `currentColor`. Other names omit the SVG; invalid sizes and colors use their defaults. Arbitrary CSS, URLs, SVG, and HTML are rejected.

```liquid
{% raw %}{% include components/icon.html name="open-book" size="large" color="accent" %}{% endraw %}
```

An omitted label makes the icon decorative. Supply escaped plain text with `label` only when the icon itself needs an accessible name:

<div class="icon-contract-fixtures not-content"><span data-fixture="meaningful">{% include components/icon.html name="starlight" label=page.icon_label size="large" color="accent" %}</span><a href="#all-icons" data-fixture="link">{% include components/icon.html name="right-arrow" %} All icons</a><button type="button" data-fixture="button">{% include components/icon.html name="approve-check" color="green" %} Confirm</button><span data-fixture="invalid-defaults">{% include components/icon.html name="star" size="huge" color="url-danger" %}</span><span data-fixture="unknown">{% include components/icon.html name="not-in-the-frozen-registry" %}</span></div>

When an icon appears beside link, button, card, tab, badge, or callout text, omit `label`; the surrounding text remains the sole accessible name. Color is supplementary and never the only meaning.

## Sizes and semantic colors

<div class="icon-contract-fixtures not-content"><span>Small {% include components/icon.html name="star" size="small" %}</span><span>Medium {% include components/icon.html name="star" size="medium" %}</span><span>Large {% include components/icon.html name="star" size="large" %}</span><span>Accent {% include components/icon.html name="information" color="accent" %}</span><span>Blue {% include components/icon.html name="information" color="blue" %}</span><span>Green {% include components/icon.html name="approve-check-circle" color="green" %}</span><span>Orange {% include components/icon.html name="warning" color="orange" %}</span><span>Purple {% include components/icon.html name="rocket" color="purple" %}</span><span>Red {% include components/icon.html name="error" color="red" %}</span><span>Gray {% include components/icon.html name="notes" color="gray" %}</span></div>

Representative narrow, wide, filled, brand, and directional glyphs render at every supported size:

{% assign geometry_names = "forward-slash,bars,star,github,right-arrow" | split: "," %}{% assign geometry_sizes = "small,medium,large" | split: "," %}<div class="icon-geometry-grid not-content">{% for geometry_size in geometry_sizes %}{% for geometry_name in geometry_names %}<div class="icon-geometry-sample" data-geometry-name="{{ geometry_name }}" data-geometry-size="{{ geometry_size }}">{% include components/icon.html name=geometry_name size=geometry_size %}<code>{{ geometry_name }} · {{ geometry_size }}</code></div>{% endfor %}{% endfor %}</div>

## All icons

<div class="icon-grid not-content" data-icon-count="130">{% for icon_name in page.icon_names %}<div class="icon-sample" data-icon-name="{{ icon_name }}">{% include components/icon.html name=icon_name size="large" %}<code>{{ icon_name }}</code></div>{% endfor %}</div>

The registry is frozen. File-type icons use a separate internal inventory. The SVG paths are covered by `assets/starlight-LICENSE`; consumers need no generator or runtime sprite request.

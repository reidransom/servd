---
title: Localization
permalink: /localization/
---

Starlyt uses Jigyll's native localization interfaces. The theme does not define
locale configuration or own message catalogs. A localized consumer provides its
locales, content editions, and `_data/locales/<locale>/messages.yml` files.
Sites without a `localization` block keep the existing English interface and do
not need a catalog.

## Configure locales and editions

Declare the default locale and every published locale in `_config.yml`:

```yaml
localization:
  default_language: en
  locales:
    en: { tag: en, label: English }
    fr: { tag: fr, label: Français }
    ar: { tag: ar, label: العربية, direction: rtl }
```

Relate editions with the same `translation_key`. Each edition owns its title,
body, and canonical route:

```yaml
---
title: Guide français
lang: fr
translation_key: guide
permalink: /demarrage/
---
```

Starlyt reads `page.language` for the document `lang` and `dir` attributes and
uses `page.alternates` for canonical, language-alternate, and `x-default`
metadata. Jigyll omits unavailable editions; Starlyt does not publish default
content under a missing locale route. Use Jigyll's `required_translations`
option when selected locales must contain every default-language edition.

## Switch between published editions

When a page has at least two editions, Starlyt renders a language disclosure
from Jigyll's `page.all_translations` collection. Its order follows the locale
order in `_config.yml`. The current edition is identified rather than linked,
and locales without an edition are absent.

Each link uses the translated edition's canonical URL. Switching languages
therefore drops the current query string and fragment instead of guessing that
another edition has an equivalent route or heading ID. The disclosure and its
ordinary anchors remain operable when JavaScript is disabled.

## Author right-to-left content

Locales with `direction: rtl` mirror the shell while retaining source and focus
order. Starlyt keeps fenced code, inline code, file names, and search input
isolated in their natural direction. Wide code and tables scroll within their
own containers; images, brand marks, search, external-link, and copy icons are
not mirrored. Only left/right arrow and caret icons reverse.

Use semantic HTML when a mixed-script run needs an explicit direction. A
literal URL inside Arabic prose can use `<bdi dir="ltr">https://example.test</bdi>`.
The retained Arabic fixture exercises the supported layout and content
features. It establishes Arabic acceptance, not blanket support for every RTL
language.

## Localize navigation

Keep one `site.navigation` tree. Use `label_key` for translated entries and
`label` for intentionally invariant text:

```yaml
navigation:
  - label_key: nav.guides
    items:
      - label_key: nav.home
        link: /
      - label_key: nav.guide
        link: /guide/
```

A navigation entry must not set both label fields. Root-relative links are
canonical content routes: Starlyt passes them through Jigyll's `localized_url`
filter for the active edition and then applies the hosting `baseurl`. Unknown
or unavailable routes fail the build instead of receiving a guessed prefix.

## Provide the Starlyt message namespace

Every localized catalog must define these keys. Values are plain text; search
result messages use `{count}`, and `anchor_label` uses `{title}`.

```yaml
starlyt:
  skip_to_content: "Skip to content"
  menu: "Menu"
  table_of_contents: "Table of contents"
  on_this_page: "On this page"
  overview: "Overview"
  search:
    label: "Search"
    dialog_label: "Search documentation"
    cancel: "Cancel"
    input_label: "Search documentation"
    placeholder: "Search documentation"
    retry: "Retry"
    empty: "Enter words to search this documentation."
    loading: "Loading search…"
    error: "Search could not load. Check your connection and retry."
    no_results: "No results found. Try another term."
    one_result: "{count} result"
    results: "{count} results"
  color_mode:
    label: "Select theme"
    dark: "Dark"
    light: "Light"
    auto: "Auto"
  copy:
    label: "Copy to clipboard"
    success: "Copied!"
    failure: "Copy failed"
  language: "Language"
  callout:
    note: "Note"
    tip: "Tip"
    caution: "Caution"
    danger: "Danger"
  anchor_label: "Section titled “{title}”"
  not_found:
    title: "Page not found"
    back_home: "Back to home"
```

The retained English, French, and Arabic fixture catalogs and the checked key
inventory live under `fixtures/localization/`. Missing messages follow Jigyll's
policy: the default is a build failure. Set `localization.missing_messages: key`
only when visibly rendering the unresolved key is preferable during catalog
work.

## Search one locale

Localized pages embed the active Jigyll site's search corpus for lazy indexing
by the existing search enhancer. Results therefore contain only pages published
for that locale, while ranking, matching, and text-only rendering retain the
normal search behavior. Corpus URLs and navigation assets keep the configured
hosting prefix. Nonlocalized sites continue to use the standalone
`assets/search-data.json` corpus.

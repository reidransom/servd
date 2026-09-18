---
title: Markdown
permalink: /markdown/
---

These are ordinary Markdown examples rendered by Jigyll and styled by Starlyt. Source blocks show how to author selected examples; the rendered content below them is also useful for manual inspection.

## Inline formatting

```markdown
Use **strong text**, *emphasis*, and `inline code` for different purposes.
Read the [code guide](../code/) for complete examples.
```

Use **strong text**, *emphasis*, and `inline code` for different purposes. Read the [code guide](../code/) for complete examples.

Inline markup should retain a readable baseline inside a paragraph. Literal values such as `<article>`, `baseurl: /docs`, and `navigation.items` should not be mistaken for page markup.

## Lists

```markdown
- Write a useful example.
- Build the consumer.
  - Keep content separate from the theme.
  - Inspect the actual output.
- Read the result.
```

- Write a useful example.
- Build the consumer.
  - Keep content separate from the theme.
  - Inspect the actual output.
- Read the result.

### An ordered procedure

1. Choose the page's public route.
2. Add that route to consumer navigation.
3. Write its content with H2 sections.
4. Build and read the page at that route.

A list is still part of the article: nesting and wrapped lines should not make the entire page wider than the viewport.

## Quotations

```markdown
> Good documentation explains what a reader can do,
> not just how the software is arranged.
```

> Good documentation explains what a reader can do,
> not just how the software is arranged.

This is a blockquote, not a custom callout or an Astro component. Starlyt does not need special component syntax to present ordinary prose.

## Tables

```markdown
| Owner | Supplies |
| --- | --- |
| Consumer | Content, navigation, configuration |
| Theme | Layout, styling, browser enhancements |
```

| Owner | Supplies |
| --- | --- |
| Consumer | Content, navigation, configuration |
| Theme | Layout, styling, browser enhancements |

### Longer cells

| Surface | Authoring input | What a reader sees |
| --- | --- | --- |
| Navigation | An ordered recursive list of labels and routes | Sidebar groups or the mobile Menu popover |
| Headings | Markdown heading levels and engine-emitted IDs | Article headings, anchor links, and an H2/H3 outline |
| Code | Fenced source with an optional language | Server-highlighted code and an optional clipboard control |

Try narrowing the window while reading this table and the code examples. Distinguish local content overflow from unwanted page-wide horizontal scrolling.

## Images

```markdown
![Three stages connected left to right: Write, Build, Read](../media/workflow.svg)
```

![Three stages connected left to right: Write, Build, Read](../media/workflow.svg)

This artwork reuses the POC's simple documentation workflow diagram. The living copy is ordinary content in this consumer; the historical fixture remains unchanged. The image has its own colors, so it does not automatically recolor when you switch the theme palette.

The relative path is resolved from this page's public `/markdown/` route. It remains inside the site when deployed under a base URL. Give images meaningful alternate text instead of using their filenames as descriptions.

## Separate a new thought

A thematic break can separate related ideas without pretending to be a heading:

---

For longer source examples, continue to [Code blocks](../code/). For the rendering responsibilities behind these examples, read [Theme internals](../internals/).

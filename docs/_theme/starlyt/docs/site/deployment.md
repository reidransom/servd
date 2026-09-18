---
title: Deployment
permalink: /deployment/
---

Jigyll produces static files. Publish the consumer's `_site/` output, not the theme repository, its installed Git checkout, or the Markdown source. No Node.js or server-side theme runtime is required to serve that output.

## Host at the root

For a site at the root of a domain, keep `baseurl` empty. Set `url` to your real public origin when deploying; leave it empty for the local documentation preview.

```yaml
url: https://docs.example.com
baseurl: ""
```

`docs.example.com` is an illustrative deployment origin, not a Starlyt service. Replace it with your own host. From this project's repository root, the installed documentation consumer builds with:

```sh
jigyll build --source .poc/living-docs
```

Serve it locally with the README's ordinary `jigyll serve` command. A local HTTP preview is preferable to opening generated HTML with a `file:` URL: absolute asset paths and Clipboard API availability depend on an appropriate origin.

## Host under a prefix

For a site published at `https://example.com/docs/`, use:

```yaml
url: https://example.com
baseurl: /docs
```

The prefix starts with a slash and has no trailing slash. Page permalinks and navigation links still exclude it. For example, the code page's permalink and navigation link remain `/code/`, not `/docs/code/`.

The theme's URL filter adds the prefix to its asset and navigation URLs. The generated output must actually be served under `/docs/`; setting `baseurl` does not make a static host mount that directory automatically.

### Build a separate prefixed output

From the repository root, after creating the documentation consumer, you can inspect prefix output without changing the tracked configuration:

```sh
jigyll build --source .poc/living-docs --baseurl /docs --destination "$PWD/.poc/prefix-preview/docs"
python -m http.server 4323 --bind 127.0.0.1 --directory .poc/prefix-preview
```

Open <http://127.0.0.1:4323/docs/>. Python's static server is an optional local preview tool for this example, not a consumer build dependency. Stop it with Control-C. Any static server that can mount the output at the prefix can serve the same unchanged files.

The destination is absolute because Jigyll resolves relative destinations from the consumer source directory, not the shell's working directory.

### Distinguish preview from deployment

Jigyll 1.10.1's preview server does not strip `baseurl` from incoming request paths. Building correct prefixed URLs and serving those files at their intended prefix are separate concerns. The static-server example above addresses the preview limitation without rewriting generated HTML or changing theme links.

Do not treat a failed prefixed request to that preview server as proof that the generated static output is wrong. Conversely, a successful root preview does not by itself show that prefix mounting is configured correctly on your host.

## Author portable article links

The Markdown filename and its public route are different concepts. This page is served at `/deployment/`, so a sibling-page link is relative to that route:

```markdown
Read [Getting started](../getting-started/).
```

Read [Getting started](../getting-started/). The same link resolves within `/docs/` when the site is hosted there.

The overview uses `./getting-started/` because its route is `/`. Images use the same rule: the Markdown guide links to `../media/workflow.svg`. Avoid root-absolute article links such as `/code/` when you want them to remain inside a deployment prefix.

## Publish a custom 404 page

Starlyt supplies the `not-found` layout, but a theme cannot create a consumer route or configure a hosting provider. Add a root `404.md` to the consumer:

```yaml
---
title: Page not found
layout: not-found
permalink: /404.html
search_exclude: true
---
```

`jigyll new` creates a starter `404.html`. Remove that starter when adding `404.md`; two source files must not publish the same permalink. The only replacement file is the documented `404.md`—no layout, include, Sass, or theme asset is copied into the consumer.

The default page renders a `Page not found` heading and a `Back to home` link. Customize its escaped plain-text message and ordered recovery links in front matter:

```yaml
not_found:
  message: The requested documentation page may have moved.
  links:
    - text: Documentation home
      link: /
      variant: primary
    - text: Contact support
      link: https://example.com/support
      variant: minimal
```

Links use the hero action contract: `text` and `link` are required; variants are `primary`, `secondary`, or `minimal`. Root-relative links pass through the configured `baseurl`; malformed links are omitted. Set `search_exclude: true` so the missing-page message does not appear as documentation content.

### GitHub Pages

For a user or organization site at `/`, publish the generated `/404.html` at the repository root. For a project site such as `https://owner.github.io/docs/`, set `baseurl: /docs`; the generated recovery links and assets then retain that prefix. GitHub Pages serves the project's `/docs/404.html` for missing project routes while preserving HTTP status 404.

GitHub Pages setup is concrete host behavior, not a portable static-host guarantee. On another host, configure missing routes to serve the generated `404.html` while preserving HTTP 404. A local static server commonly returns 200 for a direct `/404.html` request and therefore cannot prove production fallback status.

## Inspect what you publish

Use the navigation, follow an article link, load the image, and switch color modes on your chosen preview origin. These are useful manual observations, not an automated deployment gate or a claim about a production service.

Keep source files, installed themes, and historical evidence out of the deployed output by building the separate documentation consumer rather than the repository root. Review your hosting configuration separately; hosting-provider setup and production monitoring are outside this documentation workflow.

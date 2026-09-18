# POC implementation and evidence

The Node.js POC tooling has been retired without replacement. This document retains the implementation contract and historical results; its capture matrix, comparison thresholds, and evidence are not an ongoing automated testing requirement. Use ordinary Jigyll commands to install, build, and preview the theme. No browser runner, image comparator, or reference-regeneration workflow is maintained.

## Scope

Tickets 29–35 implemented the documentation-only POC. This is not a claim of complete Starlight, Astro, configuration, browser, or device compatibility. Historical comparisons used a pixelmatch threshold of 0.1 with a maximum 0.1% differing pixels for shell/prose; the results and limitations below describe that run, not a current verification gate.

## Historical reference baseline (29)

The source was stock Starlight commit `39d4e71f23b3fb6fde0e77eb983fcd38629b70b9`. The retired reference builder exported that revision into ignored `.poc/reference`, installed its frozen lockfile with pnpm 11.22.0, built the workspace, and replaced only the disposable basics example with the controlled documentation fixture. The original checkout was not modified. Search, pagination, credits, code title frames, and line numbers were disabled; no marketing homepage or custom CSS was used.

`fixtures/content` supplied the shared Markdown for both builds, and `fixtures/fixture.svg` supplied the shared image. These examples remain available for documentation. The retired fixture manifest defined the titles, URLs, ordered navigation, and viewport matrix. The Astro adapter resolved its relative image at build time; the Jigyll consumer served the same image at its relative public URL. No theme implementation files were embedded in the fixture.

The retired capture runner used `/usr/bin/chromium`, headless, DPR 1, zoom 1, default browser font size 16px, `en-US`, UTC, and explicit light/dark system and storage state. It recorded the Chromium version and executable hash, kernel, installed font list, CSS font stacks, CDP-resolved paragraph fonts, scroll position, code rectangles, and clipboard output. It waited for the network, fonts, and two animation frames; animations/transitions and caret blinking were disabled, without hiding theme content. Each case had an isolated browser context. Those environment details scope the retained results; they are not a maintained reproduction procedure.

`evidence/poc/reference-build.json` records the toolchain, configuration, revision, and lockfile checksum. `evidence/poc/reference/manifest.json` identifies every retained unmasked full-page PNG and immediate reference-versus-reference repeat measurement. The matrix comprises 18 primary and eight boundary captures plus 14 interaction captures (seven states in both modes).

Historical interaction recipes covered the mobile Menu button, mobile TOC summary, all sidebar groups collapsed/expanded, scrolling the “Maintain the examples” heading into view, keyboard focus on the skip link, and the first code-copy button with actual clipboard readback. Storage was reset for each recipe. Focus and copy cases used desktop width; menu and TOC used 390×844. Scrolled captures retain the observed scroll position in the manifest.

Calibration: independent `reference-repeat` contexts passed all 40 historical comparisons. Immediate repeats had zero differing pixels. The retained manifests identify Chromium 152.0.7977.82 and Liberation Sans paragraph rendering on Linux. `fixture-hashes.json` records the shared inputs used for that run.

## Theme installation and shared contract (30)

Consumers run `jigyll new my-site --theme <git-url>`, set `title`, `navigation`, and a layout default in their own `_config.yml`, then run `jigyll build` or `jigyll serve`. Theme configuration/data are not merged. The POC tested Jigyll 1.10.1; current Starlyt requires Jigyll 1.12.0 or later. There is no consumer Node/Astro/Sass dependency.

```yaml
title: My documentation
theme: starlyt
baseurl: '' # Or /docs
navigation:
  - label: Overview
    link: /
  - label: Guides
    items:
      - label: Installation
        link: /guides/install/
defaults:
  - scope:
      path: ''
    values:
      layout: default
```

Navigation is an ordered recursive list. Each entry has a text `label` and either a root-relative `link` (without `baseurl`) or `items`. Pages use ordinary `title` and `permalink` front matter. Author article links/images relatively for portability between hosting roots. Labels and titles are escaped. Consumers can override includes, layouts, or assets normally; no internal-file copying is needed for installation.

Shared implementation ownership is sequential:

- `_layouts/default.html` composes the fixed header, `#site-nav`, `#main-content`, `#article`, and `#toc-slot`.
- `#article` contains only rendered Markdown. The page H1, header, sidebar, and heading navigation are outside this discovery boundary.
- `_includes/header.html` owns the site-title and mode-selector insertion point. `_includes/navigation.html` owns recursive consumer navigation.
- Root `data-theme` is the effective `light`/`dark` palette. The selector's preference is separate (`auto`/`light`/`dark`), persisted under `starlight-theme`.
- Early mode initialization runs synchronously in the head, before CSS and body; navigation, TOC, and code enhancements run as ordered deferred assets. No client renderer or hydration framework is involved.
- `_sass/starlyt.scss` is the shared entry point, with focused partials for tokens, reset, navigation, TOC, and Markdown. Maintainers compile with `sass --no-source-map _sass/starlyt.scss assets/starlyt.css`. Shipped CSS is the consumer artifact.
- Responsive transitions retain 50rem (800px) and 72rem (1152px), at default browser font settings. The POC's automatic TOC and copy controls required JavaScript; the current heading TOC is server-rendered from Jigyll 1.12.0 metadata.

## Navigation, headings, Markdown, and modes (31–34)

The mobile menu uses a native popover and nested native disclosures. Enhancement adds focus containment, Escape restoration, covered-content inertness, breakpoint cleanup, and session-scoped disclosure/scroll persistence. Navigation labels remain consumer-owned text. Heading anchors and the generated H2/H3 TOC use the IDs already emitted by Jigyll; they do not derive replacement slugs. The TOC discovers headings only inside `#article`, tracks the current section, and becomes an inline desktop rail or mobile disclosure.

Markdown and code styling target the actual Jigyll/Chroma output. The copy enhancement reads `code.textContent`, preserves whitespace, reports real clipboard success or failure, and does not alter syntax markup. Copying requires JavaScript and the browser Clipboard API in a secure context. No-JavaScript reading retains links, images, native navigation disclosures/popover, and server-highlighted code; automatic TOC, heading-anchor enhancements, copying, and the mode selector are absent rather than presented as nonfunctional controls. The no-JavaScript palette is the documented default dark palette.

Mode preference is `auto`, `light`, or `dark`. Auto follows live `prefers-color-scheme` changes. Explicit choices survive ordinary page navigation; invalid or unavailable storage falls back safely to auto. The blocking head asset initializes the effective palette before body paint. The same selector moves between header and mobile navigation rather than creating duplicate focusable controls.

Functional evidence is retained in `navigation-checks.json`, `toc-checks.json`, `code-checks.json`, `mode-checks.json`, `integration-checks.json`, and `geometry-checks.json` under `evidence/poc/`. Browser smoke scenarios covered both hosting roots, both modes, repeated headings, encoded fragments, outside clicks, keyboard focus, Escape, resizing open controls, storage denial, clipboard denial, exact copied whitespace, content containment, and JavaScript-disabled reading. These are observed browser results, not claims of cross-browser coverage.

## Historical installed consumer (35)

The acceptance consumer was created through the real Git theme installer. The retired maintainer adapter wrote only ordinary consumer configuration, Markdown/front matter, and the fixture image; it added no consumer build preprocessing. The consumer runtime PATH contained only Jigyll and Git helpers, with no Node, Astro, or Sass executable. The adapter itself ran under Node.js; it has been deleted rather than replaced.

`release-build.json` and `release-base-build.json` record the tested temporary theme revision/tree, engine version, runtime PATH, and installation/build/serve commands. They document a standalone installed theme, not an Astro-powered consumer or an internal copied-template demo. Commands recorded in historical evidence are not current setup instructions.

Jigyll 1.10.1's preview server does not strip `baseurl` from incoming requests. The root consumer was served normally by Jigyll. The unchanged `/docs/` build was mounted at its deployment prefix for HTTP checks. This is a preview-server limitation, not a claim that `jigyll serve` supports prefix mounting. All fixture links and assets were checked at both origins. A deployed static host must mount the build at its configured prefix.

The retained comparison artifacts include unmasked whole-page PNGs and difference images, with whole-page, shell/prose, and code-region percentages reported separately. Code rectangles were classified for reporting, not painted over in the artifacts. Page dimensions and every code-block rectangle were compared independently of the pixel percentage. `code-regions/` contains 32 reference/consumer/difference crop triples, and `code-tokens.json` retains actual emitted token HTML, classifications, and both-mode colors.

The pinned environment is Chromium 152.0.7977.82 on Linux, DPR/zoom 1, default font size 16px, Liberation Sans prose and Liberation Mono code. `platform.json` adds resolved code-font and installed package evidence to the capture manifests. Stock Starlight and Expressive Code attribution is retained in the shipped license assets.

## Historical acceptance verdict

**Standalone-theme feasibility is demonstrated for the scoped POC.** The installed consumer passes all 40 shell/prose comparisons at pixelmatch threshold 0.1 and the unchanged 0.1% gate. The largest shell/prose difference is **0.02718%**. All page dimensions and code-block rectangles match. The final consumer's 40 immediate repeats have zero differing pixels, as did the independent reference calibration.

This is **not an exact whole-page pass**: 38/40 whole-page comparisons fall below 0.1%. The two 390×844 code-example captures differ by **0.20774% light** and **0.21660% dark**, with a maximum aggregate code-region difference of **0.54175%**. Native Chroma token boundaries/colors account for the substantive remaining code differences; the unmasked crops also retain small glyph/icon raster differences, including seven differing pixels in each unlabeled block. None are hidden or used to relax the threshold.

The paired copy-state images now retain matching control/feedback geometry and palette, not a token exemption for control layout. Code source, indentation, line breaks, clipboard success/denial, readable both-mode highlighting, typography, padding, backgrounds, and local overflow pass their functional checks. No in-scope functional check remains failed. The `/docs/` preview mounting limitation above remains explicit.

An additional engine edge case is recorded in `heading-id-engine.json`: Jigyll 1.10.1 emits the raw-HTML ID `manual&amp;quoted` as the DOM ID `manual&quotedoted`. The mutation is present in built HTML before theme JavaScript runs. Starlyt preserves and navigates to that emitted ID; it does not repair upstream raw-HTML ID fidelity or substitute its own slug.

[Ticket 16](../.scratch/starlight-theme/issues/16-syntax-fidelity.md) receives the code-token evidence: for example, Chroma emits the entire shell command as one plain span where Shiki distinguishes command/arguments/options. Native CSS cannot recover token boundaries that the renderer does not emit. No replacement renderer or consumer toolchain was introduced.

Post-POC work remains separate: token-fidelity investigation, broader browsers/devices and accessibility coverage, components, search, and Astro/configuration compatibility. The upstream heading-metadata interface released in Jigyll 1.12.0 is consumed by the current theme; it was not part of this POC verdict.

## Historical review and verification

Reviews ran sequentially. The standards pass corrected HTML escaping for generated URL attributes and Sass import order so the reset layer is registered before the base layer. The spec pass checked tickets 29–35 against the retained evidence and found no remaining scoped requirement gap; it preserved the explicit code-fidelity, no-JavaScript, preview-prefix, and broader-compatibility limitations above.

The POC's final verification included JavaScript/tooling type checking, Sass compilation, frozen fixture checksum comparison, fresh minimal-PATH Git installations, both-root browser interaction/code/mode scenarios, primary/boundary geometry checks, and a complete 40-case recapture/comparison. `edge-checks.json` additionally covers consumer override precedence, quoted URL/label escaping, explicit/repeated heading IDs, literal labels, and heading-free reading. These are historical checks, not requirements to retain or replace the removed automation.

The final run passed. All 40 post-review PNGs are byte-identical to the accepted captures, so the retained code-region crops still correspond exactly to the final images. `full-verification.json` records the checks, review outcomes, final capture hashes, and the separate engine observation.

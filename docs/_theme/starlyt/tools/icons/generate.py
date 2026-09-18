#!/usr/bin/env python3
"""Generate Starlyt's frozen public icon include and documentation fixture."""

from __future__ import annotations

import argparse
import hashlib
import json
import sys
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
SOURCE = Path(__file__).with_name("icons.json")
EXPECTED_COMMIT = "39d4e71f23b3fb6fde0e77eb983fcd38629b70b9"
EXPECTED_COUNT = 130
EXPECTED_DIGEST = "30012e5593475f9b8a6d8ab0f38bff5bde49aa7c51eec5e55e9b74db1f2976c3"
OUTPUTS = {
    ROOT / "_includes/private/icon-path.html": "private",
    ROOT / "docs/site/icons.md": "docs",
}


def load_registry() -> dict[str, str]:
    source = json.loads(SOURCE.read_text())
    icons = source["icons"]
    canonical = "\0".join(f"{name}\0{path}" for name, path in icons.items())
    digest = hashlib.sha256(canonical.encode()).hexdigest()
    if source["commit"] != EXPECTED_COMMIT or len(icons) != EXPECTED_COUNT or digest != EXPECTED_DIGEST:
        raise ValueError("icons.json is not the frozen Starlight registry")
    if list(icons) != sorted(icons):
        raise ValueError("icons.json names must be alphabetical")
    for name, paths in icons.items():
        root = ET.fromstring(f"<svg>{paths}</svg>")
        if len(root) == 0 or any(child.tag != "path" for child in root):
            raise ValueError(f"{name}: registry values may contain only SVG path elements")
    return icons


def render_private(icons: dict[str, str]) -> str:
    branches = "".join(f"{{% when '{name}' %}}{paths}" for name, paths in icons.items())
    return "{% case include.name %}" + branches + "{% endcase %}\n"


def render_docs(icons: dict[str, str]) -> str:
    names = "\n".join(f"  - {json.dumps(name)}" for name in icons)
    return f'''---
title: Icons
permalink: /icons/
icon_label: "Starlyt <mark>safe</mark> & literal"
icon_names:
{names}
---

Starlyt ships the 130 names from stock Starlight commit `{EXPECTED_COMMIT}`. This alphabetical inventory and its rendered fixture are generated from `tools/icons/icons.json`; run `python tools/icons/generate.py --check` after registry changes.

## Authoring contract

`icon.html` requires a supported `name`. `size` accepts `small`, `medium`, or `large` and defaults to `medium`. `color` accepts `currentColor`, `accent`, `blue`, `green`, `orange`, `purple`, `red`, or `gray` and defaults to `currentColor`. Other names omit the SVG; invalid sizes and colors use their defaults. Arbitrary CSS, URLs, SVG, and HTML are rejected.

```liquid
{{% raw %}}{{% include components/icon.html name="open-book" size="large" color="accent" %}}{{% endraw %}}
```

An omitted label makes the icon decorative. Supply escaped plain text with `label` only when the icon itself needs an accessible name:

<div class="icon-contract-fixtures not-content"><span data-fixture="meaningful">{{% include components/icon.html name="starlight" label=page.icon_label size="large" color="accent" %}}</span><a href="#all-icons" data-fixture="link">{{% include components/icon.html name="right-arrow" %}} All icons</a><button type="button" data-fixture="button">{{% include components/icon.html name="approve-check" color="green" %}} Confirm</button><span data-fixture="invalid-defaults">{{% include components/icon.html name="star" size="huge" color="url-danger" %}}</span><span data-fixture="unknown">{{% include components/icon.html name="not-in-the-frozen-registry" %}}</span></div>

When an icon appears beside link, button, card, tab, badge, or callout text, omit `label`; the surrounding text remains the sole accessible name. Color is supplementary and never the only meaning.

## Sizes and semantic colors

<div class="icon-contract-fixtures not-content"><span>Small {{% include components/icon.html name="star" size="small" %}}</span><span>Medium {{% include components/icon.html name="star" size="medium" %}}</span><span>Large {{% include components/icon.html name="star" size="large" %}}</span><span>Accent {{% include components/icon.html name="information" color="accent" %}}</span><span>Blue {{% include components/icon.html name="information" color="blue" %}}</span><span>Green {{% include components/icon.html name="approve-check-circle" color="green" %}}</span><span>Orange {{% include components/icon.html name="warning" color="orange" %}}</span><span>Purple {{% include components/icon.html name="rocket" color="purple" %}}</span><span>Red {{% include components/icon.html name="error" color="red" %}}</span><span>Gray {{% include components/icon.html name="notes" color="gray" %}}</span></div>

Representative narrow, wide, filled, brand, and directional glyphs render at every supported size:

{{% assign geometry_names = "forward-slash,bars,star,github,right-arrow" | split: "," %}}{{% assign geometry_sizes = "small,medium,large" | split: "," %}}<div class="icon-geometry-grid not-content">{{% for geometry_size in geometry_sizes %}}{{% for geometry_name in geometry_names %}}<div class="icon-geometry-sample" data-geometry-name="{{{{ geometry_name }}}}" data-geometry-size="{{{{ geometry_size }}}}">{{% include components/icon.html name=geometry_name size=geometry_size %}}<code>{{{{ geometry_name }}}} · {{{{ geometry_size }}}}</code></div>{{% endfor %}}{{% endfor %}}</div>

## All icons

<div class="icon-grid not-content" data-icon-count="{len(icons)}">{{% for icon_name in page.icon_names %}}<div class="icon-sample" data-icon-name="{{{{ icon_name }}}}">{{% include components/icon.html name=icon_name size="large" %}}<code>{{{{ icon_name }}}}</code></div>{{% endfor %}}</div>

The registry is frozen. File-type icons use a separate internal inventory. The SVG paths are covered by `assets/starlight-LICENSE`; consumers need no generator or runtime sprite request.
'''


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--check", action="store_true")
    args = parser.parse_args()
    icons = load_registry()
    rendered = {
        "private": render_private(icons),
        "docs": render_docs(icons),
    }
    stale = [path for path, kind in OUTPUTS.items() if not path.exists() or path.read_text() != rendered[kind]]
    if args.check:
        if stale:
            print("Generated icon files are stale:", file=sys.stderr)
            for path in stale:
                print(f"- {path.relative_to(ROOT)}", file=sys.stderr)
            return 1
        print(f"Icon registry synchronized: {len(icons)} names")
        return 0
    for path, kind in OUTPUTS.items():
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(rendered[kind])
    print(f"Generated {len(OUTPUTS)} files from {len(icons)} icons")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

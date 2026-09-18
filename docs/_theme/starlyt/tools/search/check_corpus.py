#!/usr/bin/env python3
"""Check the generated search corpus at its public build boundary."""

from __future__ import annotations

import argparse
import json
from pathlib import Path


def documents(site: Path) -> list[dict[str, object]]:
    corpus = site / "assets" / "search-data.json"
    with corpus.open(encoding="utf-8") as source:
        payload = json.load(source)
    assert isinstance(payload, dict)
    entries = payload.get("documents")
    assert isinstance(entries, list)
    return entries


def text(document: dict[str, object]) -> str:
    headings = document.get("headings", [])
    assert isinstance(headings, list)
    return " ".join(
        [str(document.get("title", "")), str(document.get("body", ""))]
        + [str(heading.get("text", "")) for heading in headings if isinstance(heading, dict)]
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("site", type=Path)
    args = parser.parse_args()

    records = documents(args.site)
    values = "\n".join(text(record) for record in records)
    urls = {str(record.get("url")) for record in records}

    assert "/docs/" in urls
    assert "/docs/guides/output-document/" in urls
    assert any("post-body-needle" in value for value in values.splitlines())
    assert "regular-body-needle" in values
    assert "collection-body-needle" in values
    assert "inline-code-needle" in values
    assert "fenced-code-needle" in values
    assert "collection-code-needle" in values
    assert "Café" in values
    assert "excluded-body-needle" not in values
    assert "unpublished-body-needle" not in values
    assert "non-output-body-needle" not in values
    assert "Skip to content" not in values

    overview = next(record for record in records if record["url"] == "/docs/")
    headings = overview["headings"]
    assert isinstance(headings, list)
    assert {heading["id"] for heading in headings if isinstance(heading, dict)} >= {"exact-section-id"}


if __name__ == "__main__":
    main()

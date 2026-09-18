#!/usr/bin/env python3
"""Build and verify Starlyt's localized consumer contract."""

from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import tempfile
from pathlib import Path


ROOT = Path(__file__).resolve().parents[2]
FIXTURE = ROOT / "fixtures/localization/site"
INVENTORY = ROOT / "fixtures/localization/message-keys.json"
THEME_IGNORES = shutil.ignore_patterns(".git", ".scratch", ".poc", "evidence", "fixtures")


def install_fixture(destination: Path) -> Path:
    site = destination / "site"
    shutil.copytree(FIXTURE, site)
    (site / "_theme").mkdir()
    shutil.copytree(ROOT, site / "_theme/starlyt", ignore=THEME_IGNORES)
    return site


def build(jigyll: Path, site: Path, destination: Path, baseurl: str = "") -> subprocess.CompletedProcess[str]:
    command = [str(jigyll), "build", "-s", str(site), "-d", str(destination)]
    if baseurl:
        command.append(f"--baseurl={baseurl}")
    return subprocess.run(command, text=True, capture_output=True, check=False)


def require_success(result: subprocess.CompletedProcess[str]) -> None:
    assert result.returncode == 0, result.stdout + result.stderr


def require_failure(result: subprocess.CompletedProcess[str], message: str) -> None:
    assert result.returncode != 0, "build unexpectedly succeeded"
    assert message in result.stdout + result.stderr, result.stdout + result.stderr


def corpus(page: str) -> dict[str, object]:
    match = re.search(
        r'<script id="search-corpus" type="application/json">(.*?)</script>',
        page,
        re.DOTALL,
    )
    assert match, "localized page has no inline search corpus"
    return json.loads(match.group(1))


def catalog_messages(path: Path) -> dict[str, str]:
    stack: list[tuple[int, str]] = []
    messages: dict[str, str] = {}
    for raw_line in path.read_text(encoding="utf-8").splitlines():
        if not raw_line.strip() or raw_line.lstrip().startswith("#"):
            continue
        indent = len(raw_line) - len(raw_line.lstrip(" "))
        key, separator, value = raw_line.strip().partition(":")
        assert separator and indent % 2 == 0, f"unsupported catalog line: {raw_line}"
        while stack and stack[-1][0] >= indent:
            stack.pop()
        path_parts = [part for _, part in stack] + [key]
        if value.strip():
            messages[".".join(path_parts)] = json.loads(value.strip())
        else:
            stack.append((indent, key))
    return messages


def check_inventory() -> None:
    expected = set(json.loads(INVENTORY.read_text(encoding="utf-8")))
    sources = "\n".join(
        path.read_text(encoding="utf-8")
        for pattern in ("_layouts/*.html", "_includes/**/*.html", "assets/*.js")
        for path in ROOT.glob(pattern)
    )
    used = set(re.findall(r"key=['\"](starlyt(?:\.[a-z][a-z0-9_]*)+)['\"]", sources))
    assert used <= expected, f"unlisted message keys: {sorted(used - expected)}"
    assert expected == used, f"unused message keys: {sorted(expected - used)}"
    catalogs = {
        locale: catalog_messages(FIXTURE / f"_data/locales/{locale}/messages.yml")
        for locale in ("en", "fr", "ar")
    }
    for locale, messages in catalogs.items():
        actual = {key for key in messages if key.startswith("starlyt.")}
        assert actual == expected, f"{locale} catalog mismatch: missing={sorted(expected - actual)} extra={sorted(actual - expected)}"
    for key, fallback in re.findall(r"key='(starlyt(?:\.[a-z][a-z0-9_]*)+)' default='([^']*)'", sources):
        assert catalogs["en"][key] == fallback, f"English fallback mismatch for {key}"
    template_literals = (
        "Skip to content",
        "Menu",
        "Table of contents",
        "On this page",
        "Overview",
        "Search documentation",
        "Cancel",
        "Select theme",
        "Dark",
        "Light",
        "Auto",
        "Retry",
        "Page not found",
        "Back to home",
        "Note",
        "Tip",
        "Caution",
        "Danger",
    )
    for pattern in ("_layouts/*.html", "_includes/**/*.html"):
        for path in ROOT.glob(pattern):
            for line in path.read_text(encoding="utf-8").splitlines():
                for literal in template_literals:
                    if literal in line:
                        assert "private/ui-message.html" in line, f"template UI literal bypasses catalog: {path}:{literal}"
    browser_sources = "\n".join(path.read_text(encoding="utf-8") for path in ROOT.glob("assets/*.js"))
    for literal in (
        "Copy to clipboard",
        "Copied!",
        "Copy failed",
        "Enter words to search this documentation.",
        "Loading search…",
        "No results found. Try another term.",
        "Section titled “",
    ):
        assert literal not in browser_sources, f"browser UI literal bypasses catalog: {literal}"


def check_localized_build(jigyll: Path, temporary: Path) -> None:
    site = install_fixture(temporary / "localized")
    root = temporary / "localized-root"
    prefix = temporary / "localized-prefix"
    require_success(build(jigyll, site, root))
    require_success(build(jigyll, site, prefix, "/docs"))

    french = (root / "fr/accueil/index.html").read_text(encoding="utf-8")
    arabic = (root / "ar/dalil/index.html").read_text(encoding="utf-8")
    prefixed = (prefix / "fr/demarrage/index.html").read_text(encoding="utf-8")
    prefixed_arabic = (prefix / "ar/dalil/index.html").read_text(encoding="utf-8")
    optional = (root / "optional/index.html").read_text(encoding="utf-8")
    solo = (root / "ar/solo/index.html").read_text(encoding="utf-8")

    assert '<html lang="fr" dir="ltr"' in french
    assert '<html lang="ar" dir="rtl"' in arabic
    assert '>Aller au contenu</a>' in french
    assert 'aria-label="Rechercher"' in french
    assert '>Choisir le thème<' in french
    assert 'data-copy-label="Copier dans le presse-papiers"' in french
    assert '>خطر</span>' in arabic
    assert 'id="search-input" type="search" dir="auto"' in arabic
    assert '<table>' in arabic
    assert 'src="/assets/rtl-layout.svg"' in arabic
    assert 'data-tabs' in arabic
    assert 'class="file-tree not-content"' in arabic
    assert "abcdefghijklmnopqrstuvwxyz-0123456789-ABCDEFGHIJKLMNOPQRSTUVWXYZ" in arabic
    assert 'href="/fr/accueil/" aria-current="page"' in french
    assert 'href="/fr/demarrage/"' in french
    assert 'rel="canonical" href="https://example.test/fr/accueil/"' in french
    assert french.count('<p class="print-canonical">https://example.test/fr/accueil/</p>') == 1
    assert prefixed.count('<p class="print-canonical">https://example.test/docs/fr/demarrage/</p>') == 1
    assert 'hreflang="x-default" href="https://example.test/"' in french
    assert 'href="/docs/fr/accueil/"' in prefixed
    assert 'src="/docs/assets/search.js"' in prefixed
    assert 'https://example.test/docs/fr/demarrage/' in prefixed
    assert 'src="/docs/assets/rtl-layout.svg"' in prefixed_arabic
    assert not (root / "ar/facultatif/index.html").exists()
    french_picker = re.search(r'<details id="language-picker"[^>]*>(.*?)</details>', french, re.DOTALL)
    optional_picker = re.search(r'<details id="language-picker"[^>]*>(.*?)</details>', optional, re.DOTALL)
    prefix_picker = re.search(r'<details id="language-picker"[^>]*>(.*?)</details>', prefixed, re.DOTALL)
    assert french_picker and optional_picker and prefix_picker
    assert "Français — documentation détaillée" in french_picker.group(1)
    assert 'aria-current="page"' in french_picker.group(1)
    assert set(re.findall(r'href="([^"]+)"', french_picker.group(1))) == {"/", "/ar/al-bidaya/"}
    assert set(re.findall(r'href="([^"]+)"', optional_picker.group(1))) == {"/fr/facultatif/"}
    assert "العربية — وثائق تفصيلية طويلة" not in optional_picker.group(1)
    assert set(re.findall(r'href="([^"]+)"', prefix_picker.group(1))) == {"/docs/guide/", "/docs/ar/dalil/"}
    assert not re.search(r'href="[^"]*[?#]', french_picker.group(1) + optional_picker.group(1) + prefix_picker.group(1))
    assert 'id="language-picker"' not in solo

    french_titles = {item["title"] for item in corpus(french)["documents"]}
    arabic_titles = {item["title"] for item in corpus(arabic)["documents"]}
    assert french_titles == {"Accueil français", "Guide français", "Page française facultative"}
    assert arabic_titles == {"البداية العربية", "الدليل العربي", "صفحة عربية منفردة"}


def check_nonlocalized_build(jigyll: Path, temporary: Path) -> None:
    site = temporary / "single/site"
    site.mkdir(parents=True)
    (site / "_theme").mkdir()
    shutil.copytree(ROOT, site / "_theme/starlyt", ignore=THEME_IGNORES)
    (site / "_config.yml").write_text(
        "title: Single language\ntheme: starlyt\nnavigation:\n  - label: Home\n    link: /\ndefaults:\n  - scope: {path: \"\"}\n    values: {layout: default}\n",
        encoding="utf-8",
    )
    (site / "index.md").write_text("---\ntitle: Home\npermalink: /\n---\n\nOrdinary English site.\n", encoding="utf-8")
    output = temporary / "single-output"
    require_success(build(jigyll, site, output))
    page = (output / "index.html").read_text(encoding="utf-8")
    assert '<html lang="en" data-theme="dark"' in page
    assert ">Skip to content</a>" in page
    assert 'aria-label="Search"' in page
    assert (output / "assets/search-data.json").exists()
    assert 'id="language-picker"' not in page


def scenario(jigyll: Path, temporary: Path, name: str, mutate) -> subprocess.CompletedProcess[str]:
    site = install_fixture(temporary / name)
    mutate(site)
    return build(jigyll, site, temporary / f"{name}-output")


def check_failures(jigyll: Path, temporary: Path) -> None:
    def remove_menu(site: Path) -> None:
        for locale in ("en", "fr", "ar"):
            catalog = site / f"_data/locales/{locale}/messages.yml"
            catalog.write_text(re.sub(r"^  menu:.*\n", "", catalog.read_text(encoding="utf-8"), flags=re.MULTILINE), encoding="utf-8")

    require_failure(
        scenario(jigyll, temporary, "missing-message", remove_menu),
        "starlyt.menu",
    )

    def expose_missing_key(site: Path) -> None:
        remove_menu(site)
        config = site / "_config.yml"
        config.write_text(
            config.read_text(encoding="utf-8").replace("  default_language: en\n", "  default_language: en\n  missing_messages: key\n"),
            encoding="utf-8",
        )

    exposed = scenario(jigyll, temporary, "missing-message-key", expose_missing_key)
    require_success(exposed)
    exposed_page = (temporary / "missing-message-key-output/fr/accueil/index.html").read_text(encoding="utf-8")
    assert 'aria-label="starlyt.menu"' in exposed_page

    def remove_required(site: Path) -> None:
        (site / "facultatif.md").unlink()
        config = site / "_config.yml"
        config.write_text(
            config.read_text(encoding="utf-8").replace("  default_language: en\n", "  default_language: en\n  required_translations: [fr]\n"),
            encoding="utf-8",
        )

    require_failure(
        scenario(jigyll, temporary, "missing-required", remove_required),
        'translation_key "optional" is missing required locale "fr"',
    )

    def conflict_labels(site: Path) -> None:
        config = site / "_config.yml"
        config.write_text(
            config.read_text(encoding="utf-8").replace("      - label_key: nav.home\n", "      - label_key: nav.home\n        label: Home conflict\n"),
            encoding="utf-8",
        )

    require_failure(
        scenario(jigyll, temporary, "label-conflict", conflict_labels),
        "localized filter accepts at most one target locale",
    )

    def unknown_route(site: Path) -> None:
        config = site / "_config.yml"
        config.write_text(config.read_text(encoding="utf-8").replace("        link: /guide/\n", "        link: /unknown/\n"), encoding="utf-8")

    require_failure(
        scenario(jigyll, temporary, "unknown-route", unknown_route),
        "/unknown/",
    )


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("--jigyll", type=Path, default=Path("jigyll"))
    args = parser.parse_args()
    jigyll = args.jigyll.resolve()
    assert jigyll.is_file(), f"Jigyll binary not found: {jigyll}"
    check_inventory()
    with tempfile.TemporaryDirectory(prefix="starlyt-localization-") as directory:
        temporary = Path(directory)
        check_localized_build(jigyll, temporary)
        check_nonlocalized_build(jigyll, temporary)
        check_failures(jigyll, temporary)
    print("localization checks passed")


if __name__ == "__main__":
    main()

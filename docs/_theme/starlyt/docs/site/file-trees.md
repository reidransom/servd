---
title: File trees
permalink: /file-trees/
project_tree:
  - name: src
    type: directory
    open: true
    comment: Primary source directory
    children:
      - name: components
        type: directory
        open: false
        comment: Closed initially; contains another focusable directory disclosure
        children:
          - name: internal
            type: directory
            open: true
            children:
              - name: helper.TS
                type: file
      - name: index.js
        type: file
        highlight: true
        comment: Application entry point
      - name: tree-only-search-needle.xyz
        type: file
        comment: This illustrative text must not enter the article search corpus
  - name: empty-open
    type: directory
    open: true
    children: []
  - name: empty-closed
    type: directory
    children: []
  - name: README.md
    type: file
    comment: Documentation
  - name: astro.config.mjs
    type: file
  - name: data.JSON
    type: file
  - name: .gitignore
    type: file
    comment: Dotfiles use the generic file icon
  - name: Dockerfile
    type: file
  - name: archive.oddity
    type: file
    comment: Unknown extension uses the generic file icon
  - name: LICENSE
    type: file
  - type: omission
  - type: omission
    label: additional generated files
  - name: "punctuation <safe> & long-file-name-that-needs-local-horizontal-space.YML"
    type: file
    highlight: true
    comment: "A deliberately long comment with punctuation: alpha, beta, gamma, and a second phrase that wraps at the reference comment breakpoint."
deep_tree:
  - name: level-1
    type: directory
    open: true
    children:
      - name: level-2
        type: directory
        open: true
        children:
          - name: level-3
            type: directory
            open: true
            children:
              - name: level-4
                type: directory
                open: true
                children:
                  - name: level-5
                    type: directory
                    open: true
                    children:
                      - name: level-6
                        type: directory
                        open: true
                        children:
                          - name: level-7
                            type: directory
                            open: true
                            children:
                              - name: level-8
                                type: directory
                                open: true
                                children:
                                  - name: level-9-is-truncated.txt
                                    type: file
invalid_tree:
  - name: missing-type.txt
  - type: unknown
    name: unknown-kind
  - type: file
    name: ""
  - type: directory
    name: missing-children
---

File trees render explicit structured front matter as native directory disclosures and static file rows. File-tree text is illustrative UI and is excluded from Starlyt's article search corpus.

## Project structure

{% include components/file-tree.html items=page.project_tree %}

Pass a page-owned array to `file-tree.html`:

```yaml
project_tree:
  - name: src
    type: directory
    open: true
    children:
      - name: index.js
        type: file
        highlight: true
        comment: Application entry point
  - type: omission
    label: additional generated files
```

```liquid
{% raw %}{% include components/file-tree.html items=page.project_tree %}{% endraw %}
```

`name` and `comment` are escaped plain text. `open` and `highlight` are enabled only by literal `true`; both otherwise default to false. File and directory items require a nonempty `name`; directories also require `children`. Invalid items and unknown types are omitted. Omission labels default to `…`.

File extensions use the frozen bounded registry and match the final suffix case-insensitively. Exact known filenames retain their registered icons. Dotfiles and unknown or unregistered extensions use the generic file icon. File-tree icons are decorative; text, native disclosure controls, and highlight shape preserve meaning without them.

## Eight-level limit

Exactly eight directory levels render below. Descendants beyond level eight become one omission row so indentation remains bounded.

{% include components/file-tree.html items=page.deep_tree %}

## Invalid items

This data renders an empty file-tree container because every item is invalid:

{% include components/file-tree.html items=page.invalid_tree %}

Directory summaries use native `details` behavior and need no JavaScript. Toggling one directory does not alter another, and descendants of a collapsed directory leave the keyboard sequence. Long unbroken names scroll locally inside the component instead of widening the page; comments wrap below 30em and stay beside the entry at wider widths. Without JavaScript the disclosure controls still work; without CSS the source remains nested lists and native details in order.

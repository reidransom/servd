---
title: Code blocks
permalink: /code/
---

Fenced code is rendered by Jigyll and highlighted by Chroma. Starlyt styles that output and adds a clipboard control when the browser supports it. The blocks on this page are the examples themselves, not a separate test fixture.

## Author a fenced block

Use a language name after the opening fence when you want highlighting:

````markdown
```yaml
title: My documentation
baseurl: ""
```
````

The rendered result:

```yaml
title: My documentation
baseurl: ""
```

Compare the two blocks: copying the outer example includes the Markdown fence, while copying the YAML block yields only its code text.

## Add a code frame

Jigyll 1.13.0 adds fenced-code UI metadata after the language. A nonempty
double-quoted `title` creates an editor frame for ordinary languages:

```javascript title="site.config.js"
export const theme = "starlyt";
```

An explicit frame does not require a title:

```yaml frame="editor"
theme: starlyt
```

A title infers a terminal frame for `bash`, `sh`, `shell`, `console`,
`powershell`, and `ps1`:

```bash title="Installing dependencies…"
jigyll build --source docs
```

Use `frame="terminal"` to select terminal presentation explicitly:

```console frame="terminal"
$ jigyll --version
```

Use `frame="none"` to retain the existing unframed output:

```json frame="none"
{"frame":false}
```

Explicit `frame` wins over language inference. `title` and `frame` may appear
in either order. Inside their double-quoted values, `\\` represents a literal
backslash and `\"` represents a literal quote:

```text title="A \"quoted\" title with a \\ path and enough additional text to remain bounded on a narrow screen"
The title is outside this copied source.
```

Unknown metadata keeps Jigyll's compatibility behavior and does not create a
frame:

```text future="retained"
This remains an ordinary code block.
```

Malformed recognized metadata fails the build with the source path and line.
Errors include an unquoted or empty title, an unsupported escape, duplicate
`title` or `frame` tokens, an unterminated value, an unsupported frame name,
and combining a title with `frame="none"`.

## Mark lines and text

Markers annotate authored source without changing it. A bare line selector is
neutral; `ins` and `del` identify inserted and deleted lines. Selectors are
1-based, inclusive, and accept comma-separated numbers and ranges:

```javascript {1,2} ins={2-3,4} del={5}
const stable = true;
const added = "new";

return added;
const removed = "old";
```

The blank third line is intentionally inserted. The neutral and inserted
selectors overlap on line 2, so insertion presentation wins. The adjacent
insertion ranges render continuously.

Double-quoted selectors mark every exact, case-sensitive text occurrence.
Prefix them with `ins=` or `del=` for insertion and deletion meaning:

```go "return value" ins="added" del="removed" ins="value()"
added := "added"
return value
removed := "removed"
call(value())
```

Text matching can cross Chroma token spans, including the keyword and name in
`return value`, without changing their order or classes. Punctuation is literal,
not a regular expression. Repeated text is marked at every occurrence.

Marker text uses the same escapes as titles. Here the single selector marks
both paths:

```text "C:\\work"
Copy C:\work, then compare C:\work.
```

Marked long lines remain local to their code scroller:

```json ins={1} showLineNumbers
{"marker":"inserted","workflow":["author the exact source","select a semantic region","render Chroma tokens on the server","preserve every character for clipboard and ordinary selection","scroll this marked line without widening the page"]}
```

Zero, reversed, or out-of-range lines; empty selectors; missing text; malformed
recognized values; and overlapping insertion/deletion selections fail the
build with the source path and fence line. Neutral overlap is valid. Markers do
not add `+`, `-`, labels, or any other characters to copied and selected code.

## Show line numbers

Add the bare `showLineNumbers` flag to start a gutter at 1. Blank lines receive
one number, while the structural trailing newline does not create another:

```text showLineNumbers
first

third
```

Use `startLineNumber=N` to start from a decimal value between 1 and 999999. It
also enables numbering, so no second flag is required. This framed example
demonstrates the 9-to-10 width transition together with every whole-line marker
meaning:

```bash title="deploy.sh" startLineNumber=8 {1} ins={2} del={3}
printf 'stable\n'
printf 'added\n'
printf 'removed\n'
```

Numbers are CSS-generated from each line's `data-line-number`; they are not
text inside `code`. Selecting or copying the block yields only the authored
source, and horizontal scrolling keeps the numbered gutter aligned. Existing
blocks without either numbering token remain unnumbered.

`showLineNumbers` cannot take a value. Duplicate tokens, a missing or
nondecimal start, and starts outside 1 through 999999 fail the build with the
source path and fence line.

## Preserve whitespace

Copy the following Python into a plain text editor. Indentation, the blank line, and the source's line breaks should remain intact:

```python
def greeting(name):
    message = f"Hello, {name}"
    return message

print(greeting("Starlyt"))
```

Starlyt reads the rendered `code.textContent` for copying. It does not trim the text, reconstruct it from colored spans, or include the copy button's label. What matters is the code a reader can paste, not the highlighter's internal token markup.

## Keep long lines local

This deliberately long JSON line is useful for inspecting horizontal scrolling inside a block. Scroll the block rather than widening the entire document:

```json title="wide.json"
{"site":"Starlyt","workflow":["write ordinary Markdown","install the theme through Jigyll","build the separate consumer","read the generated documentation","try the navigation and clipboard controls at a narrow window width"],"automatedTesting":false}
```

The copy control should remain usable even when the code extends beyond the visible block. Use a keyboard to reach the block and its control as well as a pointer.

## Plain code

A fence without a language is also valid:

```
Write
  Build
    Read
```

Plain code should remain readable in Dark and Light modes. Highlighting does not add meaning to every kind of text, and a language label should describe the source rather than be chosen only for its colors.

## Clipboard availability

The control is called **Copy to clipboard**. It becomes available with JavaScript and the browser Clipboard API, normally on HTTPS or a trustworthy local origin such as `http://127.0.0.1`.

When writing succeeds, the feedback is **Copied!**. If the browser rejects the write, it is **Copy failed**; failure must not be presented as success. When the Clipboard API is unavailable, there is no copy button. You can still select and copy text normally.

Try copying a block and pasting it into an editor. If your browser allows per-site clipboard permissions, denying access is a useful optional way to inspect the failure message. There is no requirement to automate that check.

## Highlighting scope

Chroma's token boundaries and colors can differ from the highlighter used by stock Starlight. Readable highlighting is not proof of exact syntax-token visual parity. The theme does not replace Jigyll's renderer to recover token distinctions it never emitted.

Playgrounds and Astro/MDX code APIs are not promised by this surface. Use the
documented fenced-code metadata above and [documented Markdown](../markdown/)
rather than assuming upstream component syntax works here.

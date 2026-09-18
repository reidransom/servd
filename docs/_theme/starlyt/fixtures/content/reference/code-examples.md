---
title: Code examples
---
These examples use native fenced-code highlighting. Visit the [overview](../../) or [long guide](../../guides/long-guide/).

## JavaScript

```javascript
// Preserve indentation and line breaks.
const greeting = "Hello, reader";
function greet(name) {
  return `Hello, ${name}!`;
}
console.log(greet("Starlyt"));
```

## JSON

```json
{
  "title": "Starlyt Docs",
  "enabled": true,
  "navigation": ["Overview", "Long guide", "Code examples"]
}
```

## Shell

```sh
# Build an ordinary consumer site.
jigyll build --baseurl /docs
printf '%s\n' "A deliberately long line that scrolls inside the code block without widening the document or changing the copied source."
```

## Plain text

```
<article data-example="literal">
  & This is text, not executable markup.
    indentation stays intact
</article>
```

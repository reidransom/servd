---
title: Development
permalink: /development/
---

Run the ordinary Go verification workflow from the repository root:

```sh
go build ./... && go vet ./... && go test ./... -race
```

The repository also provides `just build`, `just lint`, `just test`, `just test-race`, and `just all`. Documentation is a Jigyll consumer: build it with `jigyll build --source docs` and do not add a Node.js, Sass, or second documentation toolchain.

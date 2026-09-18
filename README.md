# servd

[![CI](https://github.com/reidransom/servd/actions/workflows/ci.yml/badge.svg)](https://github.com/reidransom/servd/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/reidransom/servd.svg)](https://pkg.go.dev/github.com/reidransom/servd)
![Go Version](https://img.shields.io/github/go-mod/go-version/reidransom/servd)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Run and manage many local development servers at once. servd gives registered web projects stable backend ports and routes them by exact hostname through one local proxy.

## Get started

```sh
brew install reidransom/tap/servd
servd version
```

See the [full servd documentation](docs/) for installation methods, quick start, command selection, hostname routing, dashboard operation, automation, and remote access.

## Contributing

```sh
go build ./... && go vet ./... && go test ./... -race
```

[MIT licensed](LICENSE). Maintained by [r2ware](https://r2ware.dev).

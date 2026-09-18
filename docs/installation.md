---
title: Installation
permalink: /installation/
---

Choose one installation method. Confirm the exact installed build afterwards with `servd version`.

## Homebrew

macOS and Linux:

```sh
brew install reidransom/tap/servd
```

## Scoop

Windows:

```powershell
scoop bucket add reidransom https://github.com/reidransom/scoop-bucket
scoop install reidransom/servd
```

## Release archive

Download the archive for your operating system and architecture and `checksums.txt` from the [latest GitHub release](https://github.com/reidransom/servd/releases/latest). macOS and Linux releases use `.tar.gz`; Windows releases use `.zip`.

Verify the downloaded archive against its matching entry in `checksums.txt` before extraction:

```sh
# macOS or Linux
shasum -a 256 servd_Darwin_arm64.tar.gz
```

```powershell
# Windows
Get-FileHash .\servd_Windows_x86_64.zip -Algorithm SHA256
```

The archive contains `servd` (`servd.exe` on Windows), `README.md`, and `LICENSE`.

## Go install

Use Go 1.25.4, the version declared in [`go.mod`](https://github.com/reidransom/servd/blob/main/go.mod), or a newer version:

```sh
go install github.com/reidransom/servd/cmd/servd@latest
```

## Verify the build

Every installation method reports version, commit, and build date without reading configuration or starting the dashboard:

```sh
servd version
```

Continue with the [quick start](../quick-start/).

---
title: Migration guidance
permalink: /migration/
---

Migration is explicit. servd keeps current supported registrations and legacy compatibility keys where documented, but it does not infer a replacement command or hostname alias.

## Command migration

Use a repository command in a root `.servd.toml` or a one-registration explicit command. Convert Procfile entries and framework detection to a command you own. Existing `launchers.toml` and former custom rules are ignored. Replace retired `servd __static` with `servd static`.

## Hostname migration

Use one scalar `hostnames.tlds` primary and an optional explicit `hostnames.tlds_fallback` list. See [Proxy ports and hostnames](../proxy-hostnames/) for the complete mapping, stale hosts-file inspection, and restart procedure.

## Compatibility cleanup

Legacy `enabled` site keys and `default_enabled` settings keys are ignored and may be removed. Runtime state is internal: do not migrate it by editing `state.json`.

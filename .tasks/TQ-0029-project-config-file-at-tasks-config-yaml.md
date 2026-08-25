---
id: TQ-0029
title: Project config file at .tasks/config.yaml
status: todo
priority: normal
labels:
  - component/config
  - feature
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Proposal

Add a project configuration file at `.tasks/config.yaml`, committed next to the
tasks it configures. This ticket is only the file, its loader and its contract;
the first thing to live in it is the label set (see the dependent ticket).

```yaml
# .tasks/config.yaml
version: 1
```

## Contract

- `version` is an integer, starts at `1`, and only changes on a **breaking**
  change. Everything else is additive: a new key must be readable by an older
  binary, which ignores what it does not know.
- A config that declares a version newer than the binary supports is a clear
  error ("config version 2 needs a newer tq"), not a silent partial read.
- The file is **optional**. A missing config means built-in defaults, so
  `tq add` in a fresh repository keeps working with no setup step, exactly as
  it does today.
- `tq init` writes it if it is missing and leaves an existing one alone —
  the same idempotent treatment `.tasks/AGENTS.md` gets, except that this file
  is user-owned, so it must never be overwritten.
- One loader in `config.go`, shared by the CLI and the HTTP server, read from
  disk per command and per request like tasks are. No cache, no watcher.
- `GET /api/config` exposes it to the board.

## Notes

- YAML gotcha for whoever implements the labels ticket: `color: #ff0000` is a
  **comment** in YAML, so that value parses as null. Hex colours have to be
  quoted, and the loader should reject an empty colour rather than render it.
- `config.yaml` is not `TQ-*.md`, so `Store.List` already ignores it. Worth a
  test so that stays true.
- Decide: tolerate unknown keys silently (best for forward compatibility, which
  is the stated goal) or warn once on stderr. Task frontmatter is strict
  (`KnownFields(true)`) precisely to avoid silent data loss on rewrite, but the
  config is never rewritten by tq, so tolerating is defensible here.

## Acceptance criteria

- No config: every command behaves exactly as it does today.
- `tq init` creates `.tasks/config.yaml` with `version: 1` and does not touch an
  existing one.
- A malformed config names the file and the line; a future version says to
  upgrade; neither is reported as a task error.
- CLI and server read the same loader; `GET /api/config` returns it as JSON.
- README and the generated `.tasks/AGENTS.md` describe the file.

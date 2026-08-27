---
id: TQ-0087
title: 'The marker is the source of truth: stop re-deriving config from the task directory'
status: todo
priority: urgent
labels:
  - bug
  - component/store
  - component/config
created: 2026-08-27T19:14:58+02:00
updated: 2026-08-27T19:14:58+02:00
---

## The rule

**The marker file is the source of truth.** A command walks up from the working
directory until it finds `.taskqueue.yaml`. Once it has that file it knows
everything: the board, the priorities, the labels, and where the task files
live. Nothing may ever walk *from* the task directory back to the config — the
task directory is an output of the marker, not a way to find it.

Today the code does the opposite in five places, and the queue silently loses
its configuration whenever `path:` points outside the marker's own directory.

## Reproduced on HEAD c41ae4c

`tq init` **generates the broken shape itself** — no hand-edited config needed:

    $ cd proj && TQ_DIR=../queue tq init
    $ cat proj/.taskqueue.yaml
    version: 1
    path: ../queue          <- written by tq init, outside the marker's directory
    columns: ... priorities: ... labels: ...

With a project declaring its own columns (`backlog`, `doing`, `shipped`):

    status on disk:                      doing
    $ tq move TQ-0001 doing
    error: invalid status "doing" (want one of inbox, todo, in-progress, done, rejected)
                                         <- the project's own column, refused

    $ tq update TQ-0001 --assignee alice <- nothing to do with status
    Updated TQ-0001: Real work
    status on disk now:                  inbox    <- silently rewritten, exit 0

Changing an assignee destroys the task's position on the board. The configured
priorities and labels are lost the same way, on every command.

## Root cause

`Store` keeps only `Dir`, the task directory, and discards the marker it was
resolved from. Five call sites then re-derive the config by walking up *from the
task directory*, which never reaches a marker that lives elsewhere:

- `internal/store/store.go:379` (`Priorities`)
- `internal/store/store.go:390` (`Columns`)
- `internal/web/server.go:241`
- `internal/cli/label.go:61`
- `internal/cli/cli.go:254` (`cli.config`)

`config.FindConfig` then returns `(nil, nil)` — "no marker found" is
indistinguishable from "no config" — so every caller silently falls back to the
built-in defaults, and `columns.Normalize` rewrites any status outside them.

## What to build

- **Resolve the marker once, carry it.** Discovery already knows where the
  marker is when it resolves the task directory; the `Store` must keep it
  instead of throwing it away. Every consumer of the config reads it from there.
- **Delete the re-derivation.** No caller may walk up from `Store.Dir` to find a
  config. After this change, searching the tree for `FindConfig(st.Dir)` or
  equivalent should return nothing.
- **Stop conflating "none" with "not found."** `FindConfig` returning
  `(nil, nil)` is what makes the failure silent. A caller that resolved through
  a marker must never afterwards decide there is no config.
- **A `path:` outside the marker's directory must keep working.** It is
  documented, and `tq init` writes it under `TQ_DIR`. This task makes it work,
  not forbidden.

## Acceptance

- A project whose `path:` points outside the marker's directory keeps its board,
  priorities and labels on every command.
- `tq move <id> <a column the project declares>` is accepted.
- An unrelated edit (`--assignee`, `--title`, a note) never changes `status`.
- The same holds over HTTP: `/api/tasks`, `/api/status` and the board's columns.
- A test that fails if any code path re-derives the config from the task
  directory, so this cannot come back.

## Notes

Found by the code review during TQ-0022, which called it the highest-value fix
available and recommended fixing it in discovery rather than per call site.
Verified independently before filing.

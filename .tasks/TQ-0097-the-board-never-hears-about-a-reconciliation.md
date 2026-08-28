---
id: TQ-0097
title: The board never hears about a reconciliation the server could not write
status: todo
priority: normal
labels:
  - bug
  - component/api
  - component/frontend
created: 2026-08-28T13:42:07+02:00
updated: 2026-08-28T13:42:07+02:00
---

## Finding

TQ-0088 gave the store an optional `Announce func(Reconciliation)` hook so a
reconciliation pass can say what it moved. The CLI wires it — every store `tq`
opens gets one, and the result goes to stderr. **The web server does not.**

Source: `internal/web/server.go:58`.

## How it fails

Reconciliation returns a `Reconciliation{Moved, Unfinished}` rather than an
error, deliberately, so that a queue it cannot write stays readable (that was a
HIGH finding in TQ-0088's own review). The consequence is that a refused write
is reported only through `Announce`.

With no hook on the web server, a stranded task whose reconciliation cannot be
written — a read-only checkout, a permissions problem, a file another process
holds — is returned with the status the file holds, which is a column the board
does not draw. The card renders in **no column**: not an error, not a warning,
just absent from the board while `tq list` in a terminal names it.

The board already has the surfaces for this. `/api/status` carries `unreadable`,
`incomplete` and `duplicated` (TQ-0011, TQ-0012, TQ-0040), each surfaced as a
toast plus a persistent footer segment. Reconciliation has no equivalent.

## Suggested fix

Wire `Announce` in the web server and carry the result the way the three
existing signals are carried: a field on `/api/status`, a toast when it newly
appears, and a footer segment while it lasts. Follow that pattern rather than
inventing a second one.

Decide also what the board should do with a task whose status names no column —
drawing it nowhere is the current behaviour and is the part users would notice
first.

## Acceptance

- A reconciliation the server could not write is visible on the board, not
  silent.
- A task whose status names no column is accounted for on screen rather than
  disappearing.
- The CLI's existing stderr reporting is unchanged.

Found by the code review during TQ-0054, against TQ-0088's committed work
(`13aae03`). Not independently reproduced — **confirm before fixing.**

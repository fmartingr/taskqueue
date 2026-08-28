---
id: TQ-0096
title: Reconciliation rewrites the queue from a config that may be half-written
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-28T13:41:46+02:00
updated: 2026-08-28T13:41:46+02:00
---

## Finding

TQ-0088 made reading reconcile: a command that finds a task filed in a column
the config no longer declares moves every stranded task to the default column
and says so. Reads writing was accepted deliberately — the alternative is
displaying a status the file does not hold.

What was **not** accepted is reconciling against a configuration that is not the
project's. Two ways that happens:

**A half-written `.taskqueue.yaml`.** The marker is read from disk on every call
(TQ-0087). A file caught mid-save parses as a smaller board, or a board with
different names, so every task not in it looks stranded — and `List` rewrites
all of them. The event stream already treats a half-saved marker as the case
that matters (`AGENTS.md`: "The config reading is a stat, never a parse: a file
caught half-saved has to register as a change like any other, and the board is
what decides what to do about one it cannot use"). Reconciliation has no
equivalent caution.

**A branch switch.** `git checkout` swaps `.taskqueue.yaml` and the task files
as two separate writes. A `tq list` landing between them reconciles the outgoing
tasks against the incoming board, or the reverse. The rewrite is permanent and
`git status` then shows every task file modified.

Source: `internal/store/store.go:604`.

## Suggested fix

Reconcile only when the configuration is one the project actually meant. Options,
not exclusive:

- Require the marker to be stable across the pass — read it, scan, read it again,
  and reconcile only if it did not move. `List` already has this shape for the
  task directory (TQ-0012's consistency check), so the machinery exists.
- Do not reconcile from a read at all when the stranded set is *everything* —
  a board that strands every task is far more likely to be a half-written file
  than a deliberate edit.
- Reconcile lazily on read (report, do not write) and let `tq init` do the
  writing, which the user's decision already makes the documented path.

The last is closest to what was decided for TQ-0088 and the smallest change, but
it weakens the safety net that decision asked for. Worth deciding explicitly.

## Acceptance

- A `.taskqueue.yaml` caught mid-write does not cause a queue-wide rewrite.
- A branch switch does not leave every task file modified.
- The TQ-0088 property still holds: a genuinely removed column still moves its
  tasks to the default, and no surface ever displays a status the file does not
  hold.

Found by the code review during TQ-0054, against TQ-0088's committed work
(`13aae03`). Not independently reproduced — **confirm before fixing.**

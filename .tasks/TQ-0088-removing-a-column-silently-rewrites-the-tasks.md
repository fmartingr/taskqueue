---
id: TQ-0088
title: Removing a column silently rewrites the tasks left in it, one edit at a time
status: todo
priority: high
labels:
  - bug
  - component/store
  - component/config
created: 2026-08-27T22:25:46+02:00
updated: 2026-08-27T22:25:46+02:00
---

## Wanted

**Removing a column from `.taskqueue.yaml` moves every task still in it to the
default column** — all of them, at once, visibly. Not lazily, not per-task, and
never while claiming on screen that it has already happened.

## Reproduced on HEAD d007202

A project with columns `backlog` (default), `review`, `shipped`, and three tasks
filed in `review`. The user then deletes `review` from their own config:

    $ tq list
    ID       STATUS   PRIORITY  ASSIGNEE  TITLE
    TQ-0001  backlog  normal    -         One      <- all three DISPLAY as backlog
    TQ-0002  backlog  normal    -         Two         while all three are still
    TQ-0003  backlog  normal    -         Three       `review` on disk

    $ tq update TQ-0001 --assignee bob               <- one unrelated edit

    .tasks/TQ-0001-one.md:status: backlog            <- persisted, silently
    .tasks/TQ-0002-two.md:status: review             <- untouched
    .tasks/TQ-0003-three.md:status: review           <- untouched

Two defects, not one.

**1. The read path lies.** `tq list` renders a normalised status without saying
it differs from the file. The board shows a column the task is not in.

**2. The write path migrates piecemeal.** `update` persists
`columns.Normalize(t.Status)` (`internal/store/store.go:1002`), so an edit that
had nothing to do with status rewrites it — for that one task only. Which tasks
have migrated then depends on which ones happened to be edited since the config
changed, and nothing records that it happened. Exit 0 throughout.

Priority does not behave this way: a value outside the vocabulary is refused on
write rather than silently rewritten.

## Why it matters

`tq update --assignee` destroying a task's position on the board is the same
class as TQ-0087, reached a different way — by editing your own configuration,
which is a documented thing to do. It is a regression from the configurable
columns work; before that the status set was closed and could not drift.

## The open decision

tq never removes a column itself — the user edits the YAML by hand — so tq has
to notice the drift. What triggers the migration?

- **On any command that reads the config**: makes a read command write, which
  this project avoids.
- **Report on read, migrate on an explicit command** (recommended). Reads never
  lie and never write; a named command does the migration in one pass and says
  what it moved. `tq init` is already documented as idempotent and is the
  natural home, or a dedicated verb.
- **Migrate on the next write, but completely** — the first write after the
  drift moves every affected task, not just the edited one. Keeps it automatic,
  but a `tq note` would silently rewrite a dozen unrelated tasks.

## Acceptance

- No command displays a status that differs from the file without saying so.
- An unrelated edit (`--assignee`, `--title`, a note) never changes `status`,
  whatever the config says.
- After the migration runs, every task that was in a removed column is in the
  default column, on disk, and the user was told which tasks moved.
- A task in a removed column is never left in a state where some of the queue
  migrated and some did not.
- The same holds over HTTP and on the board.

## Notes

Found by the code review during TQ-0087, which flagged it as the same shape as
that ticket with a different cause. Reproduced independently before filing.
Related: `internal/store/store.go:636` and `:810` also normalise on the read
side; check them together.

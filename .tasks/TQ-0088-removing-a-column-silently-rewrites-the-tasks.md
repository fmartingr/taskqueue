---
id: TQ-0088
title: Removing a column silently rewrites the tasks left in it, one edit at a time
status: done
priority: high
labels:
  - bug
  - component/store
  - component/config
created: 2026-08-27T22:25:46+02:00
updated: 2026-08-28T12:48:28+02:00
---

## Decision

**Removing a column moves every task still in it to the default column** — all of
them, in one pass, and the user is told what moved.

That logic lives in **reconcile**. Two ways it runs:

1. **`tq init` reconciles.** Editing `.taskqueue.yaml` means running `tq init`
   again. That is the documented workflow, and it must be written down —
   `tq init` is already mandatory and idempotent, so this is one more thing it
   settles rather than a new command to learn.
2. **Automatically, on detection.** If any command finds a task filed in a column
   the config no longer declares, it reconciles there and then. This is the
   safety net for a config edited without running `tq init`, and it is what stops
   the queue ever being half-migrated.

Accepted consequence, stated deliberately: a command that only reads — `tq list`
— can now write. That is the price of never displaying a status the file does
not hold. It is not the current bug: today's defect is that a read normalises
for *display* while an unrelated write persists it for *one* task, so the queue
drifts into a split state nobody is told about. Reconciliation is complete and
announced.

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

## Acceptance

- No command displays a status that differs from the file without saying so.
- An unrelated edit (`--assignee`, `--title`, a note) never changes `status` by
  itself; only reconciliation moves a task between columns.
- After reconciliation, every task that was in a removed column is in the
  default column, on disk, and the user was told which tasks moved.
- A task in a removed column is never left in a state where some of the queue
  migrated and some did not.
- `tq init` reconciles, and the docs say to run it after editing the config.
- The same holds over HTTP and on the board.

---

## Notes

Found by the code review during TQ-0087, which flagged it as the same shape as
that ticket with a different cause. Reproduced independently before filing.
Related: `internal/store/store.go:636` and `:810` also normalise on the read
side; check them together.
- 2026-08-28T12:27:02+02:00 — Implemented reconcile in the store.

  - task.Columns.Reconcile(status) is the rule: an alias resolves to the column it names, a status with no column goes to Default() — not First(). Normalize is now alias-resolution only and returns anything else unchanged, so the first-column fallback that would have marked tasks done on a done-first board is gone from the type entirely. Columns.Rank now ranks an unknown status last, matching Priorities.Rank.
  - Store.Reconcile()/reconcile() moves every stranded task in one pass and hands the moves to Store.Announce, an optional callback. The CLI wires it to stderr for every store it opens, so --json stdout stays pure and no API/JSON contract changes.
  - Removed Normalize from the three read/write paths the ticket named: List (:640), Get (:814) and update (:1002). A save now writes the status it was handed, the same way it already left the priority alone.
  - List reconciles when its scan finds a stranded task, then scans again. Get reconciles the whole queue when the task it read is stranded. tq init reconciles.
- 2026-08-28T12:32:34+02:00 — Docs updated in three places: README.md gets a new "After editing .taskqueue.yaml, run tq init again" section plus a pointer from the quick start and a rewritten paragraph under Columns; AGENTS.md gets two rules (reconcile, and only-reconciliation-moves-a-task); internal/guide/AGENTS.tmpl.md gets the same instruction in Rules of the format, regenerated into .tasks/AGENTS.md by running tq init.

  Side effect worth flagging: reconciling also rewrites an alias to the column it names, because "no command displays a status that differs from the file" covers a file that says backlog while every surface shows inbox. Running tq init in this repo therefore moved TQ-0068, TQ-0069, TQ-0070 and TQ-0071 from status: backlog to status: inbox — the four tasks whose files still carried the pre-rename spelling. Those file changes are part of this commit.
- 2026-08-28T12:47:48+02:00 — Code review (one pass, effort high) found six issues. Fixed in this diff:

  1. HIGH — a failed reconciliation returned before Announce, so tasks already moved were rewritten in silence. Store.Reconcile now returns a Reconciliation{Moved, Unfinished} and no error at all; every exit goes through one announce() so a partial pass is always reported, and the pass carries on past a refused write instead of stopping at the first.
  2. HIGH — List and Get hard-failed on a queue that cannot be written (read-only checkout, root-owned .tasks), making it unreadable. Neither fails on a reconciliation now: the tasks come back carrying exactly what their files hold, and what could not be written is named on stderr, exit 0. Verified by hand and pinned at store, cli and integration level.
  3. MEDIUM — the "Default and not First" comment overclaimed: Columns.Default() falls back to First() when no column declares default: true, so on such a board a stranded task can land in a consider_done column. Not a defect of this change — that board already files new tasks there, by the documented rule — so the comment and the README now say it plainly and two tests pin it.
  4. LOW — reconcile is a locate plus a save per stranded task, under the lock. Accepted and documented in AGENTS.md: one pass after a config edit, and rare.
  5, 6. LOW — two README inaccuracies ("three things" listing two; overstating that tq show reconciles a queue whose stranded task is not the one being shown). Both corrected.

---
id: TQ-0035
title: Columns defined in the project config
status: todo
priority: normal
labels:
  - config
  - cli
  - frontend
depends_on:
  - TQ-0029
created: 2026-08-25T12:11:41+02:00
updated: 2026-08-25T12:11:41+02:00
---

## Proposal

Move the board's columns into `.tasks/config.yaml`. They are the `status`
vocabulary, hard-coded today as `backlog`/`todo`/`in-progress`/`done` at
`task.go:45`, and the order is the board order:

```yaml
version: 1
columns:
  - name: inbox
    display_name: Inbox
  - name: todo
    display_name: To do
  - name: in-progress
    display_name: In Progress
    ready: false
  - name: done
    display_name: Done
    ready: false
    satisfies_dependencies: true
  - name: rejected
    display_name: Rejected
    ready: false
```

- `name` is what lives in frontmatter (`status: todo`); `display_name` is what
  the board shows.
- A sequence rather than a map, for the same reason as severities in TQ-0032:
  the order is meaningful and a YAML map does not preserve it.

## Strict, unlike labels

Columns and severities are closed sets. Labels are freeform suggestions
(TQ-0030), so the three keys behave differently on purpose:

- `tq move`, `tq add --status`, `POST /api/tasks` and `PATCH /api/tasks/{id}`
  reject a status that is not configured and list the valid ones.
- **A task sitting in a column that no longer exists moves to the first
  column.** It is shown there everywhere — board, `tq list`, filters, sorting —
  and the correction is written to the file the next time that task is saved.
  Reads must not write, so a listing never rewrites files; the fix rides along
  with the next real update.

## The semantics currently welded to "done" and "in-progress"

This is the real work; renaming the strings is the easy half.

- `IsReady` (`task.go:114`) excludes the literals `done` and `in-progress`.
- `IsBlocked` (`task.go:105`) counts a dependency as satisfied only when its
  status is literally `done`.
- `tq done` (`cli.go:340`) moves to the literal `done`, and `tq show` labels a
  dependency complete the same way (`cli.go:314`).

Two optional flags per column replace all of that:

- `ready: false` — tasks here are never offered by `tq ready`, because they are
  either claimed or finished. Defaults to true.
- `satisfies_dependencies: true` — a dependency parked in this column counts as
  complete. `tq done` targets it, and fails with a clear message if no column or
  more than one column claims it.

Note the deliberate asymmetry in the defaults above: **Rejected does not satisfy
dependencies.** A task depending on rejected work stays blocked until someone
edits the dependency, which is the honest outcome rather than silently treating
"we will not do this" as done.

## Defaults and migration

The built-in set, for a project with no config, becomes Inbox, To do, In
Progress, Done, Rejected. That renames `backlog` to `inbox`, so every existing
task with `status: backlog` becomes an unknown status and would be swept into
the first column by the rule above — the same place, but silently.

Recommendation: accept `backlog` as a deprecated alias for `inbox` for one
release so nothing moves without being asked, and let the auto-move rule handle
genuinely unknown values.

## Everything that hard-codes the four statuses today

- `task.go:45` `Statuses`, plus `ValidStatus`, `statusRank`, the validation
  error at `task.go:75` and the filter check at `task.go:147`.
- `cli.go:91` usage text, `cli.go:346` the `tq move` error, `cli.go:314`
  dependency rendering, `cli.go:340` `tq done`.
- `agents.go:213` — the generated `.tasks/AGENTS.md` prints the status list to
  agents, so it has to print the configured one.
- `frontend/app.ts:9` the `STATUSES` const and the `Status` type, and
  `app.ts:248` which maps it to columns.
- `frontend/index.html` — four hard-coded status `<select>` blocks (filter bar,
  task dialog, create dialog) and the quick-add composer's implied column.
- `frontend/style.css:165` `grid-template-columns: repeat(4, ...)` and the
  responsive breakpoint below it, which both assume exactly four columns.

## Acceptance criteria

- No config: the five built-in columns, with `backlog` still accepted as an
  alias for `inbox`.
- A configured set drives the board order and names, write validation, sorting,
  the CLI help and the generated agent guide.
- A task in a removed column appears in the first column and is corrected on its
  next write, never by a read.
- `tq ready` and dependency completion follow the column flags rather than the
  strings `done` and `in-progress`.
- `tq done` targets the column marked `satisfies_dependencies`, and says so
  clearly when there is not exactly one.
- Round-trip test with a custom set — three columns, unusual names — covering
  create, move, ready, the board and the generated guide.

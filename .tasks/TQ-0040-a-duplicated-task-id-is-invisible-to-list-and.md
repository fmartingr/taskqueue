---
id: TQ-0040
title: A duplicated task ID is invisible to list and the board
status: todo
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T13:18:46+02:00
updated: 2026-08-25T13:18:46+02:00
---

## Finding

`Store.List` does not go through `locate`: it reads every entry independently, so two files carrying the same `id` both parse and List returns two tasks with that ID and a nil error.

## How it fails

`tq list` and `GET /api/tasks` return both copies; `IndexTasks` (task.go) and the board's `indexTasks` collapse them to whichever comes last in ReadDir order, so the board renders two cards sharing one dataset id and clicking either gets a 500 `invalid_task_file` from PATCH. A stale `status: todo` duplicate can also mask a `done` one in `tq ready`. Nothing reports the broken invariant until someone touches that ID.

## Suggested fix

Have `List` detect IDs claimed by more than one file and report them the way `locate` already does, rather than silently returning duplicates.

Found by the /code-review pass on TQ-0006.

---
id: TQ-0075
title: Bun tests for the pure helpers in app.ts
status: done
priority: normal
labels:
  - tests
  - component/frontend
created: 2026-08-25T22:39:54+02:00
updated: 2026-08-25T22:52:10+02:00
---

## Why

`frontend/notes.ts` has `bun test` unit tests; `frontend/app.ts` has none, and
several of its functions are pure and worth pinning. This needs no dependency
and no browser, so it is worth doing whether or not the browser layer lands.

## Scope

The functions that take data and return data, around `frontend/app.ts:120-136`:

- `indexTasks` — keying tasks by ID.
- `pendingDependencies` — which of a task's dependencies are not done.
- `isReady` — the same rule the CLI's `ready` implements, duplicated in the
  frontend. Worth testing precisely because it is a second implementation: the
  two disagreeing is a real failure mode.
- `visibleTasks` — the filter the board applies.

They are unexported today, so this needs a small export or a `frontend/board.ts`
they move into. Prefer moving them: the more of the board's decisions live
outside the DOM layer, the less there is that only a browser can test.

## Acceptance criteria

- `make test-frontend` covers them alongside the notes helpers.
- `isReady` is checked against the same cases as the Go implementation,
  including a missing dependency, which blocks.
- No new dependency.

---

## Notes

- 2026-08-25T22:50:40+02:00 — Moved the four pure helpers out of app.ts into a new frontend/board.ts, along with the
  types they need (Task, Filters, Status, Priority, STATUSES, PRIORITIES). visibleTasks
  was the only one that was not already pure: it read the module-level state, so it now
  takes (tasks, filters). app.ts imports them back and passes state at the call site.
  The built bundle is the same code relocated, so the board behaves identically.
- 2026-08-25T22:51:21+02:00 — board.test.ts pins all four helpers, and isReady is driven by the same fixture and the
  same seven cases as TestReady in internal/task/task_test.go — missing dependency
  included. Checked the two implementations against each other for real as well, by
  building that fixture with the CLI and comparing 'tq ready --json' with
  visibleTasks(tasks, {ready: true}) over 'tq list --json': both return TQ-0002, TQ-0003,
  TQ-0008 and agree task by task. The rules do not disagree today.
  Also pinned that readiness is judged against every task, not the visible ones, so a
  status filter hiding a dependency cannot make it look missing.
- 2026-08-25T22:52:07+02:00 — Updated AGENTS.md where it enumerates which frontend files have unit tests, since it
  said 'currently notes.ts'.
  Left alone, and worth a ticket: the root public/ directory is still tracked
  (app.js, icon.png, index.html, style.css) after TQ-0072 moved the build output to
  internal/web/public. Nothing reads it — go:embed and DevDir both point at the new
  path and CI only staleness-checks that one — but a 'make dev' started before the
  refactor keeps rewriting root public/app.js from its in-memory build.ts, so it shows
  up as a spurious modification whenever frontend/ changes. Reverted it rather than
  committing churn to a dead directory.

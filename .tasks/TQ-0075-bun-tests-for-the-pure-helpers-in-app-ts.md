---
id: TQ-0075
title: Bun tests for the pure helpers in app.ts
status: todo
priority: normal
labels:
  - tests
  - component/frontend
created: 2026-08-25T22:39:54+02:00
updated: 2026-08-25T22:39:54+02:00
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

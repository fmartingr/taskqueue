---
id: TQ-0080
title: The board's coverage gaps the Vue migration left
status: todo
priority: normal
labels:
  - tests
  - component/frontend
created: 2026-08-26T13:12:38+02:00
updated: 2026-08-26T13:12:38+02:00
---

## Finding

A mutation pass over the migrated board found four areas where breaking the code
leaves the whole suite green.

- **`CreateDialog` submit is untested at every layer.** Only opening it is
  exercised. Breaking `splitList` there — so "backend, auth" is stored as one
  label — passes all 39 browser tests. The status/priority defaults, the
  refresh, the confirmation toast and the on-open focus all ship unverified.
- **`Toasts` is never asserted.** Rendering nothing passes the whole suite,
  while every error path in `state.ts` and all three dialogs reports through it.
  The cheapest reliable trigger is the `Created <id>` info toast, which a
  create-dialog test picks up for free.
- **The filter bar's wiring.** `#filter-status`, `#filter-assignee`,
  `#filter-ready`, `#filter-reset` and `#status-line` are never touched in a
  browser. The filter *rules* are well covered in `frontend/board.test.ts`; what
  is not covered is the `v-model` wiring and the `statusLine` computed, which
  lives in `state.ts` and so has no unit-test home.
- **Opening a card from the keyboard** (Enter/Space on a focused card). A
  pre-existing gap, but re-implemented in `Card.vue`, so it is migration surface
  now. Split Enter and Space into separate tests — one test doing Enter, Escape,
  then Space is flaky for the reason in TQ-0080.

## Suggested fix

Four browser tests, one per bullet. They belong in `browser/`, not in a
component-mount layer: this project has no such layer and adding one is an
architecture change AGENTS.md would have to approve first.

Found by the review of TQ-0076.

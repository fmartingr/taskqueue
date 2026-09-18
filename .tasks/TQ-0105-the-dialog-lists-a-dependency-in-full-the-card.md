---
id: TQ-0105
title: The dialog lists a dependency in full, the card only counts them
status: done
priority: normal
labels:
  - feature
  - component/frontend
created: 2026-09-18T10:19:18+02:00
updated: 2026-09-18T10:48:06+02:00
---

A dependency on the board is a bare ID today, in both places it appears, and an
ID answers none of the questions somebody has about it: what is it, and is it
done. `TQ-0002` in the dialog's Depends on row tells you nothing without
opening a second board, and `Blocked by TQ-0002, TQ-0003, TQ-0009` on a card
spends three lines saying what a number says in one.

The board already holds the answer. `index` in `frontend/state.ts` is every
task the listing carried, keyed by ID, and `columns` is the project's board —
so a dependency's title and the display name of its status are one map lookup
away. No new endpoint, no extra fetch, nothing in Go.

## The dialog: a list, not chips

`#task-depends-on` keeps its `TokenField` and everything it does — ＋ adds,
✕ removes, one entry at a time, written as a whole list through
`saveDependencies` (TQ-0069's rule is untouched). What changes is what the
`#default` slot draws. Today it is `<span class="token-id">{{ value }}</span>`;
it becomes a row carrying, for each dependency:

- the ID,
- the task's title,
- its status, as `columnDisplay(status, columns)` spells it — the board's own
  wording, so a renamed column reads the same here as in the header,
- a link to that task.

Because the chip's content is a slot, none of this touches `TokenField`'s write
path: the rows are still drawn from `values`, which is still what the file
holds.

A dependency the listing does not carry — a missing one, an ID nothing answers
to — renders as the ID alone and says it is not in the queue. That is not a
rendering gap to paper over: `pendingDependencies` counts a missing dependency
as blocking, deliberately, and the row is where a user finds out why a task
will not go ready.

### Following the link

Clicking a row opens that task: set `openTaskID` to its ID. `App.vue` keys the
dialog on `openTask.id` precisely so the ref can be pointed at another task
without passing through nothing, so the machinery is already there.

One thing it must get right, and it is the same hazard as the click-outside
rule in `TaskDialog.vue`: **an open editor is not thrown away by a
navigation.** Losing focus writes, and that write can be refused — switching
the dialog on the same click would take the text a refusal exists to preserve
with it. Follow what `onClick` does: while `.inline-editor, .note-editor,
.token-input` is open, the click settles the editor and does not navigate.

Whether the address should carry the open task is a separate question and not
this task's — nothing in the board links to a task today.

## The card: a number

`Card.vue`'s note keeps the trigger it has: it appears only while something is
unmet, and `pendingDependencies` stays the rule behind both it and the `blocked`
class. Only the text changes, from the joined IDs to a count:

    Blocked by 3 tasks
    Blocked by 1 task

A card whose dependencies are all done stays as clean as it is today. The IDs
are one click away in the dialog, which is the half of this task that makes
dropping them from the card safe.

## Acceptance

- The dialog's Depends on row shows, per dependency: ID, title, status, and a
  link that opens that task's dialog.
- A dependency that is not in the queue is shown as such rather than omitted.
- A click that would navigate while an editor is open settles the editor
  instead, and writes nothing twice.
- Adding and removing a dependency still works, still one at a time, still
  refused on a collision.
- A blocked card reads `Blocked by N task(s)` and carries no IDs.

## Tests

- `browser/board.test.ts:433` — "a blocked card says what it is waiting for, in
  the board and the dialog" — asserts the card's `.blocked-note` contains the
  blocker's ID. That assertion is the behaviour being changed, so it changes:
  the card is checked for the count, the dialog for the ID *and* the title and
  the status. Add to it the click that follows a dependency to its own dialog.
- Anything pure that comes out of this — a "3 tasks"/"1 task" formatter, or
  resolving a dependency to a row — belongs beside the other pure helpers with
  `bun test` cases; the components stay rendering and events.
- `make typecheck`, `make frontend` (commit `public/`), `make test-frontend`,
  `make test-browser`.

---

## Notes

- 2026-09-18T10:48:06+02:00 — Implemented.

  Pure half, unit-tested: board.ts gains Dependency + resolveDependencies, and
  pendingDependencies is now that function filtered — the 'a missing dependency
  blocks' rule is written once. format.ts gains taskCount ('1 task'/'3 tasks').

  Dialog: DependencyRow.vue draws one row — ID, status, title, in that order,
  with the status pill at a fixed 92px so every title starts at the same column
  (asked for mid-implementation). TokenField keeps the writes; only its slot
  changed, plus a .token-rows modifier that stacks the chips. A dependency the
  listing does not carry is a row of plain text saying 'not in the queue', not a
  link: pointing openTaskID at an ID the board has no task for would leave the
  dialog showing the task it was on, since state.ts's watcher only reassigns
  openTask when the listing has the ID.

  Following a row sets openTaskID; App.vue's :key does the rest. It stands down
  for an open editor the same way the click outside does, and reuses the same
  mousedown: the dialog records whether an editor was open when the press began,
  because the blur that press causes can take the editor out of the DOM before
  the click arrives.

  Card: 'Blocked by 3 tasks' — same trigger as before (pending > 0), no IDs.

  Also removed, on request: the dialog's own '#task-blocked' line. The list says
  which dependency is pending per row, so that line was repeating the IDs it
  sits above.

  Gates: make typecheck, make frontend (public/ committed), make test-frontend
  (276 pass), browser/board.test.ts 23 pass, make lint 0 issues, make build.
  make test is green except TestAScanFailureIsReportedAgainToAFreshBoard, which
  is TQ-0104 and untouched by this. The full make test-browser run is flaky under
  its own load in a way that predates this: a clean stash of this branch failed 7
  of 106 in the same way (config.test.ts and regressions.test.ts timeouts), and
  every file passes when run on its own.

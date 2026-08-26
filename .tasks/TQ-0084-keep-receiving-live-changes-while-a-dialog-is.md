---
id: TQ-0084
title: Keep receiving live changes while a dialog is open
status: todo
priority: normal
labels:
  - feature
  - component/frontend
created: 2026-08-26T14:24:52+02:00
updated: 2026-08-26T14:24:52+02:00
---

## What happens today

`/api/events` is live and the board reacts to it, except when the user is doing
something. `frontend/state.ts:113` computes:

```ts
export const busy = computed(() =>
  dragging.value !== null || composing.value !== null ||
  openTaskID.value !== null || creating.value);
```

and `onTasks` (`state.ts:271`) sets a `queued` flag instead of refreshing while
`busy`, replaying it when `busy` clears. So with a dialog open the board behind
it is frozen, and the dialog itself never hears that its own task changed —
`browser/events.test.ts:41` pins exactly that: "a change that arrives while a
dialog is open is applied when it closes".

A dialog is a layer *above* the board, though. Nothing moves under the user's
hand when a card appears behind a backdrop, and a modal can stay open for
minutes while an agent works.

## The plan

**1. Split `busy` by what it actually protects.**
A drag genuinely cannot survive a re-render; that stays. A dialog does not need
the board frozen, so the board list refreshes while one is open. Keep the
composer's guard for now — it holds a draft and focus — and treat it as a
separate question.

**2. Let the open task receive its own updates.** `TaskDialog.vue` already
holds `opened`/`baseline` snapshots for the save-time merge (TQ-0010), which is
most of the machinery. On an incoming change to the open task:

- **Notes**: append-only, so a note that is not in the local list is new and
  should appear in the panel immediately — that is the whole point when an agent
  is running `tq note` against the task on screen.
- **Untouched fields**: adopt the incoming value. Title, status, priority,
  labels and dependencies the user has not edited should show what is on disk.
- **Edited fields**: never silently overwritten. Show a quiet notice in the
  dialog ("changed on disk") and leave the user's text alone.

**3. Advance `baseline` when the dialog adopts something.** This is the subtle
part and the one most likely to go wrong: the save-time merge is defined against
what the dialog opened with, so adopting a note without recording it in the
baseline risks double-adding on save. The merge and the live adoption have to
agree on one snapshot.

**4. Handle the task disappearing.** The open task can be deleted or renamed
away underneath. Say so in the dialog rather than letting Save fail with a 404.

## Risks worth naming

- **Focus and caret**: adopting a value into a field the user is *in* is not
  acceptable even if the field is technically not dirty yet; treat focus as
  dirty.
- **Event storms**: `queued` is a single boolean and should stay that way. A
  signal is not a payload; ten events between renders are one refresh.
- **The create dialog** has no server-side counterpart, so it needs none of
  this — only the board behind it should refresh.

## Tests

- `browser/events.test.ts:41` is the behaviour being replaced. It becomes: a
  change that arrives while a dialog is open is applied **while it is open**.
- `browser/regressions.test.ts:42` still has to pass — the save-time merge is
  still correct — but its comment ("the poll stands down while the dialog is
  open, so the board cannot know about these") stops being true and must be
  rewritten, not left to mislead the next reader.
- New: a note written by the CLI appears in the open panel; an edited field
  survives an incoming change to another field; an incoming change to an edited
  field shows the notice and keeps the local text; deleting the open task is
  reported in the dialog.

## Acceptance criteria

- With a task dialog open, cards created, moved and retitled elsewhere appear on
  the board behind it without closing it.
- Notes written to the open task appear in its panel while it is open.
- No edit in progress is lost or silently overwritten, and the save-time merge
  keeps working.
- Dragging still defers the refresh until the drop.

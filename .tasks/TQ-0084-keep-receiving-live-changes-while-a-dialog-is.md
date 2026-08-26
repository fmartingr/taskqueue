---
id: TQ-0084
title: Keep receiving live changes while a dialog is open
status: todo
priority: normal
labels:
  - feature
  - component/frontend
created: 2026-08-26T14:24:52+02:00
updated: 2026-08-26T14:43:24+02:00
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

**1. A drag is the only thing that defers a refresh.**
A native drag genuinely cannot survive a re-render; that guard stays. A dialog
does not need the board frozen, and neither does the composer — `busy` collapses
to `dragging`.

The composer needs work to earn that. Its draft is component-local
(`Composer.vue:22`, `const draft = ref("")`), so a re-render that unmounts it
loses what was typed and the caret with it. Either lift the draft into the
shared state next to `composing`, or key the column's list so a refresh cannot
unmount an open composer. Whichever, the test is that typing through an
incoming change loses neither the text nor the caret position.

**2. Let the open task receive its own updates.** `TaskDialog.vue` already
holds `opened`/`baseline` snapshots for the save-time merge (TQ-0010), which is
most of the machinery. On an incoming change to the open task:

- **Notes**: append-only, so a note that is not in the local list is new and
  should appear in the panel immediately — that is the whole point when an agent
  is running `tq note` against the task on screen.
- **Any field the user has not edited**: adopts the incoming value. Title,
  status, priority, labels and dependencies all show what is on disk until the
  user touches them.
- **Edited fields**: never silently overwritten. Show a quiet notice in the
  dialog ("changed on disk") and leave the user's text alone.
- One refinement of that rule, flagged rather than smuggled in: a field that is
  **focused but not yet edited** should also be left alone. Replacing the value
  under someone's caret while they are reading or about to type it is the same
  surprise the rule exists to prevent. Say so in the implementation, and drop
  the refinement if it proves to be more trouble than the surprise.

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
- **The composer's own guard was load-bearing**: dropping it is what makes the
  draft and caret question real, rather than a detail.

## Tests

- `browser/events.test.ts:41` is the behaviour being replaced. It becomes: a
  change that arrives while a dialog is open is applied **while it is open**.
- `browser/regressions.test.ts:42` still has to pass — the save-time merge is
  still correct — but its comment ("the poll stands down while the dialog is
  open, so the board cannot know about these") stops being true and must be
  rewritten, not left to mislead the next reader.
- `browser/poll.test.ts:64`, "the poll stands down while the composer is open,
  and catches up after it", is now the opposite of the intended behaviour and is
  replaced: the board updates while the composer is open, and the draft and
  caret survive it.
- New: a note written by the CLI appears in the open panel; a field the user has
  not edited adopts an incoming change; an edited field survives one and shows
  the notice; deleting the open task is reported in the dialog.

## Acceptance criteria

- With a task dialog open, cards created, moved and retitled elsewhere appear on
  the board behind it without closing it.
- Notes written to the open task appear in its panel while it is open.
- No edit in progress is lost or silently overwritten, and the save-time merge
  keeps working.
- Typing in a quick-add composer survives a change arriving mid-draft, text and
  caret intact.
- Dragging is the only thing left that defers a refresh, until the drop.

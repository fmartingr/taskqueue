---
id: TQ-0027
title: Clicking a second note's pencil does nothing while one is being edited
status: done
priority: normal
labels:
  - bug
  - component/frontend
depends_on:
  - TQ-0076
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T13:13:15+02:00
---

## Finding

startEditingNote's blur handler calls finish() -> renderNotes(), which rebuilds every note <li>, so the click that caused the blur is swallowed and clicking a second note's pencil while one is being edited does nothing.

Source: `app.ts:598`

## How it fails

VERIFIED with real mouse input in Chromium against a running `tq serve`: on a task with three notes, click note 1's pencil (editor opens), then click note 2's pencil once. Result: no editor open anywhere. The mousedown fires blur on the editor, finish(true) calls renderNotes() at app.ts:586 which replaces all children of #task-notes, so the button under the pointer is detached before mouseup and no click is ever dispatched. The user must click twice; the same swallowed click affects any control inside #task-notes.

## Suggested fix

Commit the edit on mousedown, or re-render only the note that changed so the click target is not detached before mouseup.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-26T10:31:16+02:00 — Blocked on TQ-0076 (migrate the board to Vue 3), which lists this bug in its acceptance criteria and names it as a symptom of rebuilding the whole notes list by hand. TQ-0076 requires it fixed deliberately with a regression test, not assumed fixed because a keyed re-render no longer detaches the click target.
- 2026-08-26T13:13:15+02:00 — Fixed by TQ-0076's migration, and it needed both halves.

  The pencil acts on mousedown with the default prevented, so no focus change and no blur under the pointer; and finish(keep, position) is keyed to the note it belongs to, so the blur from the torn-down textarea cannot close the editor that was just opened. Reverting either half alone still fails.

  Covered by browser/regressions.test.ts, including a third click to prove the panel keeps working. The review added a second test for the keyboard path: the @click beside the @mousedown is load-bearing, not redundant, because Enter on a focused button fires no mousedown — removing it as a tidy-up left the suite green and made the pencil mouse-only.

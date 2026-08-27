---
id: TQ-0079
title: The dialog still overwrites body content written while it was open
status: done
priority: high
labels:
  - bug
  - component/frontend
created: 2026-08-26T13:12:38+02:00
updated: 2026-08-27T13:35:47+02:00
---

## Finding

TQ-0010 was closed by having the task dialog re-read the file before saving and
merge the *notes* (`TaskDialog.buildPatch` -> `notes.mergeNotes`). The rest of
the body is not merged: `content.value` is still the snapshot taken when the
dialog opened, so a CLI or agent edit to the non-note part of a body is
overwritten by Save exactly as it was before.

Not a regression — the board did this before the Vue migration too, and the
notes half was the reported symptom — but the ticket's framing implies the whole
body is safe, and it is not.

## How it fails

Open a task in the board. In another terminal, edit that task's body above the
notes rule (an agent revising a Finding, say). Change only Priority in the
dialog and Save: the body reverts to what it was when the dialog opened.

## Suggested fix

`buildPatch` already re-reads the task. Either merge the content half too — a
three-way merge is hard for free text, so more likely detect that it changed and
say so — or send no `body` at all when the content was not edited, and let the
notes ride on their own patch. The second is smaller and closes the common case.

Found by the review of TQ-0076.

---

## Notes

- 2026-08-27T13:22:17+02:00 — Fixed in two halves, both in the frontend — no API change.

  notes.ts gains mergeBody(opened, edited, current): the notes half still merges note by note, the content half never mixes. Untouched in the dialog and the file's content stands; touched here and not there, the dialog's stands; touched on both sides it returns "conflict". When nothing about the body moved it returns "unchanged" and TaskDialog.buildPatch leaves the body field out of the PATCH altogether, so a save that changed only Priority cannot touch content someone else wrote. A nil Body was already "leave unchanged" server-side (task.TaskPatch), so nothing on the Go side had to change.

  On a conflict buildPatch returns null, the save is refused whole (body and every other field), the textarea keeps the user's typing, and a toast — the same stack TQ-0011/0012/0040 use — says the body changed on disk and to copy the text and reopen. The listing is refetched quietly so reopening really does start from what is on disk.

  The dialog's baseline is now the whole SplitBody rather than just the notes, and append() resets it from what was written: without that the content half would read as edited for the rest of the dialog's life and the next save would refuse itself.

  Four browser tests in browser/regressions.test.ts under a TQ-0079 heading; all four fail on the code as it was. Nine mergeBody cases in frontend/notes.test.ts. All 59 browser tests, make test, lint, typecheck and build pass.
- 2026-08-27T13:35:23+02:00 — Code review at effort high, three findings, all in this diff, all addressed.

  1. refuse() refetched the listing, and the listing is what App.vue finds this dialog in — replacing it can unmount the component and take the very text the refusal had just told the user to copy. Reachable, not theoretical: a task withheld from a scan is what a file being written twice looks like (TQ-0012, TQ-0040), which is the situation a refusal is already in. The refetch is gone. Closing the dialog releases the change the stream held while it was open, so reopening still starts from the file.

  2. append() is two writes, and the note behind the patch can fail on its own. The baseline was only reset once both had landed, so a failed addNote left the dialog holding a baseline behind a write it had made itself: every later save read its own edit as somebody else's and refused. Both writes now rebase from the task the server returns. Covered by a browser test that fulfils POST /notes with a 500 and then saves; it hangs on the code as it was.

  3. The body the dialog opens with comes from the last listing, not a fresh read, so a change can predate the click. Answered in the wording — "changed on disk since this dialog read it" rather than "while it was open", which is true either way. Not answered by fetching the task on mount: that would reseed the textarea after the dialog is on screen, and the fields being read once at open is the contract the file documents. Worth its own ticket if it ever bites.

  Five browser tests now, all failing on the code as it was. make typecheck, test-frontend (146), test (go), test-browser (60), lint (0 issues) and build all pass.

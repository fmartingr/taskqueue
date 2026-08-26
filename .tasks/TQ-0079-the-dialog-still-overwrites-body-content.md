---
id: TQ-0079
title: The dialog still overwrites body content written while it was open
status: todo
priority: high
labels:
  - bug
  - component/frontend
created: 2026-08-26T13:12:38+02:00
updated: 2026-08-26T13:12:38+02:00
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

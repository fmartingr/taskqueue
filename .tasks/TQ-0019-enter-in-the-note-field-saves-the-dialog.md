---
id: TQ-0019
title: Enter in the note field saves the dialog instead of adding the note
status: todo
priority: high
labels:
  - bug
  - component/frontend
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

The "Append a timestamped note…" input sits inside `<form id="task-form">` (line 62) alongside a `<button type="submit">Save</button>` (line 126), so pressing Enter in it triggers implicit form submission — Save — instead of adding the note, and the typed text is silently discarded.

Source: `index.html:117`

## How it fails

Open a card, type "deploy blocked on TQ-0009" into the note field and press Enter, the natural gesture. HTML implicit submission activates the Save button, so the app.ts:728 submit handler runs saveOpenTask(), which PATCHes the task with the body captured at open time (the note is not in it) and calls taskDialog.close(). The dialog vanishes, nothing was POSTed to /api/tasks/{id}/notes, and the text is gone with no error. Only clicking the separate "Add note" button (type="button", wired at app.ts:718) actually works. Fix: handle keydown Enter on #task-note, or move it out of the form.

## Suggested fix

Handle Enter on the note input directly (add the note, prevent the implicit submit), or move the input out of the form element.

Filed from a `/code-review` pass at max effort.

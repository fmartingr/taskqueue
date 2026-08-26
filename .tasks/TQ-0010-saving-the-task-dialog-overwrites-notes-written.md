---
id: TQ-0010
title: Saving the task dialog overwrites notes written while it was open
status: done
priority: high
labels:
  - bug
  - component/frontend
depends_on:
  - TQ-0076
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T13:13:15+02:00
---

## Finding

patchFromDialog PATCHes the entire body reconstructed from state.openBody — captured when the dialog opened — while the poller is deliberately disabled for as long as the dialog is open, so Save silently overwrites every note the CLI or an agent wrote in the meantime.

Source: `app.ts:616`

## How it fails

User opens TQ-0007; state.openBody is frozen at app.ts:517 and the poll skips every tick because of `taskDialog.open` (app.ts:765), so the snapshot can be arbitrarily stale. An agent then runs `tq note TQ-0007 "..."` five times. The user changes only Priority and clicks Save: patchFromDialog sends `body: joinBody({...state.openBody, ...})`, ApplyPatch (task.go:277) sets t.Body wholesale, and all five notes are erased from the .md file with a 200 OK and no conflict shown to either side. handlePatchTask (server.go:166-171) offers no If-Match or updated precondition to catch it, and addNoteToOpenTask (app.ts:645) issues the same stale patch first, so even "Add note" wipes them. One-directional: `tq update` has no --body flag, so only the board can clobber this way.

## Suggested fix

Re-read the task immediately before saving and merge: keep the server's notes, apply only the content and note edits the user actually made.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-26T10:31:16+02:00 — Blocked on TQ-0076 (migrate the board to Vue 3), which names this bug in its acceptance criteria: it must fix it deliberately, with a regression test, rather than assume a different re-render fixes it. The fix itself — re-read the task and merge before saving — is framework-independent, so of the three bugs TQ-0076 claims this is the one most worth fixing ahead of it if the migration stays in the backlog. It is silent data loss.
- 2026-08-26T13:13:15+02:00 — Fixed by TQ-0076's migration. TaskDialog.buildPatch re-reads the task before saving and merges through the new pure notes.mergeNotes: the file decides which notes exist, the dialog only the wording of ones it opened with, matched on timestamp and original text rather than position.

  Covered by browser/regressions.test.ts (two tests through Save, one through Add note) and 14 unit tests in frontend/notes.test.ts. Verified by reverting the fix: the Save tests then fail with exactly the reported symptom, the note that arrived while editing gone from the file.

  The append path was a real gap the review caught by mutation — buildPatch is called from Save and from Add note, but only Save was ever driven with a stale dialog, so replacing the append path's merge with a snapshot left the whole suite green while a note was erased. Now tested.

  Note the scope: the notes half is closed, the content half is not. Save still writes back the body content captured when the dialog opened, so a CLI edit above the notes rule is still overwritten. That predates this work and is now TQ-0079.

---
id: TQ-0069
title: Trello-like modal
status: done
priority: normal
depends_on:
  - TQ-0076
created: 2026-08-25T18:37:47+02:00
updated: 2026-08-30T20:29:42+02:00
---

We need to improve the modal experience by a lot.

First of all, the modal should not block block background updates of the websocket. The modal opens and shows data, even if that allows us to modify it, but keep receiving updates. Modifying a field will apply the changes directly to the file without needing to click on save. Esc closes the modal.

If a file is changed while we are modifying a field, we will show an error to the user and we wont try to intervine. The user must check the changes itself on VCS.

The body is shown rendered as markdown, and clicking on it opens the editor to modify it. Only if we modify the body we would consider it "edited". Same for the title, do not show it in a textarea, show it as a heading and clicking on it would let users edit it.

The UI would look something like this:

┌──────────────────────────────────────────────────────────────────────────────┐
│ Implement authentication flow                                             ✕  │
│ (In Progress)                                                                │
├───────────────────────────────────────────────────────┬──────────────────────┤
│                                                       │ [ Priority: XXXX ]   │
│ Description                                           │                      │
│                                                       │                      │
│ Add login, logout and token refresh support.          │ [ Members       ]    │
│                                                       │ [ Labels        [+]  │
│ - Support OAuth2                                      │ - Label 1       [x]  │
│ - Persist session                                     │ - Label 2       [x]  │
│ - Handle expired tokens                               │                      │
│ Depends on:                                           │                      │
| - xxxxxx                                              │                      │
│                                                       │ Actions              │
│ Notes                                                 │                      │
│ ┌───────────────────────────────────────────────────┐ │                      │
│ │ Write a note...                             [Send]│ │                      │
│ └───────────────────────────────────────────────────┘ │                      │
│                                                       │                      │
│ - Note 1                                              │                      │
| - Note 2                                              │                      │
│                                                       │                      │
└───────────────────────────────────────────────────────┴──────────────────────┘

---

## Notes

- 2026-08-26T10:31:16+02:00 — Blocked on TQ-0076 (migrate the board to Vue 3). TQ-0076's own note names this ticket: built against app.ts it is imperative DOM that gets ported again; built after, it is a component.
- 2026-08-30T20:18:39+02:00 — Rebuilt the task dialog as a card rather than a form.

  The two design decisions everything else follows from:

  - The dialog holds no draft of the task. Every control is drawn from the task
    the board last read, so it follows the file by *being* it — no adoption
    rules, no per-field dirty flags, nothing local for an incoming change to
    overwrite. The only local state is the one editor the user has open.
  - A field is written when its editor closes, as a partial patch of that field
    alone. That is what makes the first decision safe: a save of every field is
    only ever correct if the dialog holds a current copy of every field, which is
    exactly what it stopped doing.

  Removed with the Save button: frontend/adopt.ts and adopt.test.ts, and
  mergeNotes/mergeBody from notes.ts. Their premise was 'the dialog writes the
  whole body back from a snapshot taken at open time', which no longer exists.
  The properties they protected are kept and still tested — TQ-0010 (a write
  must not take an agent's notes with it), TQ-0079 (a paragraph both sides
  rewrote is refused) — now in frontend/edit.ts, three functions over three
  values, with browser cases driving each.

  New: frontend/markdown.ts, a hand-written renderer for the description.
  Escapes the document first and emits its own tags, so there is nothing left
  from the file to sanitise; a URL is the exception escaping does not cover, so
  a scheme is either on the allow-list or the link stays text. Hand-written
  rather than a dependency because public/app.js is committed and a library
  would be a permanent block in every diff of it, and because the two things one
  would buy — the long tail of CommonMark, and sanitising — are unused by task
  bodies and answered directly.

  Two deliberate departures from the ticket's sketch:

  - No 'Actions' block in the sidebar. There is nothing real to put in one: the
    domain's only action is changing the column, and that is the status control
    the sketch already draws under the title. An empty section named Actions
    would be worse than none.
  - A refused write cannot be retried into success — the editor keeps refusing
    until Escape drops the text. That is the ticket's rule ('we won't intervene;
    the user must check the changes on VCS') taken literally, and Escape putting
    the file's value back is the only way out the dialog offers. If that turns
    out to be too sharp in use, the next step is a 'show me the file's version'
    affordance, not a merge.

  make test / typecheck / test-frontend / test-browser / build all pass;
  test-browser is 101 cases across 11 files. internal/web's
  TestAScanFailureIsReportedAgainToAFreshBoard fails on this machine, and fails
  identically on a clean checkout of main — chmod 000 does not make the
  directory unreadable here — so it is environmental and predates this work.
- 2026-08-30T20:29:42+02:00 — Follow-up on the same ticket: the dialog now fills the window bar a 5%
  margin, and closes on a click in that margin.

  Two things hold the outside click up, and neither is cosmetic:

  - The element carries no padding of its own and everything it draws sits in one
    child (.task-sheet) that fills it. A <dialog>'s backdrop belongs to the
    element rather than being a child of it, so a click on it arrives targeting
    the element — which is only the same thing as 'outside the sheet' while
    nothing else can be. Padding on the dialog would have been a strip of the
    element inside the visible card and indistinguishable from it.
  - The mousedown is checked as well as the click. A click event is dispatched at
    the common ancestor of its mousedown and its mouseup, so selecting the whole
    description and letting go past the edge of the sheet is otherwise exactly a
    click on the backdrop, and would throw the dialog away mid-selection.

  And the exception: a click outside is ignored while an editor is open. Losing
  focus writes, and that write can be refused — closing on the same click would
  take the text a refusal exists to preserve down with it, before the write had
  even answered. The first click outside settles the editor, the next closes the
  dialog. Verified against a PATCH that is aborted outright, which is the worst
  case: the text is only recoverable from the editor it is still sitting in.

  The notes list lost its own 260px scroller with this: the sheet is tall enough
  to be the one thing that scrolls, and two scrollbars for one column of notes
  was the old height's problem, not a choice.

  104 browser cases pass.

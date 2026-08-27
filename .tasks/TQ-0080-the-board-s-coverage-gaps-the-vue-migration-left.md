---
id: TQ-0080
title: The board's coverage gaps the Vue migration left
status: done
priority: normal
labels:
  - tests
  - component/frontend
created: 2026-08-26T13:12:38+02:00
updated: 2026-08-28T01:33:16+02:00
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

---

## Notes

- 2026-08-27T23:48:36+02:00 — Two of the four gaps had already closed by the time this was picked up, and the ticket is stale about them:

  - Toasts: asserted since TQ-0040 — browser/board.test.ts waits on #toasts .toast.error and reads its text (the duplicate-ID and unparsable-file tests).
  - #status-line: asserted since TQ-0011/TQ-0040 in board.test.ts and since TQ-0033/TQ-0034 in browser/events.test.ts.

  The browser suite grew from the 39 tests the ticket counted to 75 across TQ-0011, TQ-0012, TQ-0040, TQ-0079, TQ-0083 and TQ-0084, which is what closed them. No coverage was re-added for either.

  Left open, and what this task covers: the create dialog's submit, the filter bar's wiring, and opening a card from the keyboard.
- 2026-08-27T23:48:45+02:00 — Three areas, five tests, all in browser/ — no component-mount layer, no new dependency, and no production code touched.

  - browser/board.test.ts: the create dialog files a task end to end — the on-open focus, the project's status and priority defaults, "backend, auth" arriving as two labels, the refresh, and the Created <id> toast. The stream is refused for that one test so the dialog's own refresh is the only thing that can have put the card on the board before the toast rendered.
  - browser/board.test.ts: Enter and Space on a focused card, one test each, on a board each — never closing and reopening a dialog (TQ-0081 is rejected, so that flake is permanent).
  - browser/filters.test.ts (new): the v-model wiring of #filter-status, #filter-assignee, #filter-ready and #filter-reset, plus the statusLine computed, which has no unit-test home. The rules stay where they are covered, in frontend/board.test.ts.

  Mutation-checked on a scratch copy, 15 mutations, all killed: splitList joining the list, the missing focus, either default hard-coded, the missing refresh, the missing toast, Enter or Space dropped from Card.onKeydown, each of the four controls unbound, each field reset() forgets, and statusLine always counting the whole queue.
- 2026-08-28T01:25:24+02:00 — Correcting the mutation table in the note above, which was wrong twice.

  It is 18 mutations, not 15, and two of them these tests do NOT kill: hard-coding the create dialog's status to "inbox" and its priority to "normal" leaves the create test green, because the harness's bare marker gives a project whose defaults are exactly those two values. What kills them is browser/columns.test.ts and browser/priorities.test.ts, on projects that declare a vocabulary of their own. The test now says so where it asserts them rather than claiming the credit.

  The 16 that are killed, each by the test named:

  - splitList joined instead of split -> the create test (labels arrive as two)
  - the title never focused on open -> the create test
  - the refresh dropped before the confirmation -> the create test, in 954ms and by causation rather than by timing: every listing is held from the submit, so a dialog that confirms without waiting has nothing to race
  - the confirmation toast dropped -> the create test
  - Enter dropped from Card.onKeydown -> the Enter test
  - Space dropped from Card.onKeydown -> the Space test
  - tabindex dropped from the card -> both key tests, in 1s, naming focus rather than timing out on the dialog
  - each of #filter-status, #filter-assignee, #filter-ready unbound -> its own filter test
  - each of the five fields reset() clears -> the reset test
  - statusLine always counting the whole queue -> all four filter tests

  Two premises in the finding above were also stale. splitList has had unit cases in frontend/format.test.ts since TQ-0076, so only the browser suite was ever blind to it; the browser test's job is the wiring, and the comment says that. And statusLine is a pure computed that a unit test could drive, so the browser assertion is there because the footer is the board's own readout and the synchronisation for what follows, not because there is nowhere else to put it.

  defaultColumn had no unit coverage at all while defaultPriority had three cases; it has three now in frontend/board.test.ts, on a board whose default is not its first column, which is the one thing the built-in board cannot show.
- 2026-08-28T01:25:35+02:00 — Follow-ups this task deliberately left alone, all named by review:

  - #create-depends-on is filled by no test at any layer, so the second splitList call site in CreateDialog.submit ships unexercised. Replacing it with a literal [] survives the suite.
  - statusLine's clauses would be cheaper as unit tests in a frontend/state.test.ts than as browser assertions; state.ts has no top-level side effects, so it imports headless.
  - The stream-abort idiom (page.route on /api/events) is copied verbatim in poll.test.ts and events.test.ts and could be a harness export; the create test no longer needs it, so two sites remain.
  - counts() in browser/filters.test.ts is a local helper; a shared one would suit the other suites that read #status-line.
  - A Project.configure(yaml) helper would de-duplicate the four places that write a marker by hand (columns, labels, config, priorities tests).
  - playwright-core floats at ^1.62.1 while AGENTS.md pins typescript and vue-tsc exactly. These tests lean on press(" "), fill("") on a search input and selectOption(sel, ""), any of which a minor bump could change.

  Also worth knowing for anyone reading a slow or flaky run here: 440 orphaned `yes` processes from an earlier session had the machine at load 576, which is what made bun's spawnSync return an empty stdout and tq add fail to parse. Killing them took browser/board.test.ts from 73s to 5.5s. The suite itself was not flaky: the two files ran clean repeatedly once the load was gone.
- 2026-08-28T01:33:16+02:00 — Committed as 216f48e, unsigned: the 1Password SSH signer (op-ssh-sign) fails with "failed to fill whole buffer" for every signature, including a direct probe outside git, so it needs the app unlocked. Every other commit on this branch is signed. Re-sign with: git commit --amend --no-edit -S

---
id: TQ-0055
title: Stop managing the repository's AGENTS.md and CLAUDE.md
status: done
priority: high
labels:
  - chore
  - component/cli
created: 2026-08-25T16:15:20+02:00
updated: 2026-08-25T16:19:52+02:00
---

## Why

`tq init` writing into the repository's own `AGENTS.md` and `CLAUDE.md` has
been the single largest source of defects in this project. Editing a document
tq did not author means parsing arbitrary Markdown, and every attempt to do
that safely has produced another way to damage a committed file: TQ-0014,
TQ-0042, TQ-0043, TQ-0045, TQ-0046, TQ-0049, TQ-0052. Each fix was correct and
each uncovered the next case.

The value it buys is one line of text a person can write once.

## What

Remove root-document handling entirely. `tq init` writes the guide inside the
task directory and nothing else. In its place it prints, and the README
documents, the one line to add:

    @.tasks/AGENTS.md

Everything that existed only to edit those files goes with it: the section
finder, the fence-aware scanner, the pointer guard, the doc root, the
same-file check.

## Effect on open tickets

Tickets whose subject is the removed machinery are rejected rather than fixed;
each gets a note saying so. TQ-0049 survives, because the guide write itself
can still land on a hand-written file when the task directory is the
repository root.

---

## Notes

- 2026-08-25T16:19:52+02:00 — Removed rootDocNames, docRoot, containsAnyRootDoc, sameFile, withTaskSection, onlyGuidePointers, guidePointer, pointsAtGuide, findSection, headingLevels, scanHeadingLevels, headingLevel, indentColumns, fenceDelimiter and SyncReport. agents.go went from 465 lines to 172.
- 2026-08-25T16:19:52+02:00 — SyncAgentsDocs now takes only the store and returns the paths it wrote, which is the guide or nothing. tq init prints the line to add every run, and GuidePointer resolves it against the repository root so it is right when TQ_DIR nests the queue deeper.
- 2026-08-25T16:19:52+02:00 — JSON output: 'skipped' is gone, since nothing can be skipped, and 'pointer' is added. 'skipped' only ever existed in TQ-0014, committed earlier today and never released, so no agent can depend on it.
- 2026-08-25T16:19:52+02:00 — Tickets closed as rejected: TQ-0042 and TQ-0043, both labelled wontfix. Notes added to TQ-0002, TQ-0014, TQ-0045, TQ-0046 and TQ-0052 recording that their fixes are superseded. TQ-0049 survives and is now the only path by which tq can damage a file it did not write.

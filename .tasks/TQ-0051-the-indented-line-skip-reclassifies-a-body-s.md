---
id: TQ-0051
title: The indented-line skip reclassifies a body's Notes section in both directions
status: done
priority: urgent
labels:
  - bug
  - component/store
created: 2026-08-25T14:46:51+02:00
updated: 2026-08-25T15:54:57+02:00
---

## Finding

Regression introduced by TQ-0048 (commit 206b6c1). To stop a heading *inside*
a note from ending the notes section, `scanNotesStart` gained an unconditional
`indented(line)` skip, in Go (`task.go`) and in `frontend/notes.ts`. The skip
is too broad: an indented ATX heading is valid CommonMark at 1-3 spaces, and it
now neither closes nor opens a notes section.

## How it fails

Both directions reproduced against the pre- and post-diff binaries.

**Document content swallowed into notes.** Body:

    # Task

    ## Notes

    We keep decisions here, in prose.

     ## Acceptance

    - ships

`tq note` before 206b6c1 correctly treated all of it as content — a `## Notes`
followed by another section is ordinary prose, which is the rule TQ-0020 and
TQ-0031 established — and appended a fresh notes section. After 206b6c1 the
`## Acceptance` section is swallowed into the notes blob and a `---` rule that
was never in the file is inserted after `# Task`.

**A second notes section opened.** Body ending in an indented `  ## Notes`
holding one note: before, the note was appended in place. After, the file comes
back with two `---` rules and two `## Notes` headings, the original note
demoted to body prose. Every later `tq note` writes only into the second
section, and the board shows the first one in the Description field.

## Suggested fix

Distinguish the two cases the single predicate now conflates: a heading
indented enough to be a note's continuation line (4+ columns, or indented
relative to its bullet) versus a heading indented 1-3 spaces, which CommonMark
still treats as a heading and which must keep closing and opening sections as
before. Both parsers must move together — `frontend/notes.ts` says in its own
header comment that the two must agree.

Verified safe for the tasks currently in this repository: parsing every
existing `.tasks/*.md` with the pre- and post-diff binaries produces identical
bodies. The regression bites shapes not currently present.

Found by `/code-review` over 004aa72~1..HEAD; both directions reproduced.

---

## Notes

- 2026-08-25T15:54:56+02:00 — Fixed forward. The unconditional indented() skip is replaced by a list-item test: an indented heading is a note's own text only when it continues a list item, and otherwise it is a heading like any other, which restores CommonMark's 1-3 space allowance.
- 2026-08-25T15:54:57+02:00 — A blank line does not end a list item — a multi-line note has one between its paragraphs — so only an unindented line with content resets the state. Getting that wrong broke the case TQ-0048 added, which caught it.
- 2026-08-25T15:54:57+02:00 — Both parsers moved together, as notes.ts's header comment requires, and the TS change is mutation-checked: reverting the predicate alone fails the two new bun tests. Verified end to end that both regression cases now match the pre-regression binary byte for byte, and that all 55 existing task files parse identically to the pre-batch build.

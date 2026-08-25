---
id: TQ-0048
title: tq note collapses newlines, so a note can only ever be one line
status: done
priority: normal
labels:
  - component/cli
created: 2026-08-25T13:56:09+02:00
updated: 2026-08-25T14:47:40+02:00
---

## Finding

`AppendNote` (`task.go`) builds each bullet with
`strings.Join(strings.Fields(text), " ")`, which collapses every run of
whitespace — newlines included — into single spaces.

## How it fails

Verified:

    tq note TQ-0044 -- "Line one.

    - bullet a
    - bullet b"

stores

    - 2026-08-25T13:53:59+02:00 — Line one. - bullet a - bullet b

Structure a caller supplied is silently discarded. Long notes also become very
long single lines, which read poorly in `tq show`, in Git diffs and on the
board.

Nothing documents the constraint, so a caller discovers it only by inspecting
the result.

## Suggested fix

Either keep the flattening and say so in the guide, or preserve newlines by
indenting continuation lines under the bullet so the notes section stays valid
Markdown and a note can hold more than one point.

TQ-0041 now documents the constraint in the guide's step 4 ("One note per
point; each becomes a single line") as a stopgap. Found by `/code-review`.

---

## Notes

- 2026-08-25T14:16:25+02:00 — Chose (b): AppendNote now keeps the note's line breaks, indenting continuation lines two spaces under the bullet. Go side done — including a fix to scanNotesStart so an indented heading inside a note no longer cuts the notes section short. Frontend parity next.
- 2026-08-25T14:17:53+02:00 — Verified end to end with a multi-line note:

  - the guide in agents.go now documents multi-line notes, and .tasks/AGENTS.md was regenerated from it
  - frontend/notes.ts got the same unindented-heading rule so the board and the CLI still agree

  make test, make lint, make frontend and make build all pass.
- 2026-08-25T14:18:09+02:00 — Third note lands under the multi-line one, in the same section — the round trip holds.
- 2026-08-25T14:47:40+02:00 — Regression: the unconditional indented() skip in scanNotesStart reclassifies a body's Notes section in both directions — content swallowed into notes, or a second Notes section opened. Filed as TQ-0051 (urgent). Existing tasks in this repo are unaffected, verified by diffing every file's parsed body across both binaries.

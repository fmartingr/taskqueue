---
id: TQ-0048
title: tq note collapses newlines, so a note can only ever be one line
status: todo
priority: normal
labels:
  - component/cli
created: 2026-08-25T13:56:09+02:00
updated: 2026-08-25T13:56:09+02:00
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

---
id: TQ-0052
title: An unbalanced or tab-indented fence makes tq init duplicate the Task management section forever
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T14:47:08+02:00
updated: 2026-08-25T14:47:08+02:00
---

## Finding

Regression introduced by TQ-0045 (commit 004aa72). `headingLevels` has no
fallback for an unbalanced fence — `notesStart` in `task.go` has exactly one —
so a single stray fence marks every later line `fencedLine`, `findSection`
returns -1, and `withTaskSection` appends a fresh section on every run.

`fenceDelimiter` compounds it: it does `strings.TrimLeft(line, " ")`, spaces
only, so a tab-indented opening fence is invisible while its unindented closing
fence reads as an opener, inverting fence state for the rest of the document.

## How it fails

Both reproduced end to end with the HEAD binary. An `AGENTS.md` containing an
unclosed ```` ```sh ```` fence above its `## Task management` section grows a
duplicate section on every `tq init`: 2 sections, then 3, then 4, each reported
as a successful write. The tab-indented-fence variant does the same.

Before this diff the whole-document guard short-circuited and left the file
alone, so this is strictly a new failure mode — and it is unbounded growth of a
committed file.

The same blindness hits the heading-level choice: `slices.Contains(levels, 1)`
cannot see an H1 hidden behind a stray fence, so tq appends a second level-one
heading to a document that already has one — exactly what the comment above
that line says it prevents.

## Suggested fix

Give `headingLevels` the same unbalanced-fence fallback `notesStart` already
has: if a fence never closes, re-scan treating it as literal text. Accept tabs
as well as spaces in `fenceDelimiter`'s indent handling.

Found by `/code-review` over 004aa72~1..HEAD.

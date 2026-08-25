---
id: TQ-0052
title: An unbalanced or tab-indented fence makes tq init duplicate the Task management section forever
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T14:47:08+02:00
updated: 2026-08-25T16:00:22+02:00
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

---

## Notes

- 2026-08-25T16:00:21+02:00 — Fixed. headingLevels now mirrors notesStart: scanHeadingLevels reports whether the fences balanced, and an unbalanced document is rescanned with fences read as ordinary lines, so a stray fence can no longer hide a real Task management section.
- 2026-08-25T16:00:22+02:00 — Deviation from the suggested fix, deliberate: the ticket said to accept a tab-indented fence as a fence, but CommonMark expands a tab to the next four-column stop, so one leading tab already puts the line four columns in and it is an indented code block, not a fence. Added indentColumns to measure that properly instead. The tab case is fixed by the fallback — the plain closing fence below it is what opens an unbalanced fence.
- 2026-08-25T16:00:22+02:00 — Verified end to end against both reproductions: three tq init runs on a document with an unclosed fence produced 4 Task management sections before and 1 now, with the Setup section and the fence itself preserved. The hidden-H1 case is covered too, so tq no longer appends a second level-one heading.

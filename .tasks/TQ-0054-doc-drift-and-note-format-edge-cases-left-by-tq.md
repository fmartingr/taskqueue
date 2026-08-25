---
id: TQ-0054
title: Doc drift and note-format edge cases left by TQ-0045 through TQ-0048
status: todo
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-25T14:47:28+02:00
updated: 2026-08-25T18:50:04+02:00
---

## Finding

A cluster of smaller confirmed items from the `/code-review` pass over
004aa72~1..HEAD, none individually worth a ticket, all left by the same batch
of changes.

**Documentation drift.** `usageText` in `cli.go` still describes `init` as
"Create the .tasks directory and refresh the agent instructions", and the
README still says tq "creates it at the repository root when it does not exist
yet". After TQ-0047 init discovers first and frequently creates nothing, at a
path the user never named, and the help text gives no hint that it walks up.

**Two contracts for notes.** The README still documents notes in the old
one-line-per-note form, while the regenerated `.tasks/AGENTS.md` now promises a
note keeps its line breaks. Agents and humans read contradicting specs for the
same file, and neither mentions that the board cannot author the multi-line
form at all: `frontend/index.html` makes the Add-note control an
`<input type="text">`.

**Normalisation divergence.** Go and TS now apply different normalisation to
the same note, so a task file's canonical form depends on which surface last
wrote it. `noteBullet("para one\n\n\n\npara two")` collapses the blank run and
strips trailing whitespace; the board's `joinBody` preserves both verbatim.
`go test` and `bun test` share no fixture, so neither suite pins this, and the
result is whitespace churn in `git diff` on a file meant to be committed.

**Note text edge cases in `noteLines`.** A lone CR is embedded in the bullet
rather than treated as a line ending (`TrimSuffix(line, "\r")` also runs before
`TrimRight(line, " \t")`, so a CR followed by trailing spaces survives).
Indentation is stripped from the first line only, so a uniformly indented
pasted block lands as a paragraph on the bullet with the remaining lines two
columns deeper, rendering as an indented code block — and a pasted command is
exactly what the regenerated guide now advertises.

**`headingLevel` strictness.** It counts hashes on the raw line, dropping the
1-3 space indent allowance the old `TrimSpace`-based `findSection` had, and now
treats a bare `#` or `###` as a heading where the old code saw neither.

## Suggested fix

Take them in that order; the first two are documentation, the rest are small
and local. Splitting on `\r\n|\r|\n` and using `TrimRight(line, " \t\r")` fixes
both halves of the CR item; stripping the common leading indent across all
lines fixes the paste item.

Two further items were reported as PLAUSIBLE and are not verified here: the
`HEADING_PATTERN` regex in `frontend/notes.ts` matches a tab where Go's
`isATXHeading` requires a literal space, and `AppendNote`'s new empty-note
no-op is a silent success that its two callers happen to make unreachable.

---

## Notes

- 2026-08-25T16:19:27+02:00 — Partly overtaken by TQ-0055. The headingLevel strictness item is moot, since headingLevel is deleted. The tq init help text and README are corrected as part of that change. The notes items stand: README still documents the old one-line-per-note form, Go and TS still normalise differently, and noteLines still mishandles a lone CR and a uniformly indented paste.
- 2026-08-25T18:42:55+02:00 — The 'help gives no hint that it walks up' item changes with TQ-0029: the walk looks for .taskqueue.yaml and stops at the first one.
- 2026-08-25T18:50:04+02:00 — The walk-up documentation item here should follow TQ-0029, but the note-format items are independent, so this ticket is not blocked on it.

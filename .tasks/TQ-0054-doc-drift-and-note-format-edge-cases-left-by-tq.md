---
id: TQ-0054
title: Doc drift and note-format edge cases left by TQ-0045 through TQ-0048
status: done
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-25T14:47:28+02:00
updated: 2026-08-28T13:31:52+02:00
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
- 2026-08-28T13:17:59+02:00 — Four of the five clusters were already closed by later work, verified on HEAD:

  - Documentation drift (usageText 'Create the .tasks directory', README 'creates it at the repository root'): gone with TQ-0085, which rewrote tq init and both docs. Neither phrase exists.
  - Two contracts for notes: the README no longer carries the old one-line-per-note spec, and the generated .tasks/AGENTS.md documents the multi-line form. What was left of this item was the board half — see below.
  - headingLevel strictness: moot, the function is gone (TQ-0055).
  - The lone-CR half of the noteLines item: the ordering is already TrimRight(TrimSuffix(line, "\\r"), " \\t"), so a CR followed by trailing spaces no longer survives. A lone CR in mid-text is still not treated as a line ending; left alone as out of scope here.

  Three items done:

  1. The add-note control is a textarea, like the editor above it. Enter appends and Shift+Enter is a newline, the same rule the editor uses; a textarea never submits a form on Enter, so TQ-0019 holds either way and its browser test is unchanged.

  2. noteLines strips the indent every line shares rather than the first line's alone, so a pasted block loses the margin it was copied with and keeps its own structure. Blank lines are skipped in the common-prefix calculation, and the first line is still stripped outright since it sits after the timestamp. Three callers pre-trimmed the text and would have eaten the shared indent before noteLines ever saw it — tq note, POST /api/tasks/{id}/notes, and the dialog's own append and note-edit commit. All four now trim only to ask whether there is a note at all.

  3. Measured the Go/TS divergence rather than trusting the ticket, and it was still real: for "para one\\n\\n\\n\\n\\npara two" Go collapsed the blank run and TS kept all four, and for a line ending in spaces or a tab Go trimmed it and TS did not. The other cases agreed. Changed the TS side: formatNote now goes through a noteLines that mirrors task.go, because internal/task is what every note is written through and the board only re-renders on a save. Both suites now read one fixture, internal/task/testdata/notes.json — thirteen cases, asserted against noteBullet and AppendNote on the Go side and against formatNote and joinBody on the TS side, so the file a note lands in is byte-identical whichever surface wrote it.

  Checks: make typecheck, frontend, test-frontend, test, test-integration, lint, build all pass. make test-browser has two pre-existing failures in browser/columns.test.ts, reproduced on an untouched copy of HEAD; the cascade after them is TQ-0092.

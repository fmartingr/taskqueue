---
id: TQ-0045
title: withTaskSection scans Markdown by substring, so a code fence corrupts the doc
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:55:55+02:00
updated: 2026-08-25T16:19:26+02:00
---

## Finding

`findSection` and `withTaskSection` (`agents.go`) treat Markdown as flat lines.
Two defects follow, both silent:

- `findSection`'s inner scan calls `strings.TrimSpace` on every line and counts
  leading `#`, so a shell comment inside a fenced block reads as a heading and
  ends the section early. `withTaskSection` then splices at that point and
  deletes the fence opener.
- The "already points at the guide" guard is
  `strings.Contains(doc, "("+link+")")`, which matches anywhere in the
  document, including inside a fenced example.

## How it fails

Fence corruption, reproduced:

    ## Task management

    See [AGENTS.md](old/path.md)

    ```bash
    # how to run
    tq list
    ```

    ## Other

After `tq init` the ```` ```bash ```` opener is gone, `# how to run` and
`tq list` are body text, and the closing fence dangles orphaned. Exit 0, no
diagnostic. A shell fence whose first line is a `#` comment is about the
commonest construct in an agent instructions file.

False positive, reproduced: a document whose only occurrence of
`(.tasks/AGENTS.md)` sits inside a ```` ```md ```` example gets no
`## Task management` section at all. `tq init` reports only the guide write and
exits 0, so the project silently never gets a working pointer. `README.md`
contains exactly such a snippet, so a project that copies it to explain the
convention opts itself out of the pointer permanently.

`TestSyncAgentsDocsRewritesAStaleSection` only exercises plain prose, so
nothing catches either.

## Suggested fix

Track fence state (``` and ~~~ toggles) while scanning, and require a heading
to start at column 0 outside a fence. Resolve the guard by looking for a
pointer inside the actual `Task management` section rather than substring
matching the whole document — that fixes the false positive and the
`@`-include miss in TQ-0042 together.

Found by `/code-review` on TQ-0041; both cases reproduced by hand.

---

## Notes

- 2026-08-25T14:02:47+02:00 — Added headingLevels(): one pass over the document classifying each line as an ATX heading level, 0 for prose, or -1 (fencedLine) inside a fenced block, delimiters included.
- 2026-08-25T14:02:48+02:00 — fenceDelimiter() handles both backtick and tilde fences, closing fences at least as long as their opener with no info string, and skips four-space-indented lines, which are code blocks rather than fences.
- 2026-08-25T14:02:48+02:00 — headingLevel() now requires column 0 and hashes followed by a space or end of line, so an indented line is no longer read as a heading.
- 2026-08-25T14:02:48+02:00 — findSection() takes the precomputed levels, so a hash comment inside a shell fence no longer ends the section early and the splice no longer eats the fence opener.
- 2026-08-25T14:02:48+02:00 — Replaced the whole-document strings.Contains guard with pointsAtGuide(), which looks only inside the Task management section and skips fenced lines, so a README-style example no longer suppresses the pointer.
- 2026-08-25T14:02:48+02:00 — pointsAtGuide() also accepts an at-sign include of the link, so a Claude-style include counts as a pointer and is left alone. That is TQ-0042 symptom, which this guard rework resolves; TQ-0042 itself left open for its owner.
- 2026-08-25T14:02:48+02:00 — The heading level chosen for an appended section now comes from the same levels slice, so a hash comment in a fence no longer demotes a new section to level two.
- 2026-08-25T14:02:48+02:00 — Six new tests in agents_test.go, each confirmed failing first: fenced block past the section end, a later fence untouched, a fenced heading ignored, a fenced pointer ignored, an include kept, a pointer outside the section ignored, and the fenced-hash heading level.
- 2026-08-25T14:02:48+02:00 — taskGuide is untouched and the generated .tasks/AGENTS.md is byte-identical. make test, make lint (0 issues) and make build all pass.
- 2026-08-25T14:47:40+02:00 — Regression: headingLevels has no unbalanced-fence fallback and fenceDelimiter ignores tab indentation, so a stray fence makes tq init append a duplicate Task management section on every run. Filed as TQ-0052. Narrowing the pointer guard to the section also widened TQ-0014; noted there.
- 2026-08-25T16:19:26+02:00 — Superseded by TQ-0055: the fence-aware scanner and the section-scoped pointer guard existed only to edit the repository's own agent instructions, which tq no longer does. headingLevels, findSection, fenceDelimiter and pointsAtGuide are deleted.

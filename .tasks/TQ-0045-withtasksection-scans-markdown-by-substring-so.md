---
id: TQ-0045
title: withTaskSection scans Markdown by substring, so a code fence corrupts the doc
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:55:55+02:00
updated: 2026-08-25T13:55:55+02:00
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

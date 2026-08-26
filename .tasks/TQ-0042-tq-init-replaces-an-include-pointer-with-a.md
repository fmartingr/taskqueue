---
id: TQ-0042
title: tq init replaces an @-include pointer with a plain link, dropping the guide from agent context
status: rejected
priority: high
labels:
  - bug
  - component/cli
  - wontfix
created: 2026-08-25T13:31:50+02:00
updated: 2026-08-26T16:29:20+02:00
---

## Finding

`withTaskSection` (`agents.go`) decides a document already points at the guide
with a single check:

    if strings.Contains(doc, "("+link+")") {

That only recognises Markdown link syntax. This repository's nomenclature for
pointing at the guide is an `@.tasks/AGENTS.md` include — the syntax that
actually pulls the document into an agent's context, and the reason the root
`AGENTS.md` can stay minimal while the workflow lives in the guide. It does not
match, so `tq init` rewrites the section into `See [AGENTS.md](.tasks/AGENTS.md)`,
a plain link that loads nothing.

## How it fails

Reproduced in this repository, on a clean tree, while working TQ-0041:

    $ tq init
    Task queue already initialized in /Users/fmartingr/Code/task-queue/.tasks
    Wrote .../.tasks/AGENTS.md
    Wrote .../AGENTS.md
    Wrote .../CLAUDE.md

Two separate regressions, both reported as success with exit 0:

- `AGENTS.md`'s `## Task management` section went from `@.tasks/AGENTS.md` to
  `See [AGENTS.md](.tasks/AGENTS.md)`. The guide stops being included; agents
  silently lose the entire task workflow.
- `CLAUDE.md` was one line, `@AGENTS.md`, which already reaches the guide
  transitively. It has no `## Task management` heading, so a redundant second
  one was appended.

`tq init` is documented as safe to re-run, and the store-created path is not
required for either rewrite, so any re-run on a correctly configured repository
degrades it.

## Suggested fix

Treat an `@<link>` include as an existing pointer alongside `[...](<link>)`,
and prefer it when writing a pointer from scratch: the include is what this
repository uses, and what makes the guide the single home for the workflow. Do
not append a section to a document that reaches the guide through an include of
another root doc. Renders of an already-correct file should be
no-ops, which is what `writeIfChanged` and the "rewrites nothing that is
already right" contract in `SyncAgentsDocs`'s own doc comment promise.

Found while working TQ-0041, which had to `git checkout` both files after
regenerating the guide.

---

## Notes

- 2026-08-25T13:40:08+02:00 — Related: TQ-0043 covers a second way tq init damages committed repo docs — docRoot() ignores TQ_DIR, so the pointer is written into the enclosing Git repo even when the task dir is elsewhere. Different root cause, separate fix.
- 2026-08-25T14:04:04+02:00 — Half resolved by TQ-0045 (commit 004aa72): pointsAtGuide now recognises an @<link> include, so a root AGENTS.md carrying @.tasks/AGENTS.md is left alone. Verified in a throwaway repo — AGENTS.md untouched and absent from the written list.
- 2026-08-25T14:04:04+02:00 — Still open: a CLAUDE.md that is just @AGENTS.md, and so reaches the guide transitively, still gets a redundant '# Task management' section appended. Same repro, same run. That is the remaining half of this ticket.
- 2026-08-25T16:19:12+02:00 — Rejected by TQ-0055: tq no longer edits the repository's AGENTS.md or CLAUDE.md at all, so there is no pointer for it to replace and no redundant section for it to append. withTaskSection and pointsAtGuide are deleted.
- 2026-08-25T16:19:12+02:00 — The @-include this ticket defended is now the documented form: tq init prints '@.tasks/AGENTS.md' for the user to add themselves, and the README says the same.
- 2026-08-26T16:29:20+02:00 — Moved from done to rejected. This was closed as wontfix — the work was declined, not completed — and until TQ-0035 the board had only a done column to close it into. Rejected is now that column, and deliberately does not satisfy dependencies: a task waiting on work nobody will do is still blocked, which done would have quietly said otherwise about.

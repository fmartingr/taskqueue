---
id: TQ-0056
title: tq init writes its guide into an unrelated project's task directory
status: done
priority: urgent
labels:
  - bug
  - component/cli
created: 2026-08-25T17:43:03+02:00
updated: 2026-08-25T17:45:33+02:00
---

## Finding

`runInit` hands `SyncAgentsDocs` whatever store discovery returned. In a
project with no repository root the walk has no bound, so the store can be an
unrelated ancestor — and `tq init` then writes a guide into a task directory
belonging to a different project.

## How it fails

Reproduced with the built binary. A task directory sits in the home folder, and
the project below it has no `.git`:

    $ cd home/projects/foo && tq init
    Task queue already initialized in SP/h1/.tasks
    Wrote SP/h1/.tasks/AGENTS.md

    foo/.tasks created?               NO — adopted the home queue
    guide written into the home queue? YES — wrote outside the project

`.tasks` is meant to be committed, so this dirties another repository's working
tree. The only record is a `Wrote` line naming a path outside the project.
`writeIfChanged` compares against the exact generated text, so a guide from an
older tq, or one a person annotated, is replaced without confirmation.

No task content is lost: the file is generated and recoverable from Git.

## Suggested fix

Option C of three considered, chosen deliberately over changing discovery.

`tq init` writes the guide only into a task directory inside the tree it was
invoked in — the repository root when there is one, otherwise the working
directory. Anywhere else it skips the write and says so.

This removes the write outside the project without touching any discovery
rule. The missing bound for projects without a repository root is a separate
question, deliberately left open: see the follow-up ticket for recognising
other project markers.

---

## Notes

- 2026-08-25T17:45:33+02:00 — Implemented as option C, chosen over changing discovery. withinInvokedTree treats the enclosing repository as the boundary, or the working directory when there is none, and the guide is written only inside it.
- 2026-08-25T17:45:33+02:00 — Verified: init in a project below a stray home queue now leaves that queue's guide alone and says so on stderr, while init from a deep subdirectory of a repository still writes the guide as before.
- 2026-08-25T17:45:33+02:00 — The adoption itself is untouched and is filed as TQ-0057, which carries option B. This ticket removes only the write outside the project.

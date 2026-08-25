---
id: TQ-0046
title: tq init overwrites the root doc when the task dir is the repo root, and never converges
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:55:55+02:00
updated: 2026-08-25T13:55:55+02:00
---

## Finding

`SyncAgentsDocs` writes the guide to `<store.Dir>/AGENTS.md` and then loops
over `rootDocNames` in the doc root. It never checks whether the two are the
same file.

## How it fails

Reproduced in a scratch Git repository holding a hand-written `AGENTS.md`, with
the task directory at the repository root:

    $ TQ_DIR=<repo root> tq init
    Task queue already initialized in <repo root>
    Wrote <repo root>/AGENTS.md
    Wrote <repo root>/AGENTS.md

The hand-written file is gone, replaced wholesale by the generated task guide
plus a self-referential `## Task management` / `See [AGENTS.md](AGENTS.md)`.
The path appears twice in `written`, so the CLI prints the same line twice.

Run 2 prints the same two writes again, forever: `writeIfChanged(guide, ...)`
reverts the appended section, then the root-doc loop re-appends it. `tq init`
is never idempotent, contradicting the "rewrites nothing that is already right"
contract in `SyncAgentsDocs`'s own doc comment and the README.

## Suggested fix

Compare the resolved guide path against each root-doc path before the loop and
skip the ones that are the same file.

Distinct from TQ-0043 (`docRoot` ignoring `TQ_DIR`) and survives fixing it.
Found by `/code-review` on TQ-0041; reproduced by hand.

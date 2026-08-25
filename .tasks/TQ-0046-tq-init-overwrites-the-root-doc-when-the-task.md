---
id: TQ-0046
title: tq init overwrites the root doc when the task dir is the repo root, and never converges
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:55:55+02:00
updated: 2026-08-25T14:08:39+02:00
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

---

## Notes

- 2026-08-25T14:06:47+02:00 — Fixed by a path-identity check: SyncAgentsDocs now skips any root doc that is the same file as the guide it just wrote.
- 2026-08-25T14:06:47+02:00 — The check is os.SameFile, not a string compare, because TQ_DIR and the discovered doc root can name one file by two paths (symlink, case-insensitive FS); it falls back to comparing absolute paths only when neither file exists.
- 2026-08-25T14:06:47+02:00 — Reproduced the ticket in a throwaway git repo with a binary built outside this repo: run 1 now writes the guide and CLAUDE.md once each, runs 2 and 3 write nothing, and CLAUDE.md keeps its hand-written content plus a See [AGENTS.md](AGENTS.md) pointer.
- 2026-08-25T14:08:39+02:00 — Verified after cdd9aa4: convergence is fixed — run 1 writes once, runs 2 and 3 write nothing, and the self-referential section and duplicate report line are gone.
- 2026-08-25T14:08:39+02:00 — The other half of this ticket's title is not fixed: a hand-written root AGENTS.md is still destroyed by the guide write that precedes the loop. Split out as TQ-0049 rather than reopening, since the implementation matched this ticket's stated Suggested fix.

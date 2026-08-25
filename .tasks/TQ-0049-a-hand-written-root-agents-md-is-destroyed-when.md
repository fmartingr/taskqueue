---
id: TQ-0049
title: A hand-written root AGENTS.md is destroyed when the task dir is the repo root
status: done
priority: normal
labels:
  - bug
  - component/cli
  - wontfix
created: 2026-08-25T14:08:39+02:00
updated: 2026-08-25T16:26:46+02:00
---

## Finding

When the task directory resolves to the repository root, the guide's path and
the root document's path are the same file. `SyncAgentsDocs` writes the guide
first, unconditionally:

    changed, err := writeIfChanged(guide, taskGuide(store.Dir))

TQ-0046 added a `sameFile` skip to the root-doc loop that follows, which fixed
the self-referential section, the duplicated `written` entry and the infinite
churn. It did not — and by its own Suggested fix could not — protect the file
from that first write.

## How it fails

Reproduced after TQ-0046 (commit cdd9aa4), in a scratch Git repository holding
a hand-written `AGENTS.md`:

    $ TQ_DIR=<repo root> tq init
    Task queue already initialized in <repo root>
    Wrote <repo root>/AGENTS.md

    $ grep -c "Hand written" AGENTS.md
    0

The file now opens with the generated notice. `tq init` converges cleanly from
here — runs 2 and 3 write nothing — but the original content is gone, with no
prompt and no diagnostic, from a command that reports success.

## Suggested fix

Refuse to render the guide over a path that is also a root document, or
require the task directory to be a subdirectory of the doc root. Overwriting a
file the generator does not own is the part that needs preventing; the notice
at the top of the guide only warns people who already lost the file.

Likely entangled with TQ-0047: `runInit` calling `InitStore(c.dir)` rather
than discovering an existing queue is what puts a task directory at an
unexpected root in the first place.

Split out of TQ-0046, whose title covered this case but whose Suggested fix
addressed only the convergence half. Found by verifying that ticket's fix.

---

## Notes

- 2026-08-25T16:19:27+02:00 — Survives TQ-0055 and is now the only remaining way tq can damage a hand-written file. Re-verified after the removal: with TQ_DIR making the task directory the repository root, the guide write still lands on the root AGENTS.md and replaces it. The surrounding machinery is gone, so the fix is now small — refuse to render the guide over a path that already holds a file tq did not write.
- 2026-08-25T16:26:46+02:00 — Rejected as a scope decision, not as unreproducible: it still reproduces on the current build, verified after TQ-0055.
- 2026-08-25T16:26:46+02:00 — Reaching it requires pointing TQ_DIR at the repository root, which makes the root AGENTS.md the guide's own path by definition. That is a deliberate, unusual configuration, and the file tq overwrites is the one the user told it to write. Guarding it would mean tq inspecting a document again, which is exactly what TQ-0055 removed.
- 2026-08-25T16:26:46+02:00 — Reopen if a real project hits this without setting TQ_DIR by hand.

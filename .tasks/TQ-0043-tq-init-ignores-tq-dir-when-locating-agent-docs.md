---
id: TQ-0043
title: tq init ignores TQ_DIR when locating agent docs, rewriting an unrelated repo's AGENTS.md
status: rejected
priority: high
labels:
  - bug
  - component/cli
  - wontfix
created: 2026-08-25T13:44:56+02:00
updated: 2026-08-26T16:29:20+02:00
---

## Finding

`SyncAgentsDocs` takes the store (which honours `TQ_DIR`) but locates the
documents to update with `docRoot(workingDir)`, which consults only the
enclosing Git repository:

    func docRoot(workingDir string) string {
        ...
        if root, ok := repositoryRoot(dir); ok {
            return root
        }

So the guide is written to the `TQ_DIR` task directory while the pointer to it
is written into whatever repository the command happened to run in.

## How it fails

Reproduced in a scratch repository (`proj/` is a Git repo with an
`@.tasks/AGENTS.md` include; the task directory is outside it):

    $ TQ_DIR=../elsewhere/.tasks tq init
    Initialized task queue in .../elsewhere/.tasks
    Wrote .../elsewhere/.tasks/AGENTS.md
    Wrote .../proj/AGENTS.md

`proj/AGENTS.md` afterwards:

    ## Task management

    See [AGENTS.md](../elsewhere/.tasks/AGENTS.md)

The repository's committed instructions now point outside the repository, at a
directory that may be a temp dir, and the `@` include that actually loaded the
guide is gone. `proj/` never gained a `.tasks/` at all. Exit 0, reported as
success.

Observed for real in this repository: an agent ran `tq init` with `TQ_DIR` set
to a scratchpad, and the committed `AGENTS.md` and `CLAUDE.md` were rewritten
to point at `../../../../private/tmp/.../scratchpad/gen/.tasks/AGENTS.md`.
Restored by hand.

## Suggested fix

Derive the doc root from the store, not from the working directory: when
`TQ_DIR` places the task directory outside the enclosing repository, the
repository's own instructions are not the right thing to edit. Either resolve
the root relative to `store.Dir`, or skip the root-doc update entirely when the
task directory is not inside the doc root.

The pointer this repository uses is an `@.tasks/AGENTS.md` include, and that
nomenclature must survive the fix: the include is what puts the guide in agent
context, which is why the root document can stay minimal.

Distinct from TQ-0042 (which is about `@` includes not being recognised as
existing pointers); the two share a symptom but need separate fixes, and this
one fires even when the pointer syntax is a plain Markdown link.

---

## Notes

- 2026-08-25T14:12:27+02:00 — Adjacent case found while verifying TQ-0047 (cad90f9): SyncAgentsDocs derives its doc root from the working directory, not from the discovered task directory, so init in proj/backend of a non-Git project still creates a pointer at backend/AGENTS.md. It is now a correct pointer (../.tasks/AGENTS.md) rather than one aimed at a forked queue, so the harm is reduced, but the root cause is the same docRoot() question this ticket covers.
- 2026-08-25T16:19:12+02:00 — Rejected by TQ-0055: docRoot is deleted along with the rest of the root-document handling. tq init writes only inside the task directory, so it can no longer write a pointer into whatever repository the shell happens to be in, with TQ_DIR set or otherwise.
- 2026-08-25T16:19:12+02:00 — The doc-root residue noted here while verifying TQ-0047 — a stray pointer AGENTS.md left in a subdirectory — goes away with the same change.
- 2026-08-26T16:29:20+02:00 — Moved from done to rejected. This was closed as wontfix — the work was declined, not completed — and until TQ-0035 the board had only a done column to close it into. Rejected is now that column, and deliberately does not satisfy dependencies: a task waiting on work nobody will do is still blocked, which done would have quietly said otherwise about.

---
id: TQ-0043
title: tq init ignores TQ_DIR when locating agent docs, rewriting an unrelated repo's AGENTS.md
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:44:56+02:00
updated: 2026-08-25T13:44:56+02:00
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

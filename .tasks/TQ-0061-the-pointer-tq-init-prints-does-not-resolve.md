---
id: TQ-0061
title: The pointer tq init prints does not resolve from where it was run
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T17:44:51+02:00
updated: 2026-08-25T17:44:51+02:00
---

## Finding

`GuidePointer` builds the pointer relative to the guide's own location: the
repository root when there is one, otherwise the parent of the task directory.
It never considers the directory the command ran in.

## How it fails

Verified here, in a project with no repository root and `TQ_DIR` set elsewhere:

    $ cd project && TQ_DIR=../elsewhere/queue tq init
        @queue/AGENTS.md

From `project/` the correct line is `@../elsewhere/queue/AGENTS.md`. The
reviewer verified two more shapes: init from `project/backend/deep` adopting
`project/.tasks` prints `@.tasks/AGENTS.md`, correct from `project/` and
meaningless from `deep`; and inside a repository the pointer is repository-root
relative while the message names no directory at all.

A user follows tq's own instruction, pastes the line, and the agent loads no
guide.

## Suggested fix

Resolve the pointer against the directory the command ran in, or name the file
the line belongs in so the relative path is unambiguous.

Introduced by TQ-0055. The fallback branch has no coverage: the assertion in
`agents_test.go` uses `strings.HasSuffix` and accepts either branch, which is
why the output changed inside TQ-0023 with no test diff.

Found by `/code-review` over 20b06d2.

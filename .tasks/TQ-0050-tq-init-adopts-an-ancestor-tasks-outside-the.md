---
id: TQ-0050
title: tq init adopts an ancestor .tasks outside the repository and creates none
status: done
priority: urgent
labels:
  - bug
  - component/cli
created: 2026-08-25T14:46:51+02:00
updated: 2026-08-25T15:52:02+02:00
---

## Finding

Regression introduced by TQ-0047 (commit cad90f9). `runInit` now resolves its
store with `OpenStore`, but `DiscoverTaskDir` walks *past* the repository root
while `taskDirTarget` stops at it. `tq init` was the only escape hatch from
ancestor discovery, and it no longer is.

## How it fails

Reproduced against both binaries. An ancestor `.tasks` exists (trivially — any
`tq add` in a parent directory makes one), then a fresh Git repository below it:

    outer/.tasks/
    outer/repo/.git
    outer/repo/AGENTS.md

Running `tq init` inside `outer/repo`:

    before cad90f9:  created: True   task_dir: outer/repo/.tasks
                     repo/.tasks created: YES
                     pointer: See [AGENTS.md](.tasks/AGENTS.md)

    after cad90f9:   created: False  task_dir: outer/.tasks
                     repo/.tasks created: NO
                     pointer: See [AGENTS.md](../.tasks/AGENTS.md)

The repository gets no queue of its own, and its committed `AGENTS.md` now
points outside the repository — a dead link for anyone who clones it.

This contradicts the rule in the root `AGENTS.md` and the README: the task
directory is created "at the root of the enclosing Git repository".

## Suggested fix

Bound discovery at the repository root when one exists, so `tq init` inside a
Git repository never adopts a queue from outside it. TQ-0017 covers the same
root cause (discovery walking past the repository root) at priority low; this
diff sharpens it considerably and the two should be fixed together.

Found by `/code-review` over 004aa72~1..HEAD; reproduced by hand.

---

## Notes

- 2026-08-25T15:52:01+02:00 — Resolved by reverting cad90f9 rather than by a forward fix: init no longer discovers, so it cannot adopt an ancestor queue. Verified — a fresh repo below an ancestor .tasks now reports created=true, task_dir repo/.tasks, and a pointer inside the repo.
- 2026-08-25T15:52:01+02:00 — The root cause survives the revert: DiscoverTaskDir still walks past the repository root, which is TQ-0017. That must be fixed before TQ-0047 is attempted again, or the same regression returns.

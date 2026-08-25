---
id: TQ-0047
title: tq init forks the queue in a subdirectory instead of discovering the parent
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:56:09+02:00
updated: 2026-08-25T15:51:39+02:00
---

## Finding

`runInit` resolves its store with `InitStore(c.dir)` (`cli.go`), while every
other command goes through `OpenStore` -> `DiscoverTaskDir`, which walks *up*
for an existing `.tasks`. In a project that is not a Git repository the two
disagree.

## How it fails

Reproduced: `proj/.tasks` holding one task, no `.git`, and a `proj/backend/`
subdirectory.

    $ cd proj/backend && tq list
    TQ-0001  todo  normal  -  existing work      # finds the parent queue

    $ tq init                                    # from the same directory
    $ ls -d .tasks                               # a second queue appears
    $ tq list
    ID  STATUS  PRIORITY  ASSIGNEE  TITLE        # empty

The existing work is now invisible to everything run under `backend/`, and a
second root-doc pointer is written there.

TQ-0041 makes this worse by promoting `tq ready` to mandatory step 1: an agent
that follows the guide from a subdirectory queries the forked empty queue and
correctly concludes there is nothing to claim.

## Suggested fix

Have `runInit` discover an existing task directory the way every other command
does, and only create one where discovery finds nothing.

Found by `/code-review` on TQ-0041; reproduced by hand.

---

## Notes

- 2026-08-25T14:11:30+02:00 — runInit now resolves its store with OpenStore(c.dir) instead of InitStore(c.dir), so init discovers an existing .tasks above the working directory exactly like every other command and only creates one when discovery finds nothing.
- 2026-08-25T14:11:31+02:00 — New CLI test TestCLIInitFindsTheQueueAbove: in a non-Git project with a parent queue holding a task, init from a subdirectory reports the parent task_dir with created=false, makes no second .tasks, and list from the subdirectory still sees the parent task. It failed on all four points before the change.
- 2026-08-25T14:11:31+02:00 — New CLI test TestCLIInitCreatesAtTheRepositoryRoot guards the documented placement that discovery must not cost: with nothing to find, init from a nested directory of a Git repo still creates the queue next to .git, created=true.
- 2026-08-25T14:11:31+02:00 — Verified by hand against a throwaway project outside the repo: before, init in proj/backend forked an empty queue; now it reports proj/.tasks and list still shows the parent task. A second init writes nothing, so init still converges.
- 2026-08-25T14:11:31+02:00 — Not fixed, out of scope: init from a subdirectory of a non-Git project still creates a pointer AGENTS.md in that subdirectory, because SyncAgentsDocs derives its doc root from the working directory, not from the discovered task dir. The pointer is at least correct now -- it links ../.tasks/AGENTS.md instead of a forked queue.
- 2026-08-25T14:11:31+02:00 — TQ-0049 is unaffected: re-checked by hand that a hand-written root AGENTS.md is still overwritten when TQ_DIR makes the task dir the repo root. That path never went through InitStore(c.dir), so this change neither fixes nor worsens it.
- 2026-08-25T14:47:40+02:00 — Regression: this fix overshot. DiscoverTaskDir walks past the repository root while taskDirTarget stops at it, so tq init in a fresh repo now adopts an ancestor .tasks outside it and creates none. Filed as TQ-0050 (urgent). Also made the CLI init tests write into a real queue above TMPDIR — TQ-0053.
- 2026-08-25T15:51:39+02:00 — Reverted. Commit cad90f9 backed out because it traded a subdirectory-fork bug for a worse cross-repo one (TQ-0050): init adopted an ancestor .tasks outside the repository and created none. This ticket is open again — the original forking bug is back.
- 2026-08-25T15:51:39+02:00 — Redo condition: bound discovery at the repository root first (TQ-0017 / TQ-0050 root cause), then make runInit discover. Discovering without that bound is what caused the regression, and it also let the CLI init tests write into a real queue above TMPDIR (TQ-0053).

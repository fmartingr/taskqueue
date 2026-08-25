---
id: TQ-0047
title: tq init forks the queue in a subdirectory instead of discovering the parent
status: done
priority: high
labels:
  - bug
  - component/cli
depends_on:
  - TQ-0017
created: 2026-08-25T13:56:09+02:00
updated: 2026-08-25T16:47:37+02:00
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

## Blocked by TQ-0017, now cleared

This was fixed once, in commit cad90f9, and reverted in e300384. The fix
itself was right — one line, `InitStore` to `OpenStore` — but discovery was
unbounded at the time, so making `init` discover meant it would walk past the
repository root and adopt a queue outside the repository, creating none of its
own and writing a pointer to a path that does not exist for anyone who clones
it. That regression is TQ-0050.

TQ-0017 fixed the real cause: discovery now stops at the repository root, the
way creation always did, with `TQ_WALK_FOREVER=true` to lift the bound. The
same one-line change is safe to make again.

Two things the redo must carry that the first attempt did not:

- Anchor the CLI test fixtures. `newBareCLI` sets neither `TQ_DIR` nor a `.git`
  directory, so with `init` discovering, the fixtures could walk out of
  `t.TempDir()` and write into a developer's real queue above `TMPDIR`. That
  was TQ-0053, resolved only by the revert. TQ-0017's bound does not cover it,
  because an unanchored temp directory has no repository root to stop at.
- Keep `store.Created` honest, so `created` in the JSON reports discovery
  rather than creation.

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
- 2026-08-25T16:44:13+02:00 — Dependency on TQ-0017 recorded, and the body now carries the blocker history: why the first attempt was reverted, what TQ-0017 changed, and the two things the redo must carry that cad90f9 did not.
- 2026-08-25T16:47:36+02:00 — Implemented, second attempt. runInit resolves through OpenStore, the same one-line change as cad90f9, now safe because TQ-0017 bounds discovery at the repository root.
- 2026-08-25T16:47:37+02:00 — Learned while testing: after TQ-0017 this bug only survives in a project that is not a Git repository. Where there is a repository root, taskDirTarget already stops there and init lands in the right place anyway. My first test used a Git project and passed before the fix; the real test uses a non-Git project with the anchor one level above it, and fails without the fix.
- 2026-08-25T16:47:37+02:00 — Fixture anchoring done as required: newBareCLI now marks its temp root as a repository root, so no fixture can walk out of t.TempDir() into a developer's real queue. That closes the hazard TQ-0053 described, which the revert had only masked.
- 2026-08-25T16:47:37+02:00 — TestCLIReportsUncreatableTaskDir needed adjusting: with fixtures anchored, creation falls back to the writable repository root, so nothing was uncreatable any more. It now makes that root read-only, restoring the permissions before t.TempDir cleans up.
- 2026-08-25T16:47:37+02:00 — Verified end to end: init in a subdirectory of a non-Git project no longer forks, and list from there still sees the project's work. The TQ-0050 regression stays dead — a fresh repo below an outside queue reports created=true with its own task_dir.

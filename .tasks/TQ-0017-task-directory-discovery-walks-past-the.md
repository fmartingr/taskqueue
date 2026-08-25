---
id: TQ-0017
title: Task directory discovery walks past the repository root
status: todo
priority: low
labels:
  - review
  - store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

DiscoverTaskDir's walk-up never stops at the repository root (unlike taskDirTarget, which does), treats every os.Stat error and every non-directory .tasks as "keep walking", and walks lexically so symlinked working directories climb the wrong parents.

Source: `store.go:149`

## How it fails

I reproduced the primary case: with a .tasks directory in a parent folder and a fresh `git init` repo underneath it holding no .tasks, `tq add "hijack test"` in the repo wrote TQ-0001 into the PARENT's .tasks and never created one in the repo — contradicting AGENTS.md:57-59 ("The task directory is created on demand ... at the root of the enclosing Git repository"). A developer with a personal ~/.tasks therefore has every new project silently file into it. Two amplifiers: a permission error on the project's own .tasks is swallowed by `err == nil && info.IsDir()` so tq adopts an ancestor queue with err=nil and no diagnostic; and because filepath.Abs does not resolve symlinks, `cd ~/work && tq list` where ~/work -> /repo/services/api walks ~, /home, / and lands on ~/.tasks while the agent believes it is in the project.

## Suggested fix

Decide whether discovery should stop at the repository root the way `taskDirTarget` does. Walking to the filesystem root is what the plan specified, so this may just need documenting.

Filed from a `/code-review` pass at max effort.

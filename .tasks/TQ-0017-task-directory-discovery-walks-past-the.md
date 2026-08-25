---
id: TQ-0017
title: Task directory discovery walks past the repository root
status: done
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T16:37:48+02:00
---

## Finding

DiscoverTaskDir's walk-up never stops at the repository root (unlike taskDirTarget, which does), treats every os.Stat error and every non-directory .tasks as "keep walking", and walks lexically so symlinked working directories climb the wrong parents.

Source: `store.go:149`

## How it fails

I reproduced the primary case: with a .tasks directory in a parent folder and a fresh `git init` repo underneath it holding no .tasks, `tq add "hijack test"` in the repo wrote TQ-0001 into the PARENT's .tasks and never created one in the repo — contradicting AGENTS.md:57-59 ("The task directory is created on demand ... at the root of the enclosing Git repository"). A developer with a personal ~/.tasks therefore has every new project silently file into it. Two amplifiers: a permission error on the project's own .tasks is swallowed by `err == nil && info.IsDir()` so tq adopts an ancestor queue with err=nil and no diagnostic; and because filepath.Abs does not resolve symlinks, `cd ~/work && tq list` where ~/work -> /repo/services/api walks ~, /home, / and lands on ~/.tasks while the agent believes it is in the project.

## Suggested fix

Decided: discovery stops at the repository root, the way `taskDirTarget`
already does. The documented promise — "the task directory is created on demand
at the root of the enclosing Git repository" — becomes true of finding one as
well as making one, and a queue above your project can no longer capture it.

`TQ_WALK_FOREVER=true` restores the old behaviour and walks to the filesystem
root. It is for the layout the original plan had in mind: a queue deliberately
kept above several repositories, shared by all of them. Anything other than
`true` leaves the default in place.

`TQ_DIR` is unaffected and still wins outright; it names a directory rather
than starting a search.

### What the change touches

- `DiscoverTaskDir` (`store.go`): stop the walk once the current directory is
  the repository root, unless `TQ_WALK_FOREVER` is set. `repositoryRoot` is
  already there and is what `taskDirTarget` uses.
- A new `EnvWalkForever = "TQ_WALK_FOREVER"` alongside `EnvTaskDir`.
- The not-found error should say the walk stopped at the repository root, and
  name the variable that lifts the bound — otherwise the failure looks like
  tq simply cannot see a queue that is plainly there.
- README and the generated guide (`taskGuide` in `agents.go`) both document
  `TQ_DIR`; both should mention this too.

### Out of scope

The two amplifiers stay open, and neither is fixed by the bound:

- A permission error on a candidate `.tasks` is swallowed by
  `err == nil && info.IsDir()`, so tq walks past a queue it merely could not
  read, with no diagnostic.
- `filepath.Abs` does not resolve symlinks, so a symlinked working directory
  climbs the wrong parents.

Both survive inside a single repository and deserve their own tickets.

### Unblocks

TQ-0047 was reverted because it made `tq init` discover rather than create,
and unbounded discovery then let init adopt a queue outside the repository
(TQ-0050). With the bound in place that fix becomes safe to redo, and the test
hazards in TQ-0021 and TQ-0023 lose their reach as well.


Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T16:31:54+02:00 — Decision recorded: discovery stops at the repository root by default, with TQ_WALK_FOREVER=true to walk to the filesystem root as before. The open question in the original Suggested fix is settled; the section now describes the change rather than asking for a choice.
- 2026-08-25T16:31:54+02:00 — Reproduced again on the current build before writing it up: a .tasks in a parent directory captured tq add from a fresh git repo underneath, filing TQ-0002 into the parent and never creating a queue in the repo.
- 2026-08-25T16:31:54+02:00 — Raised from low to high: this blocks TQ-0047, which had to be reverted because unbounded discovery let tq init adopt a queue outside the repository (TQ-0050).
- 2026-08-25T16:37:48+02:00 — Implemented. DiscoverTaskDir now stops at the repository root, using the repositoryRoot helper taskDirTarget already used, so finding a queue and creating one finally agree. TQ_WALK_FOREVER=true lifts the bound; any other value leaves it in place.
- 2026-08-25T16:37:48+02:00 — Correction to this ticket's own plan: the improved not-found error is unreachable from the CLI. OpenStore is DiscoverTaskDir's only caller and it converts ErrProjectNotFound into creating a local queue, so the message never prints. It is kept and tested as the function's contract, but it is not what a user sees.
- 2026-08-25T16:37:48+02:00 — Where the confusion actually lands is the create path: a developer with tasks above the repository suddenly gets an empty local queue. tq now prints a second stderr note there, naming the excluded directory and TQ_WALK_FOREVER. That is this ticket's intent delivered where it is visible.
- 2026-08-25T16:37:48+02:00 — Out of scope and still open, as recorded before: the swallowed permission error on a candidate .tasks, and unresolved symlinks in the walk. Both survive inside a single repository and are untouched by the bound.

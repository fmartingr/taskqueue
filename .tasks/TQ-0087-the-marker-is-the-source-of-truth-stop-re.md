---
id: TQ-0087
title: 'The marker is the source of truth: stop re-deriving config from the task directory'
status: done
priority: urgent
labels:
  - bug
  - component/store
  - component/config
created: 2026-08-27T19:14:58+02:00
updated: 2026-08-27T20:43:13+02:00
---

## The rule

**The marker file is the source of truth.** A command walks up from the working
directory until it finds `.taskqueue.yaml`. Once it has that file it knows
everything: the board, the priorities, the labels, and where the task files
live. Nothing may ever walk *from* the task directory back to the config — the
task directory is an output of the marker, not a way to find it.

Today the code does the opposite in five places, and the queue silently loses
its configuration whenever `path:` points outside the marker's own directory.

## Reproduced on HEAD c41ae4c

`tq init` **generates the broken shape itself** — no hand-edited config needed:

    $ cd proj && TQ_DIR=../queue tq init
    $ cat proj/.taskqueue.yaml
    version: 1
    path: ../queue          <- written by tq init, outside the marker's directory
    columns: ... priorities: ... labels: ...

With a project declaring its own columns (`backlog`, `doing`, `shipped`):

    status on disk:                      doing
    $ tq move TQ-0001 doing
    error: invalid status "doing" (want one of inbox, todo, in-progress, done, rejected)
                                         <- the project's own column, refused

    $ tq update TQ-0001 --assignee alice <- nothing to do with status
    Updated TQ-0001: Real work
    status on disk now:                  inbox    <- silently rewritten, exit 0

Changing an assignee destroys the task's position on the board. The configured
priorities and labels are lost the same way, on every command.

## Root cause

`Store` keeps only `Dir`, the task directory, and discards the marker it was
resolved from. Five call sites then re-derive the config by walking up *from the
task directory*, which never reaches a marker that lives elsewhere:

- `internal/store/store.go:379` (`Priorities`)
- `internal/store/store.go:390` (`Columns`)
- `internal/web/server.go:241`
- `internal/cli/label.go:61`
- `internal/cli/cli.go:254` (`cli.config`)

`config.FindConfig` then returns `(nil, nil)` — "no marker found" is
indistinguishable from "no config" — so every caller silently falls back to the
built-in defaults, and `columns.Normalize` rewrites any status outside them.

## What to build

- **Resolve the marker once, carry it.** Discovery already knows where the
  marker is when it resolves the task directory; the `Store` must keep it
  instead of throwing it away. Every consumer of the config reads it from there.
- **Delete the re-derivation.** No caller may walk up from `Store.Dir` to find a
  config. After this change, searching the tree for `FindConfig(st.Dir)` or
  equivalent should return nothing.
- **Stop conflating "none" with "not found."** `FindConfig` returning
  `(nil, nil)` is what makes the failure silent. A caller that resolved through
  a marker must never afterwards decide there is no config.
- **A `path:` outside the marker's directory must keep working.** It is
  documented, and `tq init` writes it under `TQ_DIR`. This task makes it work,
  not forbidden.

## Acceptance

- A project whose `path:` points outside the marker's directory keeps its board,
  priorities and labels on every command.
- `tq move <id> <a column the project declares>` is accepted.
- An unrelated edit (`--assignee`, `--title`, a note) never changes `status`.
- The same holds over HTTP: `/api/tasks`, `/api/status` and the board's columns.
- A test that fails if any code path re-derives the config from the task
  directory, so this cannot come back.

---

## Notes

Found by the code review during TQ-0022, which called it the highest-value fix
available and recommended fixing it in discovery rather than per call site.
Verified independently before filing.
- 2026-08-27T19:26:04+02:00 — Shape chosen for the FindConfig conflation: a distinct sentinel, config.ErrNoConfig, plus config.Optional to fold it where an absent marker really is fine.

  - config.FindConfig and config.ConfigIn now report ErrNoConfig instead of (nil, nil). The package invariant is now statable: nothing in internal/config ever answers with a nil *Config and a nil error.
  - config.Load(path) reads one marker by path — no walk. A marker that has gone missing is an error, not an absence.
  - config.Optional(cfg, err) folds ErrNoConfig to (nil, nil) for the callers that want the built-in sets; every other error still comes through.
  - Store gains Marker (the path, never the parsed file) and Store.Config(), which reads it from disk on every call.
- 2026-08-27T19:33:52+02:00 — TQ_DIR decision: it overrides where the task files are, and nothing else. The board, priorities and labels come from the marker at or above the directory the command was run in — with or without the variable. Rationale: 'tq init' under TQ_DIR writes the marker where it is run and 'path: ../queue' into it, so the project is the marker you are standing in; resolving the config any other way makes 'TQ_DIR=x tq move' and 'tq move' disagree in the same directory. With no marker anywhere, Store.Marker is empty, Store.Config reports config.ErrNoConfig, and the built-in sets apply.

  This changed one landed assertion: TestCLILabelListReadsTheConfigOfTheQueueItLists (TQ-0022) pinned the old behaviour, where the vocabulary came from a walk up out of the task directory. Its stated invariant — the CLI and the board must not disagree about which labels are configured — is preserved and is now structural, since both read Store.Config. The test is renamed and asserts the new rule.

  Guard against recurrence, two layers: tqtest.EscapedQueue plants a decoy marker directly above a task directory that sits outside its own project, and the store, CLI, web and integration tests all assert the decoy never reaches them (behavioural, and it fails loudly on HEAD). A test at the repository root also greps the non-test source for a config walk handed a task directory.
- 2026-08-27T19:47:54+02:00 — BLOCKED on a decision: the code review (high) found that my TQ_DIR rule reintroduces the ticket's own data-loss symptom in a different shape, and I reproduced it against both binaries.

    proj/.taskqueue.yaml declares columns backlog, in-review, shipped
    TQ-0001 is at status: in-review

    cd /a-directory-with-no-marker
    TQ_DIR=.../proj/.tasks tq note TQ-0001 'hello'   ->  exit 0
    status on disk                                    ->  inbox

  On HEAD this keeps 'in-review', because Columns() walks up from the task directory and finds proj/.taskqueue.yaml. Under the ticket's rule that walk is forbidden, so the store has no marker, falls back to the built-in board, and columns.Normalize rewrites the status on the next save.

  Every way out has a user-visible cost, so it goes back to the user:
   (A) as built: no marker means the built-in sets — silent data loss above;
   (B) no marker means no board at all, so tq stops rewriting a status it cannot validate — needs an 'unconfigured' state in internal/task, whose zero Columns is the built-in board today, and changes what tq move accepts and what /api/config shows;
   (C) TQ_DIR relocates the tasks but does not replace the marker: no marker at or above the working directory means exit 3 — cleanest invariant, but it breaks TestCLIEnvTaskDirOverride, a landed test that runs TQ_DIR from a directory that is not a project;
   (D) keep the walk from the task directory for the TQ_DIR case — violates the ticket's rule outright.

  Everything else in the change is finished and green: make test, -race, test-integration, lint, build. Not committed.
- 2026-08-27T20:01:21+02:00 — Decision from the user: TQ_DIR is replaced outright by TQ_CONFIG_PATH, which names a configuration FILE rather than a task directory. Breaking change to a documented interface, no deprecation alias — the same way TQ-0085 removed TQ_WALK_FOREVER.

  It settles the question above by removing it. A command now gets its marker one of two ways — handed to it by TQ_CONFIG_PATH, or found by walking up from the working directory — and either way it has one. There is no third state: no store without a marker, so nothing ever falls back to the built-in board and the data loss reproduced above cannot happen. A TQ_CONFIG_PATH that is missing, is a directory, or will not parse is an error, never an absence.
- 2026-08-27T20:14:55+02:00 — TQ_CONFIG_PATH landed. Shape:

  - config.EnvConfigPath = "TQ_CONFIG_PATH"; TQ_DIR is gone with no alias.
  - config.MarkerOverride() is the variable on its own, validated: missing, a directory, or unparseable is an error wrapping config.ErrConfig, never an absence. config.MarkerPath(startDir) is the override or the walk, and it is the only way anything gets a marker.
  - store.discover() has no environment branch left at all: MarkerPath, then Load, then the task directory that marker declares. InitStore takes the handed marker when there is one and writes none.
  - Store.Marker is therefore never empty on a store from InitStore or OpenStore, so nothing ever falls back to the built-in board. config.Optional survives for the two callers that genuinely may have no project: tq init in a fresh directory, and the CLI's help before there is one.

  Review findings in my own diff, all three addressed:
  - cli.go moveTask/runDone now validate against the store's board — the marker the queue was resolved through — so a marker tq cannot read is reported by name instead of the project being told its own columns do not exist. Test: TestCLIMoveReportsABrokenMarkerRatherThanTheBuiltInBoard.
  - config.TaskDir() is nil-safe and documented as the one accessor with no built-in answer; Optional's contract reworded to match.
  - The ordering concern in store.discover evaporated: there is no pre-walk to order against any more, since MarkerPath either takes the override or walks, never both.

  Guards: the source tripwire against re-deriving config from a task directory, a self-test so its regexp cannot rot into matching nothing, and a second guard that fails if the retired variable appears anywhere in the source, README.md, AGENTS.md or the generated guide.
- 2026-08-27T20:39:25+02:00 — Code review at high effort, over the 43-commit range plus the working tree: 13 findings, none of them inside a hunk this change wrote. Every one is pre-existing code the diff does not touch — verified by comparing the finding lines against git diff -U0 HEAD. The reviewer confirmed the fix itself against a real escaped-queue layout.

  The one worth a ticket of its own, and deliberately not fixed here:

    store.update persists columns.Normalize(t.Status) on every write, and Normalize maps a status the board does not declare to the first column. So a project that REMOVES a column from its own .taskqueue.yaml while tasks still sit in it loses their status on the next unrelated edit — exit 0, nothing printed. Same shape as this defect, different cause: TQ-0087 closed the wrong-board cause, not the dropped-column one. The read side (List, Get) shows the first column for such a task too, and the comment there claims it 'keeps it and sorts last' when Rank returns 0. Priority does not behave this way — a dropped priority is preserved — so the two vocabularies disagree.

  Also noted, all outside this diff: task.SortTasks gating on raw strings rather than ranks (intransitive under an edited vocabulary); entryInTheWay reporting a real task file as 'not a task file' and swallowing its own stat errors into the ten-attempt message; a failed second rename in update deleting the only copy of the new content; frontend write-then-refresh paths reporting a committed write as failed (state.ts quickAdd/moveTask, TaskDialog save/append); events.ts drop() leaving an orphan EventSource; state.ts loadProjectConfig without an in-flight ticket. The events.go subscribe/stop race, the guide's stale 'ready' sentence and the state.ts items were already on the known-and-deferred list.
- 2026-08-27T20:41:24+02:00 — The review's three worktree-scoped concerns, resolved:

  1. 'Standing in the escaped task directory reaches the wrong project.' Reproduced, and it is the rule working, not a defect of this change. With no other marker above the queue, cd-ing into it and running a command exits 3 — honest, since the project's marker is elsewhere. The reported corruption needs a SECOND project whose own marker declares that same directory as its queue; the walk from inside it then finds that project, which is what 'walk up from the working directory' means and the only answer available. What turns it into a lost status is store.update persisting columns.Normalize — the high finding above, pre-existing and outside this diff. Changing the discovery rule to avoid it would contradict the rule this ticket exists to establish, so it stays.

  2. 'Priorities and Columns fold ErrNoConfig into the built-in sets, which is the silence TQ-0087 removed.' Fixed. config.Optional is gone from both: a store with no marker now fails rather than being handed a board tq made up. Nothing in the tree depended on the fallback — only this change's own test did, and it now asserts the failure. config.Optional survives for its two honest callers, tq init in a fresh directory and the CLI's help before there is a project.

  3. 'AGENTS.md still documents exit 3 as .tasks missing and uncreatable.' Not so — that wording is not in the repository; AGENTS.md has said 'no task queue found' since TQ-0085. The other half, that TQ_CONFIG_PATH naming a file that does not exist cannot bootstrap a project the way the old variable did, is the specified behaviour: the variable hands over a marker, and tq init without it is how a project is created. Exit 1 for it matches every other unusable config.

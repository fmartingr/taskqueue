---
id: TQ-0061
title: The pointer tq init prints does not resolve from where it was run
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T17:44:51+02:00
updated: 2026-08-26T19:42:11+02:00
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

---

## Notes

- 2026-08-25T18:42:55+02:00 — After TQ-0029 the pointer has an unambiguous base — the directory holding .taskqueue.yaml — instead of being inferred from the guide's own location. This fix likely collapses into that one; sequence it after, not before.
- 2026-08-25T18:50:04+02:00 — Affected by TQ-0029 but deliberately not blocked on it: the pointer is wrong today. If TQ-0029 is imminent, do them together — the marker's directory is the unambiguous base this fix needs.
- 2026-08-26T19:21:29+02:00 — The relative @-pointer is gone as a concept: GuidePointer is now GuidePath, which returns the guide's absolute, cleaned path and picks no base at all. There was never a right base — the file a user references the guide from is theirs to choose, so any relative path resolved from one directory and misled from every other. Human output names the guide and invites the user to include it in whatever context file their tool reads; --json keeps the pointer key (stable agent API) with the absolute path as its value.

  Verified by hand with a built binary, all three shapes from the ticket:
    no repo root, TQ_DIR=../elsewhere/queue: @queue/AGENTS.md -> <base>/elsewhere/queue/AGENTS.md
    init from project/backend/deep: @.tasks/AGENTS.md -> <project>/.tasks/AGENTS.md
    inside a repository: @.tasks/AGENTS.md -> <repo>/.tasks/AGENTS.md
- 2026-08-26T19:21:38+02:00 — Tests: agents_test.go's strings.HasSuffix(pointer, "queue/AGENTS.md") is replaced by an exact comparison, and a new TestGuidePathNamesTheGuideAbsolutely covers a project with no repository root (the branch that had none), a run from a subdirectory, a repository, and a store carrying a relative Dir. The old assertion was what let TQ-0023 change this output with no test diff. Integration coverage added in discovery_test.go for both surfaces of tq init across the same three shapes; harness_test.go gains realPath, because a subprocess resolves its own working directory and macOS hands out /var paths that are really /private/var.
- 2026-08-26T19:21:38+02:00 — Observed, not fixed, out of scope: in the TQ_DIR-outside-the-project shape init declines to write the guide at all (withinInvokedTree, from TQ-0055 -- it will not write into a tree it was not invoked in) while still naming it, so the path it prints is a file that does not exist. Pre-existing: the old relative pointer named the same missing file. Worth a separate ticket if the message should say so.
- 2026-08-26T19:22:57+02:00 — Mutation check for TQ-0064's surviving GuidePointer mutant: the branch it lived on (from := filepath.Dir(st.Dir)) no longer exists, so the mutant is gone by construction. Confirmed the replacement is pinned anyway — reintroducing the same wrong base inside GuidePath (join AgentsFileName onto filepath.Dir(st.Dir)) turns five assertions red across two packages: all four GuidePath subtests, the configured-task-dir test, and TestCLIInitWritesTheGuideAndNothingElse. Under the old loose strings.HasSuffix all of those stayed green.
- 2026-08-26T19:41:40+02:00 — Code review at high effort, twice (the first agent's report arrived late; both covered the same diff). One CONFIRMED finding inside this change: tq --help still advertised init as printing 'the line to add to your own AGENTS.md/CLAUDE.md'. There is no line any more, so the usage text now says it prints where the guide is, to reference from your own AGENTS.md/CLAUDE.md.

  Both reviewers also flagged the pointer key keeping its name while its value changed from an include line to an absolute machine-local path, and suggested a new guide_path key. Not acted on: the user's direction for this task is explicit that the key stays and no keys are added or removed. Recorded here because it is a real consumer-visible change of meaning, and worth a ticket if the project wants the rename surfaced.

  Findings outside this diff, left alone and reported upward: events.go subscribe/stop race, App.vue openTaskID never cleared when the task leaves the listing, Composer.vue losing the typed title on a failed create, FilterBar.vue status select with no self-preserving option, store.update using Normalize rather than Check, server.go ServeMux assertion and statusRecorder.Flush, README's ready: true vs consider_ready, and the done <id> help line.

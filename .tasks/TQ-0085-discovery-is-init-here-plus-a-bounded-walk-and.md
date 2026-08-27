---
id: TQ-0085
title: Discovery is init-here plus a bounded walk, and TQ_WALK_FOREVER goes away
status: done
priority: high
labels:
  - bug
  - component/cli
  - component/store
created: 2026-08-27T11:18:48+02:00
updated: 2026-08-27T12:01:08+02:00
---

## Decision

Discovery is replaced by two plain rules, and `TQ_WALK_FOREVER` is deleted.

**`tq init` creates the root where it is run. Period. No discovery.**
The user decides where the project lives, so init never looks up, never adopts
a parent, and never relocates to a repository root. It writes `.taskqueue.yaml`
and the task directory in the current folder. It is idempotent: run in a folder
that already has a marker, it changes nothing but rewriting the generated guide.

**Every other command walks up for `.taskqueue.yaml`, and errors if there is none.**
The search starts at the current folder and climbs until it reaches the home
folder. Nearest marker wins. No marker found means the command errors — it has
no idea where to read or write, and guessing is what this replaces.

**No command creates a queue implicitly any more.** `tq init` is the only thing
that writes a marker.

## Why

`TQ_WALK_FOREVER` exists to lift a bound nobody can discover (TQ-0058), and the
repository-root bound it lifts is itself an inference about where a project
starts. Both go away if init is explicit and the search has one fixed stop.

This finishes what dfec6b4 ("docs: require tq init before other commands")
started: the README already says init is mandatory and other commands need the
marker to exist, while `AGENTS.md` still promises the opposite — "commands must
not fail merely because a project has not been initialised" — and the code
follows `AGENTS.md`. The two have to agree.

## What this reverses, deliberately

- **TQ-0047** (done) — "tq init forks the queue in a subdirectory instead of
  discovering the parent". Init forking in a subdirectory is now the specified
  behaviour, not a bug: the folder you run it in is the answer.
- **TQ-0017** (done) — "discovery walks past the repository root". The bound
  moves from the repository root to the home folder rather than disappearing,
  so the walk is still bounded, just differently.
- **TQ-0057** (rejected) — was rejected because the repository bound made it
  moot. The home-folder bound is what answers it now.

## What this retires

- **`TQ_WALK_FOREVER`** — the constant, the branch in `WalkBoundary`, the early
  return in `ShadowedProjectMarker`, its mention in the generated guide, README,
  `AGENTS.md`, and its use in `internal/tqtest`'s isolation set.
- **TQ-0062's shadow warning** — it exists to say "a marker above the repository
  was not used because of the bound". With the bound at the home folder that
  marker IS used, so the warning has no case left. Remove it and its tests.
- **The on-demand creation rule** in `AGENTS.md` and every code path that
  creates a task directory outside `tq init`.

## Knock-on

- **TQ-0059** (submodules fork the queue) is resolved by this: a submodule's
  `.git` no longer bounds anything, so a command inside one finds the
  superproject's marker. Close it when this lands, or re-verify and reject it.
- Exit codes need a decision recorded: today 3 is ".tasks missing and
  uncreatable". "No project found" is the common case now and should be a
  clear, stable code with a message naming `tq init`.
- `TQ_DIR` keeps overriding everything. Unchanged.
- Test fixtures (TQ-0063/TQ-0064) anchor with a marker, which still stops the
  walk. Fixtures with no marker relied on the repository bound; on macOS
  `TMPDIR` sits outside the home folder, so those now walk to the filesystem
  root and find nothing — which is the new correct answer (an error), but
  `tqtest` needs checking rather than assuming.

## Two judgement calls made while writing this, flag if wrong

- The walk checks the home folder itself and then stops (inclusive), so a
  marker at `~/.taskqueue.yaml` is usable.
- For a path that is not under the home folder at all (`/opt/thing`, a temp
  directory on macOS), the walk stops at the filesystem root instead. It cannot
  reach the home folder, and running to the root finds nothing rather than
  wandering into another user's tree.

## Acceptance

- `tq init` in a subdirectory of an existing project creates a marker there,
  and commands run in that subdirectory use it, not the parent's.
- `tq init` twice in a row changes nothing the second time.
- A command run where no marker exists at or above it, up to the home folder,
  errors with a message naming `tq init`, and creates nothing.
- `TQ_WALK_FOREVER` appears nowhere in the tree.
- `tq help` and the generated guide describe the two rules and no longer
  document a variable that does not exist.

---

## Notes

- 2026-08-27T11:39:40+02:00 — Exit codes: 3 keeps its number and has its description restated, rather than a new code.

  Reasoning: 3 has always meant 'tq could not reach a usable task directory' — 'no .tasks directory found' was the message, and the CLI already returned 3 for the no-project case whenever creation failed. TQ-0085 makes that case ordinary rather than exceptional, so the meaning was widened, not changed: 3 is now 'no task queue found'. A fourth code was considered and declined — it would have broken every agent or script that already treats 3 as 'no queue', for no gain, since a caller cannot act differently on 'no marker' than on 'marker without its directory': the answer to both is tq init.

  Every message behind ErrProjectNotFound now ends with: run "tq init" to create one. Four of them: no marker up to the home directory, no marker running out at the filesystem root, a marker whose task directory is missing, and TQ_DIR naming a directory that does not exist. The sentence lives in one constant (store.initHint) so the four cannot drift.

  What tq init returns when the current folder is unusable (relevant to TQ-0060, not fixed here): 1. runInit now calls store.InitStore directly, and a mkdir that fails comes back as a plain error, which c.fail maps to exitError. It is no longer wrapped in ErrProjectNotFound — that wrapping lived in OpenStore's create branch, which is gone. TQ-0060's drift from 1 to 3 is therefore closed as a side effect, but the ticket was not read or worked; re-judge it against this.
- 2026-08-27T11:39:53+02:00 — Fixtures, checked rather than assumed (the ticket asked).

  - tqtest.Root is unchanged in mechanism — its marker still stops the walk — but its contract narrowed: it now writes only the marker, and the task directory the marker names does not exist. That used to be invisible because every command created it; it is now a project only tq init can work in, so crosscut_test.go and several CLI tests gained an explicit init. Documented on the fixture.
  - RootWithGit and RootWithoutAnchor collapsed into RootWithoutMarker. Their difference was the repository bound, which no longer exists: both meant 'no marker here', and the only thing that can still guarantee that is the assertion RootWithoutAnchor already carried (config.ConfigPath finds nothing above). On macOS TMPDIR sits under /var/folders, outside the home directory, so an unmarked fixture now walks to the filesystem root — the assertion is what keeps that honest.
  - RootWithoutGit and RequireNoRepositoryAbove are gone: with .git meaning nothing, 'a project on a machine without Git' is just a project.
  - config.RepositoryRoot is deleted. Nothing in tq reads .git any more.
  - tqtest's isolation set is now {TQ_DIR} alone. The pin itself is untouched: Isolate still records the ambient values before clearing, and RequireIsolated still fails a package whose TestMain skipped the call, which is what TQ-0063 was about.
  - internal/integration grew bareDir, which asserts the same premise for the compiled-binary layer, and newProject now writes the marker AND mkdirs .tasks. It deliberately does not run tq init: the generated AGENTS.md would land in the queue that several tests count the entries of.
- 2026-08-27T11:40:04+02:00 — TQ-0059 verified against a real git submodule, not a simulated one: git init in two directories, tq init + tq add in the superproject, then git -c protocol.file.allow=always submodule add ../dep vendor/dep. The submodule's .git is a FILE (38 bytes, gitdir: pointer). From inside it, tq list prints the superproject's TQ-0001 and tq add files TQ-0002 into the superproject's .tasks; the submodule directory holds only .git and its own file. Nothing forked. TQ-0059 is resolved by this change and can be closed — not touched here.

  Also verified by hand: tq init in a subdirectory of a project creates a marker there and commands below it use it, not the parent's; tq init twice leaves the marker and the guide byte-identical; a command with no marker up to the home directory exits 3, says .taskqueue.yaml and tq init on stderr, prints nothing on stdout and creates no entries.

  One literal miss on the acceptance list: 'TQ_WALK_FOREVER appears nowhere in the tree' cannot hold for .tasks/ itself — this ticket's own body names it, as do the notes on TQ-0017, TQ-0058, TQ-0059, TQ-0062 and TQ-0063, which are the record of why it existed and how it went. It appears nowhere in the code, the docs, the generated guide, the test harnesses or the browser suite, and an integration test (TestWalkForeverIsGone) pins that: help, init and the generated guide must not name it, and setting it must not lift the home-directory bound.
- 2026-08-27T12:00:22+02:00 — /code-review at high effort, twice (the first agent went unresponsive after ~45 minutes; a fresh invocation returned, and the first then reported too). Two independent reviews, five distinct findings between them. Four fixed in this diff, two declined with reasons.

  FIXED

  1. TQ_DIR bypassed the near-miss guard, so tq init would write .taskqueue.yaml beside an existing .taskqueue.yml. A regression I introduced: WriteConfigIfMissing used to walk through FindConfig, which reported the typo; it now looks in one directory, and initTaskDir returns on the TQ_DIR override before ConfigIn is ever asked. reportNearMiss now guards the write as well as the read. Test: TestInitStoreRefusesToWriteBesideANearMiss, both with and without TQ_DIR.

  2. The 'run tq init' hint forked a second project when the marker exists and its task directory does not. Both reviewers reproduced it: tq list from repo/src/deep says the queue is missing and to run tq init; doing so creates repo/src/deep/.taskqueue.yaml and a second queue. That is the one ErrProjectNotFound case where the queue's location is already known, so the message now says: run "tq init" in <the marker's directory>. Pinned at both layers — store TestDiscoverTaskDirSaysWhereToInitialiseAMissingQueue and integration TestAMissingQueueSaysWhereToInitialiseIt.

  3. WalkBoundary compared HOME and the working directory literally, so a symlinked or automounted home (HOME=/home/u, getwd -> /export/home/u) lost the bound entirely. It now falls back to resolving both sides, and climbs the literal chain by the resolved depth so the answer is a directory the walk actually arrives at. Test: TestWalkBoundaryFollowsASymlinkedHome.

  4. The doc comments overclaimed. 'Runs out at the filesystem root, finding nothing is the right answer' is false: outside the home directory the walk searches every ancestor up to / and will use a marker it meets. discovery.go, AGENTS.md, README.md, tq help and the generated guide now say that plainly. Behaviour unchanged — this is the ticket's judgement call 2, implemented as written.
- 2026-08-27T12:00:38+02:00 — DECLINED, with reasons — both raised by /code-review, neither fixed here.

  A. tq init under TQ_DIR bakes the override into a committed marker. Both reviewers flagged it: cd ~/project && TQ_DIR=/tmp/scratch tq init writes ~/project/.taskqueue.yaml with path: ../../tmp/scratch, and every later command in that project resolves to the scratch queue with TQ_DIR unset. Real, and the relative path is machine-specific, which contradicts what the marker is documented to be.

  Not fixed because it is pre-existing and unchanged by this diff — verified against the old code: taskDirTarget returned the TQ_DIR override and WriteConfigIfMissing still wrote a marker (at the repository root then, in the current directory now) with a relative path to it. The ticket says TQ_DIR keeps overriding everything, unchanged, and whether init should decline to persist a per-invocation override is a product call this ticket did not put. Worth its own task: recommend init writes no marker when TQ_DIR supplied the task directory, since moving a queue already has a documented route (write path: in the marker, then init).

  B. tq init forks a nested queue in silence, now that TQ-0062's note is gone. Declined on principle rather than on scope: telling the caller about a marker above would require init to walk up for one, and rule 1 is that init never looks up. The silence is the rule working. What makes a fork recoverable instead is finding 2 above — the one message that could have led a user into forking now names the directory to run init in.

  Also noted by a reviewer and left alone: reportNearMiss runs at every level of the walk, so a stray .taskqueue.yml above a project outside the home directory now fails every command in the subtree. The mechanism predates this change (ConfigPath always checked at each level); only the reach of the walk changed. It is the marker rule working as designed — a typo is not an absent config — and narrowing it belongs with A.

  Checked and cleared by the reviews: help's format verbs match its arguments 12/12; the committed .tasks/AGENTS.md is byte-identical to what the generator now produces; no TQ_WALK_FOREVER or RepositoryRoot anywhere in code, docs, harnesses or the generated guide; RootWithoutMarker's assertion covers exactly the ground the unbounded walk covers, so the fixture stays honest; WalkBoundary's handling of '..', '../x' and dir == home is correct; Windows USERPROFILE is moot, goreleaser ships linux and darwin only.

---
id: TQ-0063
title: The test suite's own isolation is unpinned
status: done
priority: high
labels:
  - tests
  - bug
created: 2026-08-25T17:45:14+02:00
updated: 2026-08-26T18:53:20+02:00
---

## Finding

TQ-0021 and TQ-0023 stopped the suite writing into a developer's real task
directory. Little of that is pinned, so a later refactor reopens it silently.

- `TestMain`'s `isolate()` — the layer that fixes the reported bug, ambient
  `TQ_DIR` in a shell — has no guard. The three `TestFixtures*` tests each set
  the variables themselves, so they only ever observe the per-fixture
  clearing. Delete `isolate()` and run with `TQ_DIR` exported: all three still
  pass, while real files are written into the ambient queue.
- `TestCLIFixturesCannotReachAQueueAboveTempDir` hand-builds its own fixture
  and anchors it, so changing `newBareCLI` back to a bare `t.TempDir()` keeps
  the suite green — and that build, run with `TMPDIR` inside a repository,
  writes 24 files into its committed `.tasks` and deletes the real task.
- Five hand-built `testCLI` literals skip `newBareCLI`'s clearing entirely,
  leaving `isolate()` as their only barrier.
- `TestFixturesNeutraliseAmbientConfiguration` asserts on the environment
  rather than on where the store landed, and poisons `TQ_DIR` with the absolute
  `/somewhere/real/.tasks`; as root that would be created for real.
- The two "uncreatable" tests now rely only on `os.Chmod(root, 0o555)`, which
  does not constrain uid 0, and CI runs in a container. Before the anchoring
  they failed with ENOTDIR, which no privilege bypasses. A capability-
  independent fix the reviewer verified: put a regular file at the anchored
  root's `.tasks` path.
- `requireNoQueueAbove` reimplements `ShadowedTaskDir`'s walk without its
  variable handling, and skips on an ambient condition with no signal outside
  `-v`. Calling `ShadowedTaskDir` directly was verified green.

## Suggested fix

Give the fixtures one shared constructor, so nothing hand-builds a `testCLI`.
Assert on where the store landed rather than on environment variables. Take a
snapshot of the environment at init so `isolate()` itself can be pinned.

Found by `/code-review` over 20b06d2.

---

## Notes

- 2026-08-25T18:42:55+02:00 — Pin the isolation against marker-based discovery (TQ-0029), not the .git bound, or the pins land on a mechanism that is about to be replaced.
- 2026-08-26T18:25:27+02:00 — Structural constraint found before editing: internal/tqtest imports internal/store, so internal/store's in-package test cannot import tqtest (import cycle). To fold the four private helpers into tqtest, store_test.go moves to the external package store_test, with a small export_test.go exposing the two unexported symbols its tests reach (locate, retireOldFile). That is the idiomatic Go answer and keeps every fixture in one place.
- 2026-08-26T18:37:38+02:00 — Implemented. tqtest now owns every fixture: Isolate (with an init-time snapshot of the ambient environment and an isolated flag), RequireIsolated, ClearEnv, Root (marker-anchored, per TQ-0029), RootWithGit (.git-anchored, no marker), AboveFixtures, WriteConfig, NewStore, MustCreate.

  Root plants .taskqueue.yaml, so every test whose premise is 'this directory is not a project yet' had to move to RootWithGit: an absent marker can only be bounded by the repository root, there is no third anchor. That is more than the two tests the ticket named — it covers newBareCLI, the config package's five 'no config file' tests, and the store's marker-writing and no-marker discovery tests.

  internal/cli now builds every testCLI through newCLIIn; newBareCLI and newTestCLI go through it too, and label_test.go's private writeProjectConfig is gone in favour of tqtest.WriteConfig.
- 2026-08-26T18:53:14+02:00 — Verified by mutation on a scratch copy, not on this tree:
  - Deleting tqtest.Isolate() from all six TestMains, run with TQ_DIR exported: TestTheSuiteIsIsolated fails in all six packages.
  - Leaving the call but emptying Isolate's body, same environment: fails in all six with 'the value the shell exported: Isolate did not clear it'.
  - Root losing its marker: TestFixturesNeutraliseAmbientConfiguration, TestFixturesStayInsideTempDirWhenTMPDIRIsInARepository, TestCLIInitDoesNotWriteTheGuideOutsideTheInvokedTree fail.
  - RootWithGit losing its .git: TestOpenStoreCreatesAtTheRepositoryRoot, TestCLIFixturesCannotReachAQueueAboveTempDir, TestCLIInitWritesTheGuideInsideTheInvokedTree fail.
  - Removing the planted file from the two uncreatable tests: both fail because creation now succeeds (OpenStore returns nil, the CLI exits 0 where it must exit 3).
  - Control: with TQ_DIR and TQ_WALK_FOREVER both exported, the unmutated suite is green.

  The uid-0 property is by construction rather than by a container run (the Docker daemon was not up): os.MkdirAll under a regular file is ENOTDIR, a filesystem type error rather than a permission check, so no privilege bypasses it.

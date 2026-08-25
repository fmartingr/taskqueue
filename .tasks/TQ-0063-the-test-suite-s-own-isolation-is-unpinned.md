---
id: TQ-0063
title: The test suite's own isolation is unpinned
status: todo
priority: high
labels:
  - tests
  - bug
created: 2026-08-25T17:45:14+02:00
updated: 2026-08-25T17:45:14+02:00
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

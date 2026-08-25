---
id: TQ-0023
title: Tests escape t.TempDir() when TMPDIR is inside a git repository
status: done
priority: urgent
labels:
  - tests
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T16:58:10+02:00
---

## Finding

`root := t.TempDir()` is not an isolation barrier here: taskDirTarget/DiscoverTaskDir walk UP out of the temp dir looking for `.git` and `.tasks`, so whenever TMPDIR sits inside a git repository the tests bind to that repository's task directory.

Source: `store_test.go:14`

## How it fails

VERIFIED: with `TMPDIR=<repo>/tmp` (common in Bazel/Nix/containerised CI, and with `export TMPDIR=$PWD/tmp`), `go test ./...` walked past every t.TempDir() to the enclosing `.git`, wrote 20+ junk files into the repo's own committed `.tasks/`, clobbered `.tasks/AGENTS.md`, and deleted the real task file there. TestDiscoverTaskDirNotFound (line 448) and TestOpenStoreCreatesTaskDirOnDemand (line 57) also fail. Separate escape route from the TQ_DIR one and needs a separate fix (e.g. `os.MkdirAll(root/".git")` in the helper).

## Suggested fix

Give each test root its own `.git` marker so the upward walk stops there, independently of where TMPDIR points.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T16:51:29+02:00 — Still reproduces in full on the current build, verified while fixing TQ-0021: with TMPDIR inside a git repository, go test wrote 24 files into that repository's committed .tasks and destroyed the real task in it. Unaffected by TQ-0021, which closes the TQ_DIR route only.
- 2026-08-25T16:51:29+02:00 — Partly prepared: TQ-0047 added anchorProject and newBareCLI now marks its temp root as a repository root, so the CLI fixtures no longer walk out. newTestStore in store_test.go is not anchored, which is the remaining hole and the one this ticket's suggested fix names.
- 2026-08-25T16:58:10+02:00 — Fixed. New testRoot helper returns a temp directory marked as a repository root, so the walk up stops there, and every fixture root in store_test, cli_test and agents_test goes through it. newBareCLI's separate anchor is folded into the same helper.
- 2026-08-25T16:58:10+02:00 — The failing test needed care: t.TempDir fixes its base directory on first use, so a test that calls it before setting TMPDIR never sees the repository. The regression test builds its repo with os.MkdirTemp instead, and fails on the real bug without the fix.
- 2026-08-25T16:58:10+02:00 — Two tests needed adjusting for the anchor rather than for convenience. TestOpenStoreReportsUncreatableDir had creation fall back to the now-writable anchored root, so the root is made read-only. TestCLIDoesNotInventAnExcludedQueue asserts an environment-dependent negative, so it skips when a queue really does sit above the fixture.
- 2026-08-25T16:58:10+02:00 — Verified: go test with TMPDIR inside a git repository leaves that repository's committed queue byte-identical and the suite green, as does exporting TMPDIR, TQ_DIR and TQ_WALK_FOREVER together. Before the fix the same run wrote 24 files into it and destroyed the real task.

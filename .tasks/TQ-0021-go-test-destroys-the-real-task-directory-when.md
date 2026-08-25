---
id: TQ-0021
title: go test destroys the real task directory when TQ_DIR is set
status: todo
priority: urgent
labels:
  - review
  - tests
  - data-loss
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

newTestStore calls InitStore(root), which silently ignores `root` whenever TQ_DIR is exported, so the whole suite operates on the developer's real task directory instead of the temp dir — and one test then deletes it.

Source: `store_test.go:15`

## How it fails

VERIFIED by running the suite with TQ_DIR set to a directory holding one real task: `go test ./...` wrote 20+ junk task files into it, overwrote its `.tasks/AGENTS.md`, created an `AGENTS.md` in its parent (agents_test.go:13 passes `filepath.Dir(store.Dir)` as the doc root), and — via server_test.go:137 `os.RemoveAll(store.Dir)` in TestAPIFilesystemFailuresAreServerErrors — deleted the whole directory, permanently destroying the pre-existing task file. The README (line 112) and the generated guide both tell users to set TQ_DIR, and AGENTS.md's first rule is 'Run `make test` after backend changes'. No test helper ever calls `t.Setenv(EnvTaskDir, "")`.

## Suggested fix

Clear the override in the test helpers (`t.Setenv(EnvTaskDir, "")`) so no test can reach a directory outside its own t.TempDir().

Filed from a `/code-review` pass at max effort.

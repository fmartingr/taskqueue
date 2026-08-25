---
id: TQ-0021
title: go test destroys the real task directory when TQ_DIR is set
status: done
priority: urgent
labels:
  - tests
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T16:58:10+02:00
---

## Finding

newTestStore calls InitStore(root), which silently ignores `root` whenever TQ_DIR is exported, so the whole suite operates on the developer's real task directory instead of the temp dir — and one test then deletes it.

Source: `store_test.go:15`

## How it fails

VERIFIED by running the suite with TQ_DIR set to a directory holding one real task: `go test ./...` wrote 20+ junk task files into it, overwrote its `.tasks/AGENTS.md`, created an `AGENTS.md` in its parent (agents_test.go:13 passes `filepath.Dir(store.Dir)` as the doc root), and — via server_test.go:137 `os.RemoveAll(store.Dir)` in TestAPIFilesystemFailuresAreServerErrors — deleted the whole directory, permanently destroying the pre-existing task file. The README (line 112) and the generated guide both tell users to set TQ_DIR, and AGENTS.md's first rule is 'Run `make test` after backend changes'. No test helper ever calls `t.Setenv(EnvTaskDir, "")`.

## Suggested fix

Clear the override in the test helpers (`t.Setenv(EnvTaskDir, "")`) so no test can reach a directory outside its own t.TempDir().

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T16:51:29+02:00 — Verified before fixing: with TQ_DIR pointing at a scratch queue holding one real task, go test destroyed that task file. Afterwards the same run leaves the directory byte-identical, with both TQ_DIR and TQ_WALK_FOREVER exported.
- 2026-08-25T16:51:29+02:00 — Two layers. TestMain unsets TQ_DIR and TQ_WALK_FOREVER for the whole package, which is the actual reported bug: ambient configuration from the developer's shell. newTestStore and newBareCLI also clear them per test, so a fixture is isolated even when a test set them earlier.
- 2026-08-25T16:51:29+02:00 — TQ_WALK_FOREVER did not exist when this was filed. It is the same class of escape and is cleared alongside TQ_DIR.
- 2026-08-25T16:51:29+02:00 — The AGENTS.md damage described in the finding is already gone: TQ-0055 removed root-document handling, so nothing writes outside the task directory any more.

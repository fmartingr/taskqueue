---
id: TQ-0022
title: The atomicity test passes with atomicity removed
status: done
priority: high
labels:
  - tests
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-27T18:57:18+02:00
---

## Finding

TestUpdateRewritesFileAtomically verifies neither guarantee its name claims — not atomicity, not the updated-timestamp refresh; both assertions pass on deliberately broken code.

Source: `store_test.go:265`

## How it fails

VERIFIED by mutation, twice. (a) Replacing Store.write's CreateTemp/Chmod/Sync/Rename with a plain `os.WriteFile` — removing atomicity entirely, a headline guarantee in README:188 — leaves the whole suite green, because the only check is that the directory ends up holding one correctly-named file. (b) Deleting `task.Updated = time.Now().Truncate(time.Second)` from store.go:327 also leaves the suite green: the assertion is `updated.Updated.Before(updated.Created)`, and since both truncate to the second the values are equal, so `Before` is false whether or not the field is ever refreshed. This is why `go test` stayed green through all 15 findings.

## Suggested fix

Assert what the name claims: no temp file survives, the content is complete, and the updated timestamp actually moves (inject the clock or compare against a stored value rather than a same-second truncation).

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-27T18:35:38+02:00 — Stale references in the finding, corrected against HEAD:

  - Source is internal/store/store_test.go:996 (TestUpdateRewritesFileAtomically), and the dead assertion was at :1006.
  - Store.write no longer exists: TQ-0015 (82d3141) deleted it. Staging is Store.stage (store.go:1137-1168); the content is placed by the *second* os.Rename in update (store.go:998) and by os.Link in writeNew (store.go:1182).
  - The timestamp line is store.go:932, not store.go:327.
  - The README claim is README.md:487, not README:188.

  The old assertion could never fire for a second reason the finding did not name: Create sets Created and Updated from one 'now' (store.go:803), so Updated.Before(Created) is structurally impossible, not merely same-second truncation.
- 2026-08-27T18:35:45+02:00 — fsync is left untested, deliberately, and the README claim stands.

  Store.stage's tmp.Sync() (and fsx's) is the only primitive of the write path with no in-process observation: whether the bytes reached the platter before the rename made them the task only shows in a crash, and nothing short of a crash harness can see it. Faking a check for it would be worse than none. It is stated as a coverage boundary in a comment on TestUpdateLandsTheNewContentInOneStep and on the fsx test, rather than softened out of README.md:487 — the Sync is in the code and does back the sentence, so the claim is accurate.
- 2026-08-27T18:35:56+02:00 — What now kills what, each verified by re-applying the mutation on a scratch copy:

  - update writes with os.WriteFile at the destination (staging + final rename gone) -> TestUpdateLandsTheNewContentInOneStep (inode identity, and the staging hook never fires).
  - update stages but copies into the destination -> same test (staged-file identity, and the open-FD read tears).
  - store.stage drops tmp.Chmod(0o644) -> TestUpdateLandsTheNewContentInOneStep and TestCreateLandsTheWholeFileAtOnce (mode -rw------- vs -rw-r--r--).
  - writeNew writes O_CREATE|O_EXCL in place -> TestCreateLandsTheWholeFileAtOnce.
  - fsx.WriteAtomic becomes os.WriteFile -> TestWriteAtomicReplacesTheFileRatherThanRewritingIt.
  - fsx drops its Chmod -> the two fsx mode assertions.
  - update drops t.Updated = time.Now() -> TestUpdateRefreshesTheUpdatedTimestamp.

  The refresh is observed without waiting a second: a save receives whatever Updated the caller read off disk, so the test hands it a value from 2001 and asserts the save stamped now over it. No sleep, no injected clock.

  Inode identity goes through os.SameFile, not syscall.Stat_t, so nothing is build-tagged and the suite still builds on Windows.

  One test-only hook was added, Store.duringStage (exported as DuringStage in export_test.go), a sibling of duringUpdate from TQ-0015. It is the only window in which the staged file and the task's name both exist, and it is what makes 'the content was written somewhere else and moved' assertable for a create, where there is no earlier file to compare an inode against.
- 2026-08-27T18:56:59+02:00 — Code review at high effort found one thing in this diff: the mode assertions would fail on Windows, where Go reports 0666 for anything writable. They now go through a wantReadableByAll helper in each of the two test files, which returns early on windows. Linux and macOS still catch a dropped Chmod, which is where the mutation runs and where CI runs.

  Everything else the review raised is in files this task did not touch and is left alone: store.go:816 (Create refuses a dependency on the next free ID), store.go:389 and cli.go:251 (a marker whose path: points outside its own directory loses the board), discovery.go:54 (symlinked-home walk boundary), events.go:274 and :313, guide/agents.go:195, and four frontend findings.

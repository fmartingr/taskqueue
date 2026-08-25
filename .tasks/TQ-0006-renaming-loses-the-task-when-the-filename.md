---
id: TQ-0006
title: Renaming loses the task when the filename differs only in case
status: todo
priority: urgent
labels:
  - review
  - data-loss
  - store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

Store.Update decides whether to delete the old task file with a raw byte comparison of filenames (`if written != current {`), so when the on-disk name differs only in letter case or Unicode normalization the rename replaces that same directory entry and the following os.Remove then deletes the file that was just written — total task loss reported as success.

Source: `store.go:339`

## How it fails

Reproduced on this machine (default case-insensitive APFS): `tq add "Fix bug"` creates .tasks/TQ-0001-fix-bug.md; hand-rename it to TQ-0001-Fix-Bug.md (which taskFilePattern accepts and which readFile:279-280 explicitly calls harmless — "a stale title suffix ... gets fixed on the next write"); then `tq note TQ-0001 "progress"` prints `Note added to TQ-0001` and exits 0, but `ls .tasks` is now EMPTY and `tq show TQ-0001` returns `task not found` (exit 2). Same loss with NFD-vs-NFC names, the normal result of a git checkout of a .tasks directory committed from an HFS+ Mac. Fix: compare with os.SameFile, not `!=`.

## Suggested fix

Compare the old and new paths with `os.SameFile` (stat both) instead of `written != current`, and remove the old file only when it is genuinely a different file.

Filed from a `/code-review` pass at max effort.

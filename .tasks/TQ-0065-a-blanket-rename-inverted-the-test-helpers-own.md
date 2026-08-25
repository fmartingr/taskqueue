---
id: TQ-0065
title: A blanket rename inverted the test helpers' own comments
status: todo
priority: low
labels:
  - docs
  - tests
created: 2026-08-25T17:45:33+02:00
updated: 2026-08-25T17:45:33+02:00
---

## Finding

TQ-0023 replaced `t.TempDir()` with `testRoot(t)` across the test files by
substitution. Several comments were contrasting *against* `t.TempDir()`, so the
rename inverted their meaning.

`testRoot`'s own doc now reads "testRoot returns a temp directory that
discovery cannot climb out of", immediately followed by "testRoot(t) alone is
not an isolation barrier". Both sentences describe `testRoot`; the second is
true only of `t.TempDir()`. The same inversion appears at three more places in
`store_test.go` and `cli_test.go`, including "Not testRoot(t): it fixes its base
directory on first use", which is `t.TempDir`'s property.

A reader either distrusts the helper or copies the wrong contract into a new
test — and it invites deleting the anchor, which TQ-0064 shows nothing catches.

Separately, `newTestCLI`'s doc comment is orphaned: `requireNoQueueAbove` and
`anchorProject` were inserted between it and the function, across two commits,
so each carries another function's documentation.

## Suggested fix

Read the comments the rename touched and restore the contrast. Move the
orphaned doc back onto `newTestCLI` and give the inserted helpers their own.

Found by `/code-review` over 20b06d2.

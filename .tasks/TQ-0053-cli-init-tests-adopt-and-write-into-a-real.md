---
id: TQ-0053
title: CLI init tests adopt and write into a real queue above TMPDIR
status: todo
priority: high
labels:
  - bug
  - tests
created: 2026-08-25T14:47:08+02:00
updated: 2026-08-25T14:47:08+02:00
---

## Finding

Regression introduced by TQ-0047 (commit cad90f9). Because `tq init` now walks
up for a `.tasks`, the CLI test fixtures adopt whatever queue happens to sit
above `TMPDIR`. `newBareCLI` sets neither `TQ_DIR` nor a `.git` anchor, so
nothing bounds the walk. The pre-diff tree could not do this, because
`InitStore` never walked up.

## How it fails

Verified on both trees. With a `.tasks` present at `$SCRATCH` and
`TMPDIR=$SCRATCH/tmp`, the pre-diff tree passes `go test -run TestCLIInit ./...`
and leaves that queue empty. HEAD fails four tests — `TestCLIInit`,
`TestCLIInitWritesAgentDocs`, `TestCLIInitFindsTheQueueAbove`,
`TestCLIInitCreatesAtTheRepositoryRoot` — and writes `AGENTS.md` and a
`TQ-0001-existing-work.md` into that real queue.

A test suite that writes outside its `t.TempDir()` can damage a developer's
own tasks, and the failure depends on where `TMPDIR` points, so it will look
like flakiness.

`TestCLIInitCreatesAtTheRepositoryRoot` creates a `.git` anchor, but discovery
walks straight past it — the same root cause as TQ-0050.

## Suggested fix

Anchor the fixtures: set `TQ_DIR`, or bound discovery at the repository root
(TQ-0050), which also fixes this.

Found by `/code-review` over 004aa72~1..HEAD.

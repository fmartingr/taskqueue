---
id: TQ-0092
title: The browser harness leaks what it creates and hides its own failures
status: todo
priority: high
labels:
  - bug
  - tests
  - component/ci
created: 2026-08-28T12:57:54+02:00
updated: 2026-08-28T12:57:54+02:00
---

## Finding

Six defects in `browser/harness.ts`, all in how it manages the resources it
creates. Measured, not inferred.

**1. The built binary's temp directory is never removed.** `tqBinary()`
(`harness.ts:106`) calls `mkdtempSync(join(tmpdir(), "tq-browser-bin-"))` and
nothing anywhere calls `rmSync` on it. `Project.cleanup()` removes projects and
`server.stop()` removes log dirs; the binary dir has no owner. Measured on this
machine: **616 `tq-browser-bin-*` directories, ~3 GB** (a `tq` binary each),
plus **139 orphaned `tq-browser-log-*`** and several hundred project dirs.

**2. `open()` has an orphan window.** `opened.push(board)` happens in `show()`
(`harness.ts:417`), but `new Project()` creates a directory and
`await project.serve()` spawns a child *before* that. A throw in `seed()` or a
failed `serve()` leaves both unreachable by `afterEach`. Demonstrated: a
one-test file whose `seed` throws leaks exactly one temp dir. Observed at scale:
one broken-tree run produced 82 failures and 82 orphaned projects, and Bun
printed `killed 2 dangling processes` — its own safety net reaping servers the
harness lost.

**3. A failed `go build` is not cached.** `builtBinary` is assigned only on
success (`harness.ts:119`), so every `Project.run()` retries. Measured on a
broken tree: **82 tests x 1 `go build` each**, 19.8 s, 193% CPU, and 82
identical multi-line failures.

**4. `mustRun` validates the exit code only.** Under load `Bun.spawnSync`
returned `code = 0` with **empty stdout**, so `Project.add`'s `JSON.parse`
threw `SyntaxError: JSON Parse error: Unexpected EOF` and failed a real test.
Reproduced at ~1 in 400 in a targeted probe. `mustRun` should reject empty
stdout when `--json` was passed, and name the command.

**5. `browser.close()` costs exactly 30 s under load** — Playwright's graceful
close timeout, then SIGKILL. That is the `a beforeEach/afterEach hook timed out`
failure that appears once per file on a loaded runner, including on files
nobody touched. With 12 test files that is 12 potential 30 s stalls.

**6. The bun test timeout equals Playwright's default action timeout.** Both are
30 000 ms (`harness.ts:45` sets bun's; nothing calls `page.setDefaultTimeout`),
and the test's clock starts first — so a hung wait is torn down before
Playwright's own timeout fires. The result is
`this test timed out after 30000ms` plus
`Unhandled error between tests: ... Target page, context or browser has been closed`,
with the selector, the DOM state and the real message all lost. The comment at
`harness.ts:40-45` justifies 30 s as what "keeps a real failure legible"; it
currently guarantees the opposite.

## Why they matter together

4, 5 and 6 compound: a flake produces an illegible error, which produces a
30 s stall, which takes the file's Chromium down and fails every later test in
that file. That is why browser failures in this repo report as
"Target page, context or browser has been closed" rather than naming anything.

## Suggested fix

Each is small and independent. Ordered by value: **6** (make failures legible —
raise `TEST_TIMEOUT_MS` above the Playwright default, or set
`page.setDefaultTimeout` below it), **4** (reject empty stdout), **1** (remove
the binary dir on exit), **2** (register cleanup before spawning), **3** (cache
the build failure), **5** (bound `browser.close()`).

## Acceptance

- A full `make test-browser` run leaves no `tq-browser-*` directory behind.
- A `seed()` that throws leaks nothing.
- A broken tree fails once, legibly, not once per test.
- A wait that times out names what it was waiting for and what it saw.

Found across several code review passes during TQ-0080; every measurement above
was taken on this machine.

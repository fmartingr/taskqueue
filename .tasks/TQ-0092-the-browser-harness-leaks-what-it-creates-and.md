---
id: TQ-0092
title: The browser harness leaks what it creates and hides its own failures
status: done
priority: high
labels:
  - bug
  - tests
  - component/ci
created: 2026-08-28T12:57:54+02:00
updated: 2026-08-28T23:20:32+02:00
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

---

## Notes

- 2026-08-28T23:20:05+02:00 — All six fixed in browser/harness.ts, plus one the measurements uncovered.

  Timeouts (6): lowered Playwright's rather than raising bun's. PAGE_TIMEOUT_MS
  20s, TEST_TIMEOUT_MS unchanged at 30s, both pinned so no library default can
  swap the order back. Raising bun's to 60s was tried first and reverted: it
  made failures legible but doubled what a wedged file costs, since bun's clock
  is also what every test pays once its browser is gone (one file went from 4.5
  to 9 minutes). 20s is above every deliberate wait in the suite.

  Leak counts, whole temp directory, name-level diff over a full run: 680
  tq-browser-bin-*, 204 tq-browser-log-*, 412 project dirs before; a full
  make test-browser now adds zero of any kind, on eight consecutive runs
  including ones with 1, 4, 5, 7 and 9 failures. Suite went 297.8s -> 72.0s on
  a clean run.

  The binary directory (1) has no owner bun can give it at the end of a run:
  process.on("exit") does not fire under bun test, and a module-scope afterAll
  belongs to whichever file loaded the module first, not to the run. So it is
  built and removed per test file instead — 0.17s of warm go build each.

  The orphaned tq-browser-log-* had a cause nobody had named: closeSync on the
  descriptors handed to Bun.spawn raised EBADF, which abandoned the rmSync on
  the next line. The descriptors outlive the child, and closing a number the OS
  has since reused closes whatever now holds it. They are closed immediately
  after the spawn now — the child has its own copies and nothing here ever
  writes through the originals.

  Not fixed, and not this ticket: the first page.click in live.test.ts or
  notes.test.ts still wedges in a loaded full run, roughly two runs in three on
  this machine (load average 6-9, Spotlight indexing the 4,100 leaked
  directories at 150% CPU). Both files pass alone, and all twelve pass
  file-by-file, 99/99, every time. It failed the same way before this change
  (baseline: 93 pass, 7 fail, live.test.ts). What did change is that the first
  failure now names the selector and what the locator resolved to, instead of
  'Target page, context or browser has been closed'.

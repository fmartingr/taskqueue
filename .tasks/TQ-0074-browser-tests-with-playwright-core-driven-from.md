---
id: TQ-0074
title: Browser tests with playwright-core, driven from bun test
status: done
priority: normal
labels:
  - tests
  - component/frontend
  - component/ci
created: 2026-08-25T22:39:54+02:00
updated: 2026-08-25T23:27:22+02:00
---

## Decision

Browser coverage uses **`playwright-core`**, driven from `bun test`. Decided
after measuring the alternatives rather than arguing about them.

Rejected: `happy-dom` (10 packages, simulated DOM, weakest exactly where
`app.ts` is riskiest — drag and drop would be synthetic events); and
`@playwright/test` (brings its own runner, config and fixtures beside the Bun
one, for no gain here).

## What it costs, measured

- `bun add -d playwright-core`: **1 package, 13 MB**, and `bun.lock` appears.
- **The browser is not included.** `playwright-core` ships no binary. A clean
  checkout and CI both need `playwright install chromium`, a few hundred MB,
  plus a cache step in the workflow. This is easy to miss on a machine that
  already has the cache from something else.
- Verified working before deciding: `bun test` launched Chrome for Testing
  through `playwright-core` and asserted against the DOM, one pass in 4.15s.
  It needed an explicit `executablePath`; on macOS the binary is inside
  `Google Chrome for Testing.app`, not `Chromium.app`.

## Not a rule change

The "no JavaScript dependencies" line meant the frontend runtime and its build —
no npm packages, no Node, no bundler but Bun — not test tooling. A browser driver
used only by tests sits outside that, so nothing has to be relaxed to allow it.

`AGENTS.md` said "no JavaScript dependencies at all", which does not say that,
and is the wording that made this look like a decision needing approval. It has
been corrected: the constraint is on what the built `public/` output depends on.

What still holds, and should be checked in review: `frontend/build.ts` gains no
import, and nothing playwright touches ends up in `public/`.

## Shape

Mirror what the Go integration harness already does, because the hard parts are
the same ones:

- Start the real binary with `Bun.spawn`, `tq serve --port 0`, read the address
  from its banner, and point the browser at it.
- One temp project per test, carrying its own `.taskqueue.yaml`, so discovery
  cannot climb out and tests stay parallel-safe.
- Kill the server on cleanup and print its stderr when readiness fails, rather
  than a bare timeout.
- Locate the browser from `PLAYWRIGHT_BROWSERS_PATH` or the default cache, and
  fail with the install command when it is missing rather than a stack trace.

## What to cover

The parts of `frontend/app.ts` that no test reaches today:

- Drag and drop between columns, and that the move reaches the API.
- The inline composer: opening it, adding a task, keeping it open, escaping out.
- The task dialog: opening a card, editing fields, saving, closing.
- Note editing through the panel, which is the multi-line path TQ-0048 added.
- The poll that refreshes the board, and that it skips while dragging, composing
  or with a dialog open.
- A task created by the CLI appearing in the board without a reload.

## Also needed

- `make test-browser`, separate from `make test` and `make test-integration`.
- A CI job installing and caching the browser.
- The frontend build stays dependency-free: this is test-only, so `build.ts`
  must not gain an import.

## Acceptance criteria

- `bun.lock` is committed, and `build.ts` still imports nothing.
- `make test-browser` passes locally and in CI, and says how to install the
  browser when it is missing.
- The suite is parallel-safe and each test cleans up its server.
- `make test` and `make test-integration` are unchanged in scope and speed.

---

## Notes

- 2026-08-25T22:40:15+02:00 — Decision recorded rather than proposed: playwright-core driven from bun test, chosen over happy-dom and @playwright/test after measuring each. Verified working before deciding — bun test launched Chrome for Testing through playwright-core and asserted on the DOM.
- 2026-08-25T22:40:15+02:00 — The cost that is easy to miss: playwright-core ships no browser, and it only worked here because this machine already had Playwright's cache. A clean checkout and CI both need playwright install chromium and a cache step.
- 2026-08-25T22:46:21+02:00 — Correction: this is not a rule change. The no-JavaScript-dependencies line was about the frontend runtime and its build — npm, Node, a bundler other than Bun — not about test tooling, so a browser driver needs no exception.
- 2026-08-25T22:46:21+02:00 — AGENTS.md said 'no JavaScript dependencies at all', which is what made this read as a bigger decision than it is. Reworded so the constraint is on what the built public/ output depends on. bun.lock is simply committed, like any dev dependency.
- 2026-08-25T22:58:44+02:00 — Harness up: browser/harness.ts builds tq once, gives each test a temp project with its own .taskqueue.yaml, starts tq serve --port 0 and reads the banner, resolves Chromium from PLAYWRIGHT_BROWSERS_PATH or the platform cache. First 7 tests pass in 6s — real HTML5 drag and drop works through Playwright's CDP drag interception, no synthetic events needed.
- 2026-08-25T23:23:26+02:00 — Found the real flake, and it was not the browser: tq serve logs every request on stderr, and a Go process that writes to a broken pipe on fd 1 or 2 is killed by SIGPIPE. Draining the pipes from Bun was not reliable enough — a cancelled or collected stream killed the server mid-test. The server's stdout and stderr now go to files, which cannot break, and stderr() reads the file back for failure messages.
- 2026-08-25T23:27:22+02:00 — Suite covers all six behaviours from the ticket, 22 tests in ~38s: drag and drop between columns (and the move reaching the API), the composer, the task dialog, the notes editor, the poll refreshing the board, and the poll standing down while dragging, composing or with either dialog open. make test-browser and make browser-install added, plus a CI job that caches ~/.cache/ms-playwright keyed on bun.lock. Verified test-only: build.ts is byte-identical and internal/web/public/ is unchanged by a rebuild.

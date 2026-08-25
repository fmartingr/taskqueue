---
id: TQ-0074
title: Browser tests with playwright-core, driven from bun test
status: todo
priority: normal
labels:
  - tests
  - component/frontend
  - component/ci
created: 2026-08-25T22:39:54+02:00
updated: 2026-08-25T22:40:15+02:00
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

## The rule this changes

`AGENTS.md` says: *no JavaScript dependencies at all, so there is no lockfile to
commit*. That stops being true. Amend it in the same change — state that the
frontend runtime still has no dependencies, that the only one is a browser
driver used by tests, and that `bun.lock` is committed.

Do not let this become a precedent by accident: the rule should say what would
justify the next dependency, not just that this one was allowed.

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

- `bun.lock` is committed and `AGENTS.md` explains why.
- `make test-browser` passes locally and in CI, and says how to install the
  browser when it is missing.
- The suite is parallel-safe and each test cleans up its server.
- `make test` and `make test-integration` are unchanged in scope and speed.

---

## Notes

- 2026-08-25T22:40:15+02:00 — Decision recorded rather than proposed: playwright-core driven from bun test, chosen over happy-dom and @playwright/test after measuring each. Verified working before deciding — bun test launched Chrome for Testing through playwright-core and asserted on the DOM.
- 2026-08-25T22:40:15+02:00 — The cost that is easy to miss: playwright-core ships no browser, and it only worked here because this machine already had Playwright's cache. A clean checkout and CI both need playwright install chromium and a cache step.

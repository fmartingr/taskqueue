---
id: TQ-0099
title: The harness still ties its own timeout and orphans a page it gave up closing
status: todo
priority: normal
labels:
  - bug
  - tests
  - component/ci
created: 2026-08-29T08:24:31+02:00
updated: 2026-08-29T08:24:31+02:00
---

## Finding

TQ-0092 fixed the harness's leaks and set out to make its failures legible. Four
things in the committed result still work against that, and one of them is the
same defect TQ-0092 was filed to remove.

**1. The timeout fix does not cover the first await.** `page.setDefaultTimeout`
is applied **after** `browser.newPage()`, so the first await on a fresh page
still runs on Playwright's own 30 s — tied with `TEST_TIMEOUT_MS`, which is
exactly the tie TQ-0092 removed everywhere else. The test's clock wins, the
Playwright error is orphaned as `# Unhandled error between tests`, and the
selector and DOM state are lost. Apply the timeouts before anything can await on
the page.

**2. Giving up on `page.close()` after 5 s orphans a live page.** It keeps
polling a server that is about to be killed. This is a plausible cause of the
cascades still seen in `live.test.ts` and `notes.test.ts` — the page outlives
the fixture and the next test inherits a machine still serving the last one.

**3. 5 s is short for a Chromium close under load.** The bound exists so a wedged
close cannot cost 30 s, which is right; the number is not. Pick one that
tolerates a loaded runner, or make the bound adaptive.

**4. The shared readiness deadline can blame a half it never probed.** One
deadline spans both `waitFor` phases, so a timeout in the first can be reported
against the second. The message names the wrong thing.

## Evidence

Found by the code review during TQ-0098, against TQ-0092's committed work
(`3d69f8b`). The reviewer's control run has `origin/main`'s harness passing
98/98 as well, so these are **load behaviour, not a regression** — the suite is
green file-by-file either way.

Independently observed this session: full runs wedge roughly two in three on a
loaded machine, always the first `page.click` in `live.test.ts` or
`notes.test.ts`, while every file passes on its own. Each wedge leaves ~3 temp
directories, which is teardown being killed rather than the harness leaking —
TQ-0092's leak fix holds (measured delta 0 across a clean full run).

## Suggested fix

Take 1 and 2 first; they are the ones that cost information and cascade. 3 and 4
are small once those are settled.

## Acceptance

- A wait that times out on a fresh page reports the selector and what it saw,
  not `Target page, context or browser has been closed`.
- A page that will not close is not left polling.
- A timeout message names the phase that actually timed out.
- The leak stays closed: a full run leaves zero new `tq-browser-*` directories.

Not independently reproduced — **confirm before fixing.**

---
id: TQ-0077
title: CI browser job cannot find Chromium on Linux
status: todo
priority: urgent
labels:
  - bug
  - tests
  - component/ci
created: 2026-08-26T10:18:43+02:00
updated: 2026-08-26T10:18:43+02:00
---

## Symptom

CI is red on `main`. The `browser` job fails, 0 pass / 4 fail, every test in
`browser/` dying before it starts:

```
error: Found chromium-1234 in /home/runner/.cache/ms-playwright but no
executable inside them.
    at findChromium (browser/harness.ts:130:13)
```

Run 32945901595, job 98106423032. Every other job is green.

## Cause

`browser/harness.ts` hard-codes the relative path to the executable inside a
`chromium-<revision>` directory, and the list is **half migrated to Chrome for
Testing**:

```
chrome-mac-arm64/Google Chrome for Testing.app/…   ← new naming
chrome-mac/Google Chrome for Testing.app/…         ← new naming
chrome-linux/chrome                                ← legacy naming
chrome-win/chrome.exe                              ← legacy naming
```

The installer in this run says what it actually laid down:

```
Chrome for Testing 151.0.7922.34 (playwright chromium v1234)
  downloaded to /home/runner/.cache/ms-playwright/chromium-1234
```

Chrome for Testing archives are `chrome-linux64`, `chrome-mac-x64`,
`chrome-mac-arm64`, `chrome-win64` — the sibling download in the same log is
`chrome-headless-shell-linux64.zip`, which is the same convention. On Linux the
binary is at `chrome-linux64/chrome`, and the list only knows `chrome-linux`.

So this passes on an arm64 Mac and fails on Linux, which is why it was not
caught: the macOS entries were already fixed — the comment above the list calls
that "the single most expensive detail in this file" — and Linux never ran until
CI did. The cache was cold (`Cache not found for input keys`), so nothing here
is a stale-cache problem.

## Fix

**Preferred: stop hand-rolling the lookup.** playwright-core knows where it
installed its own browsers; `chromium.executablePath()` resolves it, and
launching without an explicit `executablePath` resolves it too. That deletes
`browsersRoot()`, `EXECUTABLES` and `findChromium()` along with this entire
class of bug. Keep the friendly install hint by catching the failure and
re-throwing with `INSTALL_HINT`.

It also fixes a second thing quietly: the run installed
`chromium_headless_shell-1234` as well, and current Playwright prefers the
headless shell for headless runs. Forcing an `executablePath` overrides that
choice; letting the library resolve it does not.

**Fallback, if the explicit path must stay:** add `chrome-linux64/chrome`,
`chrome-win64/chrome.exe` and `chrome-mac-x64/…`, and keep the legacy entries
for older caches.

## Verify on Linux, not locally

A green run on an arm64 Mac proves nothing here — that is exactly how this
shipped. Verify by running the workflow (or `docker run --rm -it ubuntu` with
bun + `bunx playwright-core install chromium`) and confirming the resolved path
under `chromium-1234`.

While in there: `findChromium` returns the first executable found across *all*
revisions, so a developer with an older revision in their cache keeps passing
while the newest install is broken. Prefer the newest revision that actually
resolves, or let the library decide (see above).

## Acceptance criteria

- The `browser` job passes on `ubuntu-latest` from a cold cache.
- The suite resolves the browser the same way on Linux and macOS, without a
  per-platform path list in the repository — or, if the list stays, with an
  entry for every layout Playwright currently ships.
- `make browser-install` then `make test-browser` works on a machine with an
  empty `~/.cache/ms-playwright`.

## Separate, not blocking

The run also warns that `actions/checkout@v4`, `actions/setup-go@v5` and
`actions/cache@v4` target Node 20 and are being forced onto Node 24. Worth a
follow-up bump; it is not what broke this.

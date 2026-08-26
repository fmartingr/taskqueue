---
id: TQ-0012
title: A rename during a listing makes the whole list 404
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T18:01:38+02:00
---

## Finding

Store.List is a TOCTOU — it snapshots filenames with os.ReadDir then os.ReadFiles each one (`task, err := s.readFile(name)`) — so a concurrent CLI title change that renames a file mid-listing makes readFile return ErrTaskNotFound, which writeStoreError maps to HTTP 404 for the ENTIRE task list.

Source: `store.go:243`

## How it fails

I reproduced this independently: with a 29-task queue, a running `tq serve` and a loop of `tq update TQ-00NN --title ...`, 1 of 120 `GET /api/tasks` calls returned 404 (a sibling angle measured 50/400 = 12.5% with a 120-task queue and a tighter loop). Interleaving: the handler ReadDirs and gets TQ-0050-task-number-50.md; a CLI process completes store.Update, renaming it; the handler reaches the stale name, os.ReadFile returns ENOENT, readFile:270 converts it to ErrTaskNotFound and List aborts. This fires on precisely the workflow the design brief advertises ("CLI edits are visible to a running server"). The frontend swallows it — refreshQuietly (app.ts:471) only console.errors — so the board silently freezes on stale cards; on first load the user gets an empty board. A second terminal's `tq list` exits 2 for a task that exists.

## Suggested fix

Treat a file that disappeared between ReadDir and ReadFile as skipped rather than fatal.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-26T18:01:38+02:00 — Revalidated on main (2d8ddfa): still present, measured under load.

  Store.List remains a TOCTOU — it snapshots names with os.ReadDir then reads each one (internal/store/store.go:305-325, return nil, err on any read error), and readFile turns os.ErrNotExist into ErrTaskNotFound (store.go:372-378), so a rename from a concurrent tq update --title aborts the entire listing.

  Measured against a real tq serve with 60 tasks and a rename loop, 25s:
    GET /api/tasks   total=1588  200=1505  404=83   (5.2%)
    tq list --json   runs=1211   failures=75        (6.2%, each exit 2)

  404 body: {"code":"task_not_found","error":"task not found: TQ-0042-....md"} — writeStoreError maps ErrTaskNotFound to 404 for the whole collection response (internal/web/server.go:306, applied at :144).

  One thing changed since filing: the board no longer freezes forever. refreshQuietly (frontend/state.ts:171-181) sets a stale flag so the fallback poll retries, so the board recovers instead of sitting on stale cards. That is symptom mitigation in one client — the store-level defect and the CLI's exit 2 are untouched, and the suggested fix (treat a file that vanished between ReadDir and ReadFile as skipped rather than fatal) still applies at internal/store/store.go:321-324.

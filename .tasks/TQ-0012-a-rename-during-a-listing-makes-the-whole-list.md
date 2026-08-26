---
id: TQ-0012
title: A rename during a listing makes the whole list 404
status: done
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T22:20:34+02:00
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
- 2026-08-26T22:06:53+02:00 — The 404 in the title was already closed by TQ-0011 (a128222): List skips a file that vanished between os.ReadDir and readFile. Measured on that commit with 60 tasks, a live tq serve and a concurrent tq update --title loop, 500 requests: 200=500, 404=0, and tq list --json exited 0 every run.

  What was left is what this task fixed: 46 of those 500 listings silently returned fewer than 60 tasks, and 13 of 200 tq list --json runs were short and said nothing about it. A rename between the directory read and the file reads leaves the task under a name the pass never looked at; a task created in that window is in no name at all.

  Store.List now reads the directory, scans, reads the directory again and compares the task-file name sets. A difference means the snapshot is of no single moment, so the pass is redone, up to 3 attempts. On exhaustion the listing is returned with Listing.Incomplete set rather than passing as the whole queue: the CLI warns on stderr (stdout stays pure JSON, exit stays 0) and GET /api/status carries "incomplete". A file that cannot be parsed does not move the directory, so TQ-0011's skip-and-report is unchanged and never triggers a retry.

  After: 200=500, 404=0, 0 responses short; tq list --json 200 runs, all exit 0, none short.
- 2026-08-26T22:20:30+02:00 — Review pass found two more things in this diff, both fixed here.

  The consistency check was blind to a duplicate: tq update writes the new file before retiring the old, so mid-retitle one task has two files and both readings of the directory can fall inside that instant and agree. The pass then held the task twice and called itself consistent. A repeated ID is now the same signal as a changed entry set, so the pass is redone. A pair that outlives the retries is a queue with two files claiming one ID — TQ-0040's to resolve, not this one's — and is reported through Listing.Incomplete rather than retried forever.

  The integration writer goroutine outlived its test: both new tests t.Fatalf inside their loops, which skips the explicit stop, leaving tq update forking at a temp directory being removed. It is registered with t.Cleanup now and the stop is idempotent. The assertions also went from a length check to one-of-each-ID, so a listing that drops one task and holds another twice cannot pass.

  Incomplete is wired to the board the way TQ-0011 wired unreadable: a toast when it appears, a footer segment while it lasts. Covered by a browser test that plants a second file for one ID, which is the deterministic way to make the server report it.

  Final measurement, 60 tasks, live tq serve, concurrent tq update --title loop: GET /api/tasks 200=500 404=0, 0 short, 0 long, 0 responses that were not one of each ID; tq list --json 200 runs, all exit 0, none short.

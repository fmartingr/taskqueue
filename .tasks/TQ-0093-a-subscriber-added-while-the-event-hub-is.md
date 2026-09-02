---
id: TQ-0093
title: A subscriber added while the event hub is stopping is never closed
status: todo
priority: high
labels:
  - bug
  - component/api
created: 2026-08-28T12:58:14+02:00
updated: 2026-09-02T22:43:47+02:00
---

## Finding

`subscribe()` checks whether the hub has stopped, then does real work before
registering the subscriber, so a `stop()` landing in that window strands it.

`internal/web/events.go` (~:274-299): the
`select { case <-h.done: ...; default: }` is check-then-act. After it passes,
`subscribe` does disk I/O — `taskFingerprint` is a readdir plus a stat per file,
`configFingerprint` walks to the marker — **before** taking `h.mu` and inserting
into the subscriber map. If `stop()` runs in that window, `closeAll()` has
already drained the map, so the new subscriber lands somewhere nobody reads
again and its `wake` channel is never closed.

**Verified**: an in-package test that subscribes in a goroutine, sleeps 50 us,
then calls `stop()` failed on iteration 0 — `wake never closed after stop; still
registered=true`.

## How it fails

Ctrl-C on `tq serve` while a board is reconnecting (`frontend/events.ts` retries
from 500 ms). The handler sits in its select, `r.Context()` is never cancelled,
and `httpServer.Shutdown` burns its full 5 s before printing
`shutdown: context deadline exceeded`.

`TestSubscribeAfterStopIsAlreadyClosed` covers only the sequential case.

## Suggested fix

Re-check `h.done` inside the same `h.mu` critical section that does the insert.

## Also in the same file, lower

- **`writeEvent` discards the write error** (~:417-424; callers ~:387, :404,
  :412). `Fprintf`'s error is dropped and `Flush()` returns nothing, so the
  handler has no exit signal. A half-open connection never cancels
  `r.Context()`, so the goroutine, its ticker and its map entry survive until
  TCP gives up. Worse, once the peer's socket buffer fills the 25 s ping blocks
  *inside* `Fprintf` — the server sets only `ReadHeaderTimeout` — and
  `closeAll()` closing `sub.wake` cannot free a goroutine parked in `Write`.
- **A second board never hears a standing scan failure** (~:237-245, cleared at
  ~:313-315). `h.lastErr` suppresses repeat `scan-failed` frames and is cleared
  only when the last subscriber leaves, so a board opened while another is
  already connected gets the two opening frames and never the error. The two
  boards then disagree.
- **Every connection scans the disk and throws the reading away** (~:288-297).
  `freshTasks`/`freshConfig` are computed unconditionally but kept only when
  `len(h.subscribers) == 0`, which is the opposite of the "one scan serves every
  board" invariant the type comment states. It also bypasses `h.scans`, so
  `scanCount()` under-reports real disk reads.

## Acceptance

- A subscriber added concurrently with `stop()` always has its `wake` closed.
- Ctrl-C during a reconnect does not hold `Shutdown` to its timeout.
- A test that fails without the fix (the reproduction above is deterministic
  enough to use).

Found by a code review of the TQ-0033/TQ-0034 event stream work, re-flagged in
several later passes. Line numbers drift; locate by symbol.

---

## Notes

- 2026-09-02T22:43:47+02:00 — TQ-0104 is the third finding here from the other side, and it is filed because a test already fails on it: TestAScanFailureIsReportedAgainToAFreshBoard, red on main.

  The clearing this task suggests — h.lastErr cleared when the last subscriber leaves — is in the code today, at subscribe's release. It is not enough. A board that reconnects overlaps its own teardown, so the map is never empty between the two streams and the standing failure is never reported again. Both findings are the same root: lastErr is hub-wide state answering a per-subscriber question. Fixing that closes both.

  The other two findings here — the subscribe/stop race and writeEvent discarding its error — are untouched by TQ-0104.

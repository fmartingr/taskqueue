---
id: TQ-0104
title: A board reconnecting while the old stream is still closing never hears a standing scan failure
status: todo
priority: high
labels:
  - bug
  - component/api
  - tests
created: 2026-09-02T22:43:38+02:00
updated: 2026-09-02T22:43:47+02:00
---

`make test` is red on main. `internal/web`'s
`TestAScanFailureIsReportedAgainToAFreshBoard` fails at the second board's
wait, and has done for long enough that four tasks have written it off in their
notes as unrelated to themselves: TQ-0069, TQ-0071, TQ-0102, and again while
TQ-0103 was verified. A gate nobody can read is a gate nobody reads.

The test is right and the hub is wrong.

## Finding

`h.lastErr` is one string on the hub, and it answers a question that belongs to
each subscriber: *has this board been told?* A repeat `scan-failed` frame is
suppressed while the message is unchanged, and the only thing that clears it is
the last subscriber leaving (`subscribe`'s release, `internal/web/events.go`).

A board that reconnects overlaps its own teardown. The new stream is a new
request, and the old request's release runs on the server when its context is
cancelled — after the new one has already subscribed. The map is therefore
never empty between the two, `lastErr` is never cleared, and the standing
failure is never reported again. The board that reloaded the page shows a queue
it cannot read and says nothing about it.

That is the scenario the test's own comment names: "someone who reloads the
page after the directory broke has been told nothing."

## Evidence

The race is what the test catches, and a delay closes it:

    // after closeFirst(), before the second stream opens
    time.Sleep(300 * time.Millisecond)

With that line the test passes 3 of 3 runs. Without it, it fails 5 of 5, always
at the second board's `waitForEvents` — the first board hears the failure, so
the scan and the broadcast are both fine. Do **not** take the sleep as the fix:
it makes the test agree with the bug by waiting for a teardown a browser does
not wait for.

## Suggested direction

Make "has been told" a property of the subscriber rather than of the hub. Two
shapes, either of which the test would accept:

- Carry the standing failure in the frames a stream opens with, beside the
  tasks and config fingerprints. A fresh board then learns the state of the
  queue on connect rather than on the next change of it.
- Keep the last error per subscriber, so suppression is per board and a new one
  starts with nothing suppressed.

The first is smaller and matches `current()`, which already exists to tell a
new stream what it missed.

## Related

TQ-0093 carries the neighbouring symptom of the same root: two boards connected
at once, where the second gets the opening frames and never the error. Its
suggested clearing — on the last subscriber leaving — is what the code does
today, and this task is the case that shows it is not enough. Fixing the
per-subscriber question closes both; they can be taken in either order.

## Acceptance

- `TestAScanFailureIsReportedAgainToAFreshBoard` passes with no sleep added to
  it, repeatedly (`-count=10`).
- A board that reloads while the task directory is unreadable says so.
- `make test` is green on main.

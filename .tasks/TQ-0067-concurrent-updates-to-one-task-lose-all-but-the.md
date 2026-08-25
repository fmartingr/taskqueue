---
id: TQ-0067
title: Concurrent updates to one task lose all but the last write
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T18:04:35+02:00
updated: 2026-08-25T18:04:35+02:00
---

## Finding

`Store.Update` takes a whole task and writes it. Every caller therefore does
read-modify-write: `Get`, change a field, `Update`. Nothing serialises that, so
two callers who read the same version both write their own copy and the later
one wins entirely.

`tq note` is the sharpest case, because a note is an append: losing one loses
information nobody can reconstruct.

## How it fails

Verified with a probe against a single store, ten goroutines appending a note
to the same task:

    notes kept: 1 of 10

Reachable from the board and the API as well as the CLI, since both surfaces
call the same store. Two agents working one task — which the queue exists to
support — is exactly the case.

## Suggested fix

Serialise read-modify-write inside the store rather than asking callers to do
it. A mutex like the one TQ-0008 added for ID allocation covers a single
process; `AppendNote` could also take the note text and do the read, append and
write itself, so the round trip is not open to callers at all.

Cross-process races remain the documented limitation.

Distinct from TQ-0008, which was ID allocation: this survives that fix.
Found while implementing it.

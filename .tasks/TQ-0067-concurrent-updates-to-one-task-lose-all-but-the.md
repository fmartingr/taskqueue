---
id: TQ-0067
title: Concurrent updates to one task lose all but the last write
status: done
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T18:04:35+02:00
updated: 2026-08-25T18:16:08+02:00
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

---

## Notes

- 2026-08-25T18:16:07+02:00 — Reproduced both losses first: ten concurrent notes on one task kept 1, and ten concurrent label adds kept 1.
- 2026-08-25T18:16:08+02:00 — Fixed by closing the round trip inside the store, as the ticket suggested. Mutate holds the lock across the read, the change and the write; Note and Patch are built on it. The CLI and the HTTP handlers now call those instead of doing Get, change, Update themselves, so the racy sequence is no longer reachable from either surface.
- 2026-08-25T18:16:08+02:00 — Update keeps its old shape and now takes the lock, but it stays last-write-wins by nature: the caller already read the task outside it. Its doc comment says so and points at Mutate.
- 2026-08-25T18:16:08+02:00 — Verified against a live tq serve: ten concurrent note requests kept all ten, where the probe that opened this ticket kept one. Mutation-checked by removing the lock from Mutate, which loses notes again.
- 2026-08-25T18:16:08+02:00 — Scope note: this covers one process, which is the server and anything embedding the store. Ten separate tq processes racing still lose, and that stays the documented cross-process limitation carried over from TQ-0008.

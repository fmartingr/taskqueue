---
id: TQ-0008
title: Concurrent creates hand out the same task ID
status: done
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T18:04:36+02:00
---

## Finding

Store.Create allocates IDs by scanning the directory (`id, err := s.NextID()`) and only writes later, with no lock, no O_EXCL and no mutex on Store — and the HTTP server hands that same *Store to net/http, so concurrent handlers inside a single process collide.

Source: `store.go:289`

## How it fails

Reproduced against a live `tq serve`: 20 concurrent `POST /api/tasks` all returned 201 Created but handed out only TWO distinct ids — 14 files named TQ-0001-*.md and 6 named TQ-0002-*.md. Interleaving: goroutine A ReadDir -> highest=0 -> TQ-0001; goroutine B ReadDir (A has not written yet) -> also TQ-0001; both os.Rename into place, and because the slugs differ neither overwrites. Store.locate (store.go:212-223) then hard-fails: `tq show/move/update/note TQ-0001` exit 1 with `TQ-0001 is claimed by 14 files`, and GET/PATCH/notes on that ID return HTTP 500 forever, since NextID has moved past the collision. If two racers pick the same title the filenames match and the second rename silently overwrites the first — one task vanishes with a 201 receipt. The comment at store.go:359-361 excuses this as a two-process PoC limit; it needs no second process, and the outcome is permanent unreachability, not "two tasks with one ID".

## Suggested fix

Guard allocation with a mutex on Store and write the new file with O_CREATE|O_EXCL, retrying with the next ID on collision. Cross-process races stay a documented limitation; in-process ones are ours.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T18:04:35+02:00 — Reproduced first with a test: 20 concurrent creates handed out 2 distinct ids, and 10 racers sharing a title left 1 task on disk. Both match the ticket.
- 2026-08-25T18:04:36+02:00 — Two parts, both needed. A mutex on Store makes allocation and the claim one step, which is what gives distinct ids — removing it alone drops 20 creates back to 1 id. writeNew links the staged file into place instead of renaming, so a name already taken fails rather than replacing another task, and Create retries with the next number.
- 2026-08-25T18:04:36+02:00 — Verified against the ticket's own reproduction, a live tq serve with 20 concurrent POSTs: 20 files, 20 distinct ids, 0 unreachable tasks. The whole suite passes under -race.
- 2026-08-25T18:04:36+02:00 — Cross-process allocation stays a documented limitation, as the ticket allows. The link makes a losing racer fail instead of overwriting, which is the part worth having without a lock file.
- 2026-08-25T18:04:36+02:00 — Found while working this: Update has a read-modify-write race of its own, and concurrent notes keep 1 of 10. Filed as TQ-0067, since it survives this fix.

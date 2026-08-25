---
id: TQ-0009
title: Concurrent note appends overwrite each other
status: done
priority: high
labels:
  - bug
  - component/api
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T18:40:49+02:00
---

## Finding

handleAddNote is an unsynchronized read-modify-write (Get at 192, `task.Body = AppendNote(...)` at 197, Update at 198) with no version check, so concurrent note appends silently discard each other while every request returns 200 OK showing the note as saved.

Source: `server.go:197`

## How it fails

Reproduced: 20 concurrent `POST /api/tasks/TQ-0001/notes` — every request returned 200 with its note present in the JSON response — left exactly 2 notes in the file. 18 notes destroyed, no error anywhere. Interleaving: A Get reads body B0; B Get reads the same B0; A writes B0+noteA; B writes B0+noteB, overwriting A wholesale (store.write renders the entire task and renames over the destination). The identical unlocked sequence is duplicated in the CLI at cli.go:458-464, so an agent running `tq note` while a board user clicks Add note races across processes too — whichever Update lands second wins.

## Suggested fix

Take the same store mutex across the Get/AppendNote/Update sequence, so a read-modify-write cannot interleave with another handler.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T18:40:49+02:00 — Already fixed. This is the same defect as TQ-0067, which I filed while working TQ-0008 without noticing this ticket existed; commit 66dd976 moved the Get, AppendNote, Update sequence inside the store as Store.Note, guarded by the mutex, and both handleAddNote and the CLI call it.
- 2026-08-25T18:40:49+02:00 — Verified against this ticket's own reproduction rather than assumed: 20 concurrent POSTs to /api/tasks/TQ-0001/notes against a live tq serve now leave 20 notes in the file, where this ticket recorded 2. All 20 returned 200.
- 2026-08-25T18:40:49+02:00 — The cross-process half of this ticket is not fixed and cannot be by a mutex, which is all the Suggested fix here proposes. Measured: 10 separate tq note processes leave 2 of 10 notes. That is the deliberate limitation the README lists.
- 2026-08-25T18:40:49+02:00 — Sharpened that README entry instead of leaving it. Last-writer-wins understates losing an append, and it said nothing about the in-process guarantee now being real. It states both, with the numbers behind them.

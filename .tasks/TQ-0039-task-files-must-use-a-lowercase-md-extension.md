---
id: TQ-0039
title: Task files must use a lowercase .md extension, and only tq-owned lowercase paths
status: done
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T13:18:46+02:00
updated: 2026-08-27T00:02:53+02:00
---

## Decision

`.md`, lowercase, is the only extension a task file may have. A file named
`TQ-0001-fix-bug.MD` is not a task file — tq neither reads it, adopts it, nor
renames it. Folding case when matching filenames was considered and declined:
it would make the store's view of a directory depend on the filesystem's
case sensitivity, which differs between APFS, ext4 and NTFS.

The invariant is one-way and holds across the whole codebase: **every path tq
writes, renames, links or removes ends in a lowercase `.md`, and every path tq
matches must too.**

## Where it stands

The write side already satisfies the rule and needs pinning, not changing:

- `TaskFileName` (`internal/store/store.go:229`) always emits `.md`.
- `task.Slugify` (`internal/task/task.go:480`) lowercases the whole slug.
- `taskFilePattern` (`internal/store/store.go:225`) matches lowercase `.md` only.

So tq cannot itself create an uppercase-extension file. What is missing is what
happens when one arrives from outside — a Windows-origin checkout, an editor
that uppercases extensions, a hand-copied file.

## How it fails today

With `.tasks/TQ-0001-fix-bug.MD` present, on a case-insensitive filesystem:

- `List`, `locate` and `NextID` all skip it, so the queue looks emptier than it
  is and `NextID` hands out `TQ-0001` again.
- `tq add` with a colliding slug fails `could not claim a task ID after 10
  attempts` — `writeNew`'s `os.Link` is refused by an entry the store cannot
  see, and the retry loop re-derives the same ID every time. The message names
  neither the file in the way nor the reason.
- `tq add` with a non-colliding slug succeeds and writes `TQ-0001-anything.md`
  beside the invisible file. Two files now claim `TQ-0001`, silently — which is
  the input condition for TQ-0040.

The data loss this task was originally filed for is gone (see the note below);
what remains is that the rule is unstated, unpinned and fails illegibly.

## Suggested fix

- State the rule where the format is defined: `internal/guide/agents.go` (which
  generates `.tasks/AGENTS.md`) and `AGENTS.md`'s "Rules of the format".
- Make the collision legible. When a create or rename is refused by an existing
  directory entry the store does not recognise as a task file, say so and name
  it, rather than exhausting the retry loop on a message about task IDs.
- Pin the invariant with tests: that `TaskFileName` is lowercase for titles that
  are not (`"Fix BUG.MD"`), that a planted `.MD` file is not adopted by `List`,
  `locate` or `NextID`, and that the collision path reports the file rather than
  the attempt count. A case-insensitive filesystem is not required to test the
  matching half; only the `os.Link` collision needs one, so guard that case.

## Out of scope

Migrating or renaming existing `.MD` files. tq reports them; the user renames
them. Nothing in tq mutates a path it does not own.
---

---

## Notes

- 2026-08-26T17:57:55+02:00 — Revalidated on main (2d8ddfa): the reported data loss no longer reproduces.

  Verified by A/B against a binary built from 4008ad3^, which reproduces the report verbatim — with .tasks/TQ-0001-fix-bug.MD present, `tq add "Fix bug"` exits 0, replaces the file's content and leaves `tq list --json` returning []. On current HEAD the same scenario exits 1 with "could not claim a task ID after 10 attempts" and leaves TQ-0001-fix-bug.MD byte-for-byte intact.

  The fix arrived incidentally with 4008ad3 (TQ-0008): Store.writeNew (internal/store/store.go:688) claims the destination with os.Link rather than os.Rename. On a case-insensitive filesystem link() refuses an entry differing only in case exactly as it refuses an exact one — which is the "refuse to clobber" remedy this task asked for. Confirmed with a standalone probe on APFS: os.Link fails 'file exists' where os.Rename silently clobbers.

  Rejecting: the high-priority overwrite it was filed for is gone. The other half of the suggested fix was not taken and is a separate, lower-severity concern — taskFilePattern (internal/store/store.go:225) is still case-sensitive, so a .MD file stays invisible to List, locate and NextID. Residue, verified on HEAD: the queue looks emptier than it is; a colliding slug now fails with the misleading attempt-exhausted message instead of naming the file in the way; and a non-colliding slug creates a second file claiming the same ID with no warning. File that separately if it is worth fixing.
- 2026-08-26T18:06:50+02:00 — Re-scoped and reopened. The rejection above stands on its facts — the overwrite is fixed — but the residue it identified is now the task, with a decision attached: lowercase .md is the only permitted extension, and tq only ever manipulates lowercase paths. Case folding in taskFilePattern is explicitly declined, so a .MD file stays foreign; the work is to state that rule, pin it, and make the collision it causes legible instead of surfacing as 'could not claim a task ID after 10 attempts'. Priority dropped high -> normal: no data loss remains, this is correctness and diagnostics.
- 2026-08-26T23:40:51+02:00 — Plan: a name matching TQ-<n>[-slug].md case-insensitively but not exactly is a *foreign claim* - never read, never adopted, never renamed. Reported through Listing.Unreadable (TQ-0011's channel), which needs no new CLI/API/frontend plumbing, with the task file already holding that ID named when there is one. Create names the entry in the way (found by inode, since a case-insensitive filesystem will not say what it called it) instead of exhausting the ID retries; update refuses to rename over one. locate names it in the not-found error. taskFilePattern stays case-sensitive, as decided.
- 2026-08-26T23:46:08+02:00 — Implemented. claimPattern recognises a TQ-<n>[-slug].md name in any extension case; taskFilePattern stays exact, so the difference is exactly the foreign set. readTaskDir returns both lists from the one directory reading a listing already does, so nothing added a third pass.

  Reported through Listing.Unreadable rather than a fourth field: a caller does the same thing with a foreign file as with a broken one - names it and carries on - so the CLI, /api/status and the board needed no change. The reason names the task file already holding that ID when there is one, which is the second claim duplicatedIDs cannot see. The real task is NOT withheld: it parses, locate finds it, and hiding it would cost a healthy card.

  Create and update find the entry in the way by inode rather than by name, because a filesystem that answers to a spelling it did not store will not say what it called the entry. Create stops there instead of retrying an ID NextID cannot advance; update refuses to rename over it, which is the only place write could have clobbered a path tq does not own.
- 2026-08-27T00:02:53+02:00 — Review pass (effort high) found no confirmed correctness bug in scope; one low finding taken. entryInTheWay resolves the name and reads the directory a moment later, so a task file renamed into that moment - a concurrent retitle writes the new name before retiring the old - could be what the match by identity landed on, and Create would have failed hard calling a real task file 'not a task file' instead of retrying with a higher ID. The match by identity now skips anything the store reads as a task file; the exact-name match is untouched, and update's alias case (a hand-renamed file that is the same entry under another spelling) still passes through as before. Pinned with hard links rather than a race: one file under two names, the task file sorting first.

  Two findings from the same pass are elsewhere and were left alone: frontend/components/App.vue (TQ-0081, noted there) and internal/config/server.go's validateServer rejecting a zoned IPv6 host, which is unfiled.

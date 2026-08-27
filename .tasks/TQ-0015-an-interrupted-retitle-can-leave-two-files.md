---
id: TQ-0015
title: An interrupted retitle can leave two files claiming one ID
status: done
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-27T09:42:41+02:00
---

## Finding

The title-change rename is three unsynchronized steps — locate (319), write-new (333), `os.Remove(filepath.Join(s.Dir, current))` (340) — so a second concurrent writer, or a Ctrl-C, resurrects or strands the old filename and leaves two files claiming one ID, permanently bricking the task.

Source: `store.go:340`

## How it fails

Reproduced on the first attempt: `tq update TQ-0001 --title "title 1"` and `tq update TQ-0001 --assignee agent-1` run concurrently left BOTH TQ-0001-target.md and TQ-0001-title-1.md on disk. Interleaving: A and B both locate TQ-0001-target.md; A renames to TQ-0001-title-1.md and removes the old name; B renders its stale copy and renames tmp -> TQ-0001-target.md, resurrecting what A deleted, and B's `written == current` so B removes nothing. locate then returns ErrInvalidTaskFile forever — every CLI verb exits 1 and every API call 500s for that ID, recoverable only by hand. The same end state arrives with no concurrency: the CLI installs no signal handler, so Ctrl-C between the rename at 416 and the remove at 340 leaves both files. If instead the Remove merely fails (read-only checkout, chflags uchg, NFS), Update reports failure for a change that fully landed — the HTTP PATCH turns that into a 500 while the write is on disk.

## Suggested fix

Serialise locate/write/remove under the store mutex so an interrupted rename cannot strand both names.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-27T08:48:57+02:00 — Approach decided (2026-08-27): rename the file, do not write a new one and delete the old.

  The suggested fix in the Finding — 'serialise locate/write/remove under the store mutex' — does NOT work. The mutex already serialises within one process (Update/Mutate take it). The measured failure needs two INDEPENDENT processes (two terminals, or a CLI edit against a running tq serve), which share no mutex. Re-measured on HEAD 505fee2: 13 of 25 trials with two concurrent tq update calls left two files claiming one ID.

  The fix is structural, not a lock. Today update() is locate -> write(new name) -> retireOldFile(old). The resurrection comes from the WRITE: A writes title-1.md and removes target.md; B, holding a stale locate, writes its copy TO target.md, recreating the file A just deleted. Two files, and locate refuses the ID forever.

  Invert it: rename current -> new name, then write the content into the new name. os.Rename is atomic and the source vanishes, so only one file exists at any instant, and B never writes at the old name. B's rename gets ENOENT, which is B learning it lost the race — re-locate and retry.

  What this fixes:
  - concurrent update: loser gets ENOENT and retries, instead of two files
  - Ctrl-C mid-retitle: one file, new name, old content — a stale slug, which is cosmetic since the frontmatter ID identifies the task, and it self-heals on the next write
  - a failed Remove reporting failure for a write that fully landed: there is no remove step any more

  What it does NOT fix, and is not this task: two processes both reading then both writing still loses the earlier change. That is a lost update, not a brick.

  Wrinkle: os.Rename silently clobbers its destination, which is why writeNew uses os.Link. The entryInTheWay guard at store.go:949-953 already covers this and TQ-0039 hardened it to match by inode; it stays, and the rename slots in behind it.

  Also in scope: install a signal handler so Ctrl-C is handled deliberately (cheap insurance, no longer load-bearing), and stop reporting failure for a write that landed.
- 2026-08-27T09:08:37+02:00 — Measured on the branch (2026-08-27): the rename inversion alone does NOT reach 0.

  Baseline on HEAD ffc56c7, 25 trials of two concurrent tq update on one ID: 9/25 left two files.
  With update() inverted to rename-then-write exactly as decided: 6/25. Not 0.

  Why. The decided approach closes the LOSER's hole but not the WINNER's. The
  save is now two renames: claim (current -> new name) and publish (staged temp ->
  new name). The publish is still a rename onto a name, and a rename creates its
  destination when nothing is there. Interleaving that still leaves two files:

    A locate=target, B locate=target (both stage their temp, fsync)
    A claim:   rename(target -> title-1)          ok
    B claim:   rename(target -> target)           ENOENT, so B relocates to title-1
    B claim:   rename(title-1 -> target)          ok  <- title-1 is now free
    A publish: rename(tmpA -> title-1)            CREATES title-1 -> two files

  The two files' mtimes come out within a microsecond of each other: the two
  processes fsync and then race through the same two renames in lockstep, so the
  one-syscall window between claim and publish is hit about a quarter of the time.

  There is no way to close that window with os.Rename and os.Link alone. Rename
  always creates its destination; link never replaces one. 'Replace this file only
  if it is still there' is not expressible without a lock, a platform syscall
  (renameatx_np RENAME_SWAP / renameat2 RENAME_EXCHANGE), or an in-place write
  that gives up crash atomicity and lets a reader catch a torn file.
- 2026-08-27T09:34:05+02:00 — Shipped: option A, the rename inversion, with the residual accepted deliberately.

  update() is now stage -> rename(current -> new name) -> rename(temp -> new name).
  The old order (write the new name, then remove the old) is gone, and with it
  retireOldFile and the 'saved X but could not retire Y' error: there is no step
  left after the content lands, so no path reports failure for a write that landed.
  A loser's move gets ENOENT and re-locates, bounded at 10 attempts.

  What this fixes outright:
  - an interrupt mid-retitle leaves ONE file, under the new name, holding the old
    content. A stale slug is cosmetic — the frontmatter ID identifies the task —
    and the next save converges it. It used to leave two files and brick the ID.
  - a failed Remove can no longer report failure for a change that fully landed,
    and the HTTP layer can no longer 500 over a write that is on disk.
  - Ctrl-C during init/add/move/done/update/note is now held until the command
    returns (holdSignals), so a signal cannot land mid-move at all.

  What it does NOT fix, measured rather than assumed. Two concurrent saves of one
  task can still leave two files. Over 200 trials of the harness in the note above:
  62/200 before the change, 51/200 after. On any single 25-trial run the two
  overlap (3-11 before, 6-10 after) — the harness has both processes fsync and
  then race the same two renames in lockstep, so the window is hit often enough
  that 25 trials cannot separate them.

  Why the window survives. The content write is itself a rename, and os.Rename
  always creates its destination. A save can therefore land at a name a losing
  writer freed in the microsecond between this save claiming that name and
  filling it. 'Replace this file only if it is still there' is not expressible
  with os.Rename and os.Link: rename always creates, link never replaces.

  Considered and declined:
  - option B, a post-publish self-repair that folds a duplicate back onto one file:
    measured 0/25 over 150 trials, ~45 lines, no lock and no sleep. Declined by the
    user as premature optimisation. 'A rename should be able to work most of the
    time.'
  - option D, an exchange syscall (renameatx_np RENAME_SWAP on darwin, renameat2
    RENAME_EXCHANGE on linux): airtight, because an exchange requires both files to
    exist and so cannot create one. Declined for its portability cost — build-tagged
    syscall code plus a fallback for every other OS.

  The residual is mitigated, not fixed, by TQ-0040: an ID two files claim is
  detected and named on tq list, tq ready and GET /api/status, with the sentence a
  write to that ID is refused with, instead of the task going silently missing.
  Delete the copy you do not want and the task comes back.
- 2026-08-27T09:42:01+02:00 — Final numbers from the committed code, and the review.

  The 25-trial harness on the shipped binary: 7/25. Over 200 trials the shipped
  code left 44 duplicates in one round and 51 in another, against 62/200 for the
  old order — a real but modest reduction, and the single-run 25-trial figures
  overlap completely (3-11 before, 6-10 after). The harness has both processes
  fsync and then race the same two renames in lockstep, which is why 25 trials
  cannot separate them. Any test asserting zero duplicates under concurrency
  would flake, so none was written; the deterministic guarantees are pinned
  instead — the ENOENT re-locate, retry exhaustion writing nothing, the
  interrupted move converging, and TQ-0039's refusal to rename over a foreign
  entry. TQ-0040's existing tests already pin that a pair is named rather than
  silent.

  A /code-review pass at high effort confirmed two over-claims in the first draft
  of this change and both were corrected: 'a save that reports failure has written
  nothing' is false on one path — the claim rename can succeed and the content
  rename then fail, which leaves the file moved to its new name still holding the
  old content, the same residue an interrupt leaves. It now says 'has written no
  new content'. It also raised that holdSignals never re-raises, so a write
  command blocked on a stalled mount ignores Ctrl-C until SIGKILL; that is the
  accepted cost of the cheap version and is written down where it lives. The
  interrupt line is now printed only for a command that succeeded, so a failed
  command no longer prints a sentence implying a completed write.

---
id: TQ-0015
title: An interrupted retitle can leave two files claiming one ID
status: todo
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

The title-change rename is three unsynchronized steps — locate (319), write-new (333), `os.Remove(filepath.Join(s.Dir, current))` (340) — so a second concurrent writer, or a Ctrl-C, resurrects or strands the old filename and leaves two files claiming one ID, permanently bricking the task.

Source: `store.go:340`

## How it fails

Reproduced on the first attempt: `tq update TQ-0001 --title "title 1"` and `tq update TQ-0001 --assignee agent-1` run concurrently left BOTH TQ-0001-target.md and TQ-0001-title-1.md on disk. Interleaving: A and B both locate TQ-0001-target.md; A renames to TQ-0001-title-1.md and removes the old name; B renders its stale copy and renames tmp -> TQ-0001-target.md, resurrecting what A deleted, and B's `written == current` so B removes nothing. locate then returns ErrInvalidTaskFile forever — every CLI verb exits 1 and every API call 500s for that ID, recoverable only by hand. The same end state arrives with no concurrency: the CLI installs no signal handler, so Ctrl-C between the rename at 416 and the remove at 340 leaves both files. If instead the Remove merely fails (read-only checkout, chflags uchg, NFS), Update reports failure for a change that fully landed — the HTTP PATCH turns that into a 500 while the write is on disk.

## Suggested fix

Serialise locate/write/remove under the store mutex so an interrupted rename cannot strand both names.

Filed from a `/code-review` pass at max effort.

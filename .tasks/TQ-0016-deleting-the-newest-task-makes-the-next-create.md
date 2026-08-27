---
id: TQ-0016
title: Deleting the newest task makes the next create recycle its ID
status: done
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-27T10:21:50+02:00
---

## Finding

NextID derives the next ID from the highest ID currently on disk (`return fmt.Sprintf("TQ-%04d", highest+1), nil`), so removing the highest-numbered task makes the next create recycle its ID and silently re-bind every dangling depends_on to an unrelated new task.

Source: `store.go:379`

## How it fails

I reproduced it. TQ-0001 "Real task one" depends on TQ-0002 "Real task two"; the second task's file is removed (an `rm`, a git revert, a branch merge — these are plain Markdown files in Git by design). `tq add "Buy milk" --status done` is then handed TQ-0002 again, and `tq show TQ-0001` prints `Depends on: TQ-0002 (done)` while `tq ready` now lists TQ-0001 as available work. An agent picks up a task whose real prerequisite was never completed, and no warning is printed at any step. Note there is no `tq delete` command and no DELETE route — Store.Delete (store.go:348) has no caller outside store_test.go — so removal is always a raw file operation, which is exactly the path that trips this.

## Suggested fix

Either document it, or refuse to reuse an ID that any task still lists in depends_on. A high-water mark would need state, which the architecture rules out.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-27T10:03:31+02:00 — NextID now takes the number one past the highest a task file claims and then advances past any number a task still lists in depends_on. A skip, not a counter: a high-water mark is the index file the architecture rules out, and two branches would each bump it and merge back to the same number. Recycling a number nothing references is left alone — no stale pointer, nothing to re-bind.

  Accepted cost: seeing depends_on means reading the files and not just their names, so a create now costs a pass over the whole queue. Paid where it is cheapest — creates are rare, and every listing already reads every file. The reading reuses the listing machinery: scan's read loop is split into readTasks, which NextID calls directly. Directly rather than through scan, because scan fires the duringScan test hook and a test that files a task from inside that window would re-enter its own hook.

  Matching is on the whole ID string, the way IndexTasks keys dependencies. A 'TQ-2' names no task tq would ever write, so it reserves nothing; only the exact spelling NextID returns can re-bind.
- 2026-08-27T10:03:41+02:00 — What a skipped file can hide, and what was concluded.

  Duplicated: covered. NextID reads through readTasks, which is scan's loop before List withholds anything, so both files of a doubled ID are read and the dependencies in them reserve their numbers. Withholding is about which of two files a reader means; it says nothing about whether the pointer inside them could re-bind.

  Unreadable: not covered, and accepted. A file that will not parse keeps its depends_on to itself, so a reference inside one reserves nothing. It cannot spring the trap either: such a file is withheld from every listing and reported instead, tq ready never offers it and tq show refuses it, so nobody is handed it as work until it is repaired — and repairing it is when its dependencies are read for the first time. Closing the hole would mean guessing at the dependencies of a file that failed to parse, which would also reserve numbers merely mentioned in a body. Asserted in TestAReferenceInsideAnUnreadableFileReservesNothing so it stays a decision.

  Foreign (.MD) claim: left exactly as TQ-0039 put it. Such a file is not adopted and its number is not taken, so NextID still hands it out and Create reports the entry in the way rather than stepping past it in silence. The harmful half is covered anyway: a foreign-claimed number a readable task depends on is in the referenced set and is skipped like any other.
- 2026-08-27T10:21:28+02:00 — Code review at high effort found two things in this diff, both fixed here.

  1. NextID reading through readTaskDir quietly changed which entries feed the highest number: readTaskDir skips directories and the old loop did not, so a directory answering to a task file's name stopped raising it. That hands a create a number whose name it can never link, and the retry derives the same number ten times over and blames task IDs for an entry it never mentions — the TQ-0039 failure, reintroduced. NextID now does its own single os.ReadDir and splits it two ways: every entry claiming a task file's name counts toward the number, and only the ones that are not directories are opened. TestAnEntryOccupyingATaskFileNameStillTakesItsNumber pins it.

  2. The note on unreadable files claimed the damage could not happen through one. It can, in one order: the merge that leaves the conflict markers is also the merge that removed the task the dependency named, the create takes the freed number while the file will not parse, and the dependency binds to it when the conflict is resolved. The doc and the test comment now say that rather than implying repair always comes first.

  Findings outside this diff were left alone and reported: internal/web/events.go subscribe TOCTOU, frontend Composer/App/FilterBar/CreateDialog, internal/cli/cli.go relative-path prefix check, and the guide's stale ready rule (all on the deferred list or in files this task does not touch).

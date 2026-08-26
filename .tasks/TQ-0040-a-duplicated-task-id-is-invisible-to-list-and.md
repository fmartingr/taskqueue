---
id: TQ-0040
title: A duplicated task ID is invisible to list and the board
status: done
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T13:18:46+02:00
updated: 2026-08-26T23:34:12+02:00
---

## Finding

`Store.List` does not go through `locate`: it reads every entry independently, so two files carrying the same `id` both parse and List returns two tasks with that ID and a nil error.

## How it fails

`tq list` and `GET /api/tasks` return both copies; `IndexTasks` (task.go) and the board's `indexTasks` collapse them to whichever comes last in ReadDir order, so the board renders two cards sharing one dataset id and clicking either gets a 500 `invalid_task_file` from PATCH. A stale `status: todo` duplicate can also mask a `done` one in `tq ready`. Nothing reports the broken invariant until someone touches that ID.

## Suggested fix

Have `List` detect IDs claimed by more than one file and report them the way `locate` already does, rather than silently returning duplicates.

Found by the /code-review pass on TQ-0006.

---

## Notes

- 2026-08-26T23:19:25+02:00 — The pair was already visible, just mislabeled. TQ-0012 had made List notice a repeated ID — a retitle writes the new file before retiring the old, so an instant of two files for one task is real — but it could not tell that instant from a queue that simply has two files for one ID, so it called both 'the task directory kept changing while it was read' and burned all three attempts chasing a race that was not happening.

  What tells them apart is looking again: the pair is redone once, and a pair still there after a pass the directory held completely still for is not a retitle in flight. That one is reported and the retries stop, so a persistent duplicate costs 2 passes rather than 3 and Incomplete stays false — it is not an inconsistent scan, it is a queue to fix.

  Both copies are withheld from the listing. An ID appears in a listing once or not at all, which is what lets every caller index by it: two cards on one dataset key, and a stale todo copy masking a done one in tq ready, were both that invariant breaking.

  Detection is on the file names, not the tasks that parsed, so it agrees with locate: a second file for an ID stops that task being addressable whether or not it parses. The sentence is locate's own, factored into duplicateClaim and reported through Listing.Duplicated — the CLI prefixes it with 'not listed:', /api/status carries it as duplicated, and the board toasts it and counts it in the footer, the way unreadable and incomplete already travel.

  locate now filters taskFileNames instead of keeping its own ReadDir loop, which is where the two surfaces had drifted apart in the first place.
- 2026-08-26T23:34:03+02:00 — Code review (high) found two real bugs in the first cut, both now fixed and both covered by TestAPairOnlySeenWhileTheDirectoryMovedIsNotCondemned, which fails against the first cut.

  1. The evidence for 'this pair is permanent' was being taken from a pass the directory moved during — exactly what a retitle in flight looks like — so a retitle nobody finished in time could be reported as two files to choose between, and the task vanished from every surface while nothing was wrong. Only a pass at rest counts as evidence now: the pair has to be found by two of them.

  2. On the exhausted path the pair was published even though no pass at rest had ever confirmed it. It is withheld there all the same — an ID is in a listing once or not at all — but what the listing says about it is Incomplete, which is true, rather than 'keep the one you want', which would be telling someone to delete a file.

  Three findings were out of scope and left alone: the events.go subscribe/stop race and the guide's stale 'ready' rule (internal/guide/agents.go, still the pre-columns wording, which contradicts the line four lines above it), plus a stale comment in FilterBar.vue about status being spelled out by the board.

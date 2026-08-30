---
id: TQ-0071
title: Allow retrieving tasks by number only
status: done
priority: normal
created: 2026-08-25T18:42:34+02:00
updated: 2026-08-30T21:36:24+02:00
---

Right now you need to put `TQ-0028` if you want to see that tasks, but it's easier if you use `tq show 28` directly.

---

## Notes

- 2026-08-30T21:36:24+02:00 — Implemented as task.NormalizeID plus task.FormatID, called only at the CLI boundary (taskID/taskIDs in internal/cli).

  - 28, 0028, tq-28 and TQ-0028 all mean TQ-0028; the padding is FormatID's, so every number NextID hands out is one the shorthand reaches (a test checks the two against each other).
  - A string already shaped like an ID is returned untouched: TQ-28 is a valid ID somebody could have hand-written into frontmatter, and re-padding it would send the lookup to a file that is not there.
  - The number is padded, never parsed, so a number too big for an int names a task that does not exist rather than wrapping into one that does.
  - It applies to every ID argument (show, move, done, update, note) and to --depends-on, --add-dependency and --remove-dependency, which are expanded before they are written; the store, the API and the board still deal only in whole IDs.
  - Docs: usage text, README, the guide template (so tq init regenerates .tasks/AGENTS.md) and the AGENTS.md rule.

  make test: internal/web's TestAScanFailureIsReportedAgainToAFreshBoard fails, and it fails the same way on a clean checkout of main — unrelated to this change. Everything else passes, plus make test-integration and make build.

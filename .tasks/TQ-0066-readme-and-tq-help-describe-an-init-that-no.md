---
id: TQ-0066
title: README and tq help describe an init that no longer exists
status: todo
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-25T17:45:33+02:00
updated: 2026-08-25T17:45:33+02:00
---

## Finding

`tq init` changed twice — it discovers a queue instead of only creating one
(TQ-0047), and it refuses to write a guide outside its own tree (TQ-0056) —
and no user-facing documentation followed.

Worse than lag, one statement is now false. The README says the search "stops
at that same repository root, so a queue in a parent directory ... cannot
capture a project that has none of its own". That now covers init, and it is
untrue for a project without a repository root, which is exactly where init
adopts an arbitrary ancestor queue (TQ-0057).

Also stale: the README's description of init as creating the directory, its
command table, and the built-in usage text, none of which mention adoption. The
CLI's Environment section omits `TQ_WALK_FOREVER`, which now redirects init as
well.

## Suggested fix

Describe what init does: find an existing queue, create one only when there is
none, and leave a guide alone when the queue belongs to another tree. Correct
the false sentence about the bound. Add `TQ_WALK_FOREVER` to the Environment
section.

Distinct from TQ-0054, which is about the notes format.

Found by `/code-review` over 20b06d2.

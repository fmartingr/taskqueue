---
id: TQ-0086
title: Reconcile AGENTS.md with the mandatory tq init
status: done
priority: normal
labels:
  - docs
  - chore
created: 2026-08-27T11:18:59+02:00
updated: 2026-08-27T12:03:20+02:00
---

`AGENTS.md` still states "The task directory is created on demand by any command that needs it, at the root of the enclosing Git repository (or TQ_DIR). Commands must not fail merely because a project has not been initialised."

`README.md` has said the opposite since dfec6b4: `tq init` is mandatory and other commands need the marker to exist.

TQ-0085 makes the code follow the README. This is the doc sweep for anything TQ-0085 does not itself touch — fold it in there if it lands first.

---

## Notes

- 2026-08-27T12:03:20+02:00 — Done by TQ-0085 (cae237f). AGENTS.md's on-demand-creation rule is replaced by the two discovery rules plus 'only tq init creates', with exit code 3 restated as 'no task queue found'. README.md, internal/guide/agents.go, the regenerated .tasks/AGENTS.md and tq help all say the same. The contradiction with dfec6b4 is gone.

---
id: TQ-0094
title: Two branches allocate the same task ID and git merges both cleanly
status: inbox
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-28T12:58:46+02:00
updated: 2026-08-28T12:58:46+02:00
---

## Finding

`NextID` scans only the local task directory for the highest number, so two
branches allocate the same ID independently, and **git merges both cleanly**
because the filenames differ.

    branch A: 84 tasks -> TQ-0085-add-auth.md
    branch B: 84 tasks -> TQ-0085-fix-cache.md
    git merge: both files land, no conflict, no warning

Two agents working in parallel is the workflow this tool exists for, and it
needs no crash, no race and no mistake to produce a duplicate.

## Why the existing defences do not reach it

- `Store.Create` already reserves an ID atomically (`os.Link` + retry,
  `internal/store/store.go:566-619`, TQ-0008). That is correct and closes the
  same-filesystem case — but the two files are never on one filesystem at the
  same time. The collision happens at merge, long after both reservations
  succeeded.
- TQ-0040 detects a duplicated ID on read and reports it. That is containment,
  not prevention: the queue still has to be repaired by hand.
- TQ-0015 (interrupted retitle) and TQ-0016 (ID recycling) close two other
  sources. Neither touches this one.

## What a fix would mean

Making IDs collision-proof is a **format change**, not a patch. Whatever is
chosen touches every task file, the filename convention, `tq` itself, the
generated guide and every doc that shows an ID. Candidates:

- An origin component in the ID (`TQ-0085-a3f`), so two branches cannot produce
  the same string.
- Allocation that reads something git will conflict on, so a merge fails loudly
  instead of silently accepting both.
- Keep sequential IDs and add a repair command that renumbers duplicates,
  accepting that merges produce them.

The third is the cheapest and keeps the readable IDs the project clearly values;
the first is the only one that actually prevents the collision.

## Open question

Is a human-readable sequential ID worth more than merge safety? The whole
project reads as though it is — IDs appear in filenames, commit messages, task
bodies and the guide. That is a product decision, not a bug fix, which is why
this is filed rather than fixed.

## Acceptance

Whatever is chosen: two branches that each file a task and then merge must not
produce two tasks with one ID without the user being told at merge time or
immediately after.

Identified while working TQ-0040; the merge behaviour follows from `NextID`
scanning local files only and has not been reproduced end to end with a real
merge. **Reproduce before acting.**

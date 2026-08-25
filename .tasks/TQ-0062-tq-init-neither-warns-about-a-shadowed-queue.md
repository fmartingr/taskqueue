---
id: TQ-0062
title: tq init neither warns about a shadowed queue nor refuses a stray one
status: todo
priority: normal
labels:
  - bug
  - component/cli
depends_on:
  - TQ-0029
created: 2026-08-25T17:44:51+02:00
updated: 2026-08-25T18:49:56+02:00
---

## Finding

Two related gaps in what `tq init` accepts and reports, both from resolving
through `OpenStore` directly rather than through `c.store()`.

**No shadow warning.** Every other command goes through `c.store()`, which
prints the `ShadowedTaskDir` note when a queue above the repository was
excluded. `tq init` calls `OpenStore` itself and prints nothing. The reviewer
verified that `tq list` in a directory explains more than `tq init` does in the
same directory — on the one command whose job is telling you where the queue is.

**A stray queue is blessed.** `tq init` in a subdirectory holding its own or a
vendored `.tasks` now adopts that directory as the project's queue. The
reviewer verified this with `repo/.tasks` as the real queue and
`repo/vendor/foo/.tasks` as a stray: init in the vendored directory reports it
as the task directory and writes the guide into it. Before TQ-0047, init
normalised to the repository root, and no command does that now.

## Suggested fix

Route `runInit` through `c.store()` so it reports what every other command
reports. Then decide whether init should prefer the repository root over a
nearer stray directory, which is a behaviour question rather than a bug.

Found by `/code-review` over 20b06d2.

---

## Notes

- 2026-08-25T18:42:55+02:00 — 'Shadowed' is currently defined by the .git bound. After TQ-0029 it becomes 'a marker above, a stray .tasks here', which is a different and much clearer check. Re-scope once the marker exists.

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
updated: 2026-08-26T18:01:21+02:00
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
- 2026-08-26T18:01:21+02:00 — Revalidated on main (2d8ddfa): half of this is fixed, half is still live. Narrowing the task to the remaining half.

  FIXED — the stray-queue half. TQ-0029's marker (commit 574e7d3, this task's own dependency) made DiscoverTaskDir resolve through .taskqueue.yaml (internal/store/store.go:160) and explicitly refuse a directory that merely happens to be named .tasks (store.go:178, 186-188). Verified: with a stray repo/vendor/foo/.tasks, tq init there reports the project's real queue, written=null, and leaves the stray untouched. With a pre-marker queue it still normalises back, writing .taskqueue.yaml and the guide at the repository root.

  STILL PRESENT — the shadow-warning half. runInit calls store.OpenStore directly (internal/cli/cli.go:244) instead of c.st() (cli.go:660), and c.st() is the only caller of store.ShadowedTaskDir and the only place the note is printed (cli.go:668-669). Verified in a temp tree with a queue above the repository, with and without a marker on it: tq list prints both 'note: created .../repo/.tasks' and the 'is above this repository and was not used; set TQ_WALK_FOREVER=true' note, while tq init on the identical tree prints no note at all. tq list still explains more than tq init does.

  Remaining fix is the one suggested: route runInit through c.st(). Worth doing alongside the re-scope this task's own note anticipated — ShadowedTaskDir (internal/store/store.go:129) still defines 'shadowed' by a bare .tasks above the repository, which the marker made obsolete. No test asserts init's shadow behaviour; ShadowedTaskDir appears in no test under internal/cli or internal/integration.

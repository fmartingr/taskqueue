---
id: TQ-0059
title: repositoryRoot ignores a .git file, so submodules and worktrees fork the queue
status: todo
priority: high
labels:
  - bug
  - component/store
depends_on:
  - TQ-0029
created: 2026-08-25T17:44:26+02:00
updated: 2026-08-25T18:49:56+02:00
---

## Finding

`repositoryRoot` accepts any `.git` it can stat, but a submodule and a linked
worktree carry `.git` as a *file* holding `gitdir: ...`, not a directory. The
review reports that the bound then behaves differently there.

## How it fails

Reported by `/code-review`, verified by the reviewer with the built binary,
not re-verified here: with `super/.git` and `super/.tasks` holding real work,
and `super/vendor/mod/.git` as a file, `tq init` in the submodule reports
`created=true` and a task directory inside the submodule. `tq list` there shows
an empty board while the superproject holds the tasks.

Every anchor in the test suite creates `.git` as a directory, so the file shape
has no coverage. A later change requiring `IsDir` would silently unbound
discovery inside every worktree with the suite green.

## Suggested fix

Decide what a submodule should do — treat the submodule as its own root, or
look through to the superproject — then cover both `.git` shapes in the
fixtures. Confirm the reported behaviour first.

Found by `/code-review` over 20b06d2.

---

## Notes

- 2026-08-25T18:42:55+02:00 — TQ-0029 moves the source of truth to a .taskqueue.yaml marker, so discovery stops bounding on .git. What survives here is whatever still calls repositoryRoot — the creation target and where the guide is written — so re-measure the blast radius after TQ-0029 instead of hardening the .git bound as it stands.

---
id: TQ-0016
title: Deleting the newest task makes the next create recycle its ID
status: todo
priority: normal
labels:
  - review
  - store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

NextID derives the next ID from the highest ID currently on disk (`return fmt.Sprintf("TQ-%04d", highest+1), nil`), so removing the highest-numbered task makes the next create recycle its ID and silently re-bind every dangling depends_on to an unrelated new task.

Source: `store.go:379`

## How it fails

I reproduced it. TQ-0001 "Real task one" depends on TQ-0002 "Real task two"; the second task's file is removed (an `rm`, a git revert, a branch merge — these are plain Markdown files in Git by design). `tq add "Buy milk" --status done` is then handed TQ-0002 again, and `tq show TQ-0001` prints `Depends on: TQ-0002 (done)` while `tq ready` now lists TQ-0001 as available work. An agent picks up a task whose real prerequisite was never completed, and no warning is printed at any step. Note there is no `tq delete` command and no DELETE route — Store.Delete (store.go:348) has no caller outside store_test.go — so removal is always a raw file operation, which is exactly the path that trips this.

## Suggested fix

Either document it, or refuse to reuse an ID that any task still lists in depends_on. A high-water mark would need state, which the architecture rules out.

Filed from a `/code-review` pass at max effort.

---
id: TQ-0039
title: Creating a task can overwrite one whose file uses a different extension case
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T13:18:46+02:00
updated: 2026-08-25T13:18:46+02:00
---

## Finding

`taskFilePattern` is case-sensitive, so a file named `TQ-0001-fix-bug.MD` (an editor that uppercases extensions, a Windows-origin checkout) is invisible to `NextID`, `locate` and `List`. On a case-insensitive filesystem `Store.write`'s `os.Rename` then lands on that same directory entry.

## How it fails

With `.tasks/TQ-0001-fix-bug.MD` present, `tq add "Anything"` reports `Created TQ-0001` and exits 0 while overwriting the first task's content. `store.List()` afterwards returns 0 tasks with a nil error: the queue looks empty and nothing warns.

## Suggested fix

Fold case when matching task filenames, or stat the destination before the rename in `Store.write` and refuse to clobber an existing entry.

Found by the /code-review pass on TQ-0006, which fixed the same data-loss class in `Store.Update`.

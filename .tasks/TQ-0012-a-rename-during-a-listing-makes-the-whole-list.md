---
id: TQ-0012
title: A rename during a listing makes the whole list 404
status: todo
priority: high
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

Store.List is a TOCTOU — it snapshots filenames with os.ReadDir then os.ReadFiles each one (`task, err := s.readFile(name)`) — so a concurrent CLI title change that renames a file mid-listing makes readFile return ErrTaskNotFound, which writeStoreError maps to HTTP 404 for the ENTIRE task list.

Source: `store.go:243`

## How it fails

I reproduced this independently: with a 29-task queue, a running `tq serve` and a loop of `tq update TQ-00NN --title ...`, 1 of 120 `GET /api/tasks` calls returned 404 (a sibling angle measured 50/400 = 12.5% with a 120-task queue and a tighter loop). Interleaving: the handler ReadDirs and gets TQ-0050-task-number-50.md; a CLI process completes store.Update, renaming it; the handler reaches the stale name, os.ReadFile returns ENOENT, readFile:270 converts it to ErrTaskNotFound and List aborts. This fires on precisely the workflow the design brief advertises ("CLI edits are visible to a running server"). The frontend swallows it — refreshQuietly (app.ts:471) only console.errors — so the board silently freezes on stale cards; on first load the user gets an empty board. A second terminal's `tq list` exits 2 for a task that exists.

## Suggested fix

Treat a file that disappeared between ReadDir and ReadFile as skipped rather than fatal.

Filed from a `/code-review` pass at max effort.

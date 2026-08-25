---
id: TQ-0058
title: TQ_WALK_FOREVER stops tq init giving a new repository its own queue
status: todo
priority: normal
labels:
  - bug
  - component/cli
created: 2026-08-25T17:44:26+02:00
updated: 2026-08-25T18:42:55+02:00
---

## Finding

With `TQ_WALK_FOREVER=true`, discovery is unbounded by design, so `tq init`
inside a fresh repository adopts a queue outside it and creates nothing.

## How it fails

Reproduced: `outer/.tasks` exists, `outer/repo/.git` exists, the repository has
no queue. `TQ_WALK_FOREVER=true tq init` in the repository reports
`created=false` and the outer task directory.

This is consistent, not broken: `tq add` in the same directory also files into
the outer queue, which is what the variable asks for. There is a workaround —
`TQ_WALK_FOREVER= tq init` creates the repository's own queue, verified.

The gap is discoverability. Nothing tells the user how to opt out for one
command, and the multi-repository layout the variable exists for is exactly
where a new repository needs its own queue.

## Suggested fix

Say it in the output: when init adopts a queue because the bound was lifted,
name the variable and show how to unset it for one command. A flag that forces
creation would also do, but the environment variable already answers it.

Found by `/code-review` over 20b06d2; reproduced by hand, including the
workaround.

---

## Notes

- 2026-08-25T18:42:55+02:00 — TQ-0029 puts a .taskqueue.yaml marker at the repository root, so 'which queue belongs to this repository' stops being inferred from .git, and most of TQ_WALK_FOREVER's reason to exist goes with it. Re-scope after TQ-0029, or close if the escape hatch has no user left.

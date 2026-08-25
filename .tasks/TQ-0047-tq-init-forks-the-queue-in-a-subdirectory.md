---
id: TQ-0047
title: tq init forks the queue in a subdirectory instead of discovering the parent
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T13:56:09+02:00
updated: 2026-08-25T13:56:09+02:00
---

## Finding

`runInit` resolves its store with `InitStore(c.dir)` (`cli.go`), while every
other command goes through `OpenStore` -> `DiscoverTaskDir`, which walks *up*
for an existing `.tasks`. In a project that is not a Git repository the two
disagree.

## How it fails

Reproduced: `proj/.tasks` holding one task, no `.git`, and a `proj/backend/`
subdirectory.

    $ cd proj/backend && tq list
    TQ-0001  todo  normal  -  existing work      # finds the parent queue

    $ tq init                                    # from the same directory
    $ ls -d .tasks                               # a second queue appears
    $ tq list
    ID  STATUS  PRIORITY  ASSIGNEE  TITLE        # empty

The existing work is now invisible to everything run under `backend/`, and a
second root-doc pointer is written there.

TQ-0041 makes this worse by promoting `tq ready` to mandatory step 1: an agent
that follows the guide from a subdirectory queries the forked empty queue and
correctly concludes there is nothing to claim.

## Suggested fix

Have `runInit` discover an existing task directory the way every other command
does, and only create one where discovery finds nothing.

Found by `/code-review` on TQ-0041; reproduced by hand.

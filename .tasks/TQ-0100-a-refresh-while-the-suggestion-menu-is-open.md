---
id: TQ-0100
title: A refresh while the suggestion menu is open moves the highlight back to the top
status: todo
priority: normal
labels:
  - bug
  - component/frontend
created: 2026-08-29T08:24:52+02:00
updated: 2026-08-29T08:24:52+02:00
---

## Finding

The search bar's suggestion menu resets its highlighted row to the first one
whenever the task listing changes. With the board live (TQ-0084 made `busy`
just `dragging`, so a listing lands while a menu is open), a user part-way down
a list of labels is silently returned to the top, and Enter takes the wrong
value.

## Why it is newly reachable

The filter bar froze its label options while the select was focused —
`@focus="holding = true"` / `@blur="releaseLabelOptions"` — for exactly this
reason. **TQ-0098 deleted that guard along with the component.** The search
menu never had an equivalent, and its code did not change in that commit, so
nothing was broken by the removal: the protection simply no longer exists
anywhere.

Needs the listing to genuinely change while the menu is open and the highlight
is off the first row. An agent running `tq add` or `tq note` against the project
does it; so does another board.

## Suggested fix

Hold the highlight the way the filter bar held its options: while the menu is
open and the user has moved off the first row, keep the highlighted **value**
across a refresh rather than the index. If the value it named is gone from the
new suggestions, fall back to the first row — that is the honest answer, and it
is visible rather than silent.

Consider whether the suggestion list itself should be held too. The filter bar
held the whole option set; holding only the highlight is less surprising than
options appearing and disappearing under the cursor, but it means the list can
still change shape mid-keystroke.

## Acceptance

- A refresh arriving while the menu is open does not move the highlight.
- Enter still takes the row the user is looking at.
- A highlighted value that no longer exists degrades to the first row rather
  than to nothing.
- A browser test that fails without the fix — the listing has to change while
  the menu is open, which the harness can do by filing a task against the
  running project.

Found by the code review during TQ-0098, which deliberately left it out rather
than riding it on that commit. Not independently reproduced — **confirm before
fixing.**

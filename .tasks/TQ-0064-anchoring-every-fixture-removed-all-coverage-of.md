---
id: TQ-0064
title: Anchoring every fixture removed all coverage of projects without Git
status: todo
priority: high
labels:
  - tests
  - bug
depends_on:
  - TQ-0029
created: 2026-08-25T17:45:14+02:00
updated: 2026-08-25T18:49:56+02:00
---

## Finding

TQ-0023 routed every fixture root through `testRoot`, which plants a `.git`.
That fixed the escape and deleted a whole configuration from the suite, in the
same series that shipped `tq init` changes whose remaining bug lives only in
that configuration.

Verified here by mutation: break `taskDirTarget`'s fallback to `startDir`, and
break `GuidePointer`'s non-repository branch — the full suite stays green for
both.

The reviewer confirmed the same against a coverage profile, and adds that
`ShadowedTaskDir`'s `!ok` guard and `DiscoverTaskDir`'s filesystem-root
exhaustion branch are now unreachable too.

Consequences already visible:

- `TestCLIInitFindsTheQueueAbove` is named for subdirectory forking, but its
  fork assertion can never fire: with the anchor at the outer directory, the
  unfixed path lands the fork at that root instead. It cannot catch the bug it
  exists for.
- The `GuidePointer` assertion in `agents_test.go` uses `strings.HasSuffix` and
  accepts either branch, so the function's output changed inside TQ-0023 with
  no test diff. That is TQ-0061.
- No test covers `tq init` from a deep subdirectory of a fresh repository
  creating the queue at the repository root — the property TQ-0047 is about.
  The reverted attempt had one; the redo dropped it.
- `TestCLIInitDoesNotAdoptAQueueOutsideTheRepository` never asserts that the
  outside queue was created, so it can degrade into asserting nothing.

## Suggested fix

Add an unanchored fixture for projects without Git, isolated some other way —
a queue planted at the temp root stops the walk inside the fixture, which
TQ-0056's test uses. Then restore the dropped repository-root case, tighten the
loose assertions to exact strings, and assert preconditions.

Found by `/code-review` over 20b06d2; mutation-verified here.

---

## Notes

- 2026-08-25T18:42:55+02:00 — TQ-0029 makes 'no Git' stop being a special configuration: the marker anchors discovery whether or not .git exists. Recheck which fixture configurations are still worth restoring once it lands.

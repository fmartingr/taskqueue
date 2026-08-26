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
updated: 2026-08-26T18:01:21+02:00
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
- 2026-08-26T18:01:21+02:00 — Revalidated on main (2d8ddfa): still present but narrower than filed — two of the four named branches are now covered, one layer up.

  FIXED, indirectly. The integration harness stopped anchoring: newProject (internal/integration/harness_test.go:73) plants the marker rather than .git, and discovery_test.go:102,139,159 use a bare t.TempDir(). That is exactly this task's suggested-fix shape. Mutation testing confirms it kills two branches — taskDirTarget's fallback to startDir (store.go:120-123) and DiscoverTaskDir's no-repository return (store.go:208) both die under make test-integration. The dropped repository-root case is also back, as TestOpenStoreCreatesAtTheRepositoryRoot (internal/store/store_test.go:142).

  STILL PRESENT. Every fixture in the Go unit suite is still anchored — testRoot (internal/store/store_test.go:30) and tqtest.Root (internal/tqtest/tqtest.go:39) both MkdirAll(.git) — so make test alone catches none of this. Aggregate coverage across all 7 test binaries shows the no-Git branches unexecuted: store.go:120 startDir fallback 0, store.go:138 ShadowedTaskDir !ok guard 0, store.go:208 no-repository return 0, config/discovery.go:47 WalkBoundary no-repo return 0.

  Two branches still survive mutation in BOTH suites: GuidePointer's non-repository branch (internal/guide/agents.go:63) and ShadowedTaskDir's !ok guard (internal/store/store.go:138), plus the wording of the no-repository not-found message. The GuidePointer mutant is user-visible and undetected — on a no-Git project the printed pointer goes from @.tasks/AGENTS.md to @../AGENTS.md and both suites stay green, because agents_test.go:110 is still the loose strings.HasSuffix flagged by TQ-0061.

  Remaining work: a no-Git fixture in the unit suite, plus exact-string assertions on those two functions.

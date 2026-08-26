---
id: TQ-0064
title: Anchoring every fixture removed all coverage of projects without Git
status: done
priority: high
labels:
  - tests
  - bug
depends_on:
  - TQ-0029
created: 2026-08-25T17:45:14+02:00
updated: 2026-08-26T21:22:00+02:00
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
- 2026-08-26T20:55:29+02:00 — Three of the four branches this ticket names were closed elsewhere, not here.

  GuidePointer's no-repository branch: gone. TQ-0061 replaced GuidePointer with GuidePath and deleted the base selection outright, so the mutant has no line left to live on and the assertions are exact strings now.

  taskDirTarget's fallback to startDir and DiscoverTaskDir's no-repository return: covered by make test-integration since the harness stopped anchoring with .git. Reconfirmed on this branch.

  ShadowedProjectMarker (the renamed ShadowedTaskDir, TQ-0062) kept its surviving mutant: dropping the !ok guard left both suites green. It is user-visible — a project without Git warns about its own marker, because the walk then starts at the process working directory. Killed in two places now: a store unit test that chdirs into the project the way the CLI passes its working directory, and an integration subtest that drives the real binary, where the bogus note actually reaches stderr.
- 2026-08-26T20:55:40+02:00 — Every assertion added or tightened here is mutation-proven on a scratch copy:

  - ShadowedProjectMarker's no-repository guard removed -> store TestShadowedProjectMarker/a project without a repository excludes nothing, and integration TestInitNamesTheProjectTheBoundExcluded/a project without a repository excludes nothing. Both green on HEAD before.
  - The no-repository not-found message reworded -> TestDiscoverTaskDirWithoutARepositoryOrAMarker, which pins the whole string. Green on HEAD before.
  - ConfigPath stops walking up -> TestCLIInitFindsTheQueueAbove now fails on 'init forked a second queue in the subdirectory'. Against the old fixture the same mutant landed the queue at the .git anchor instead, so that assertion never fired. Dropping the anchor for a marker is what makes it reachable.
  - taskDirTarget stops preferring the repository root -> TestCLIInitCreatesAtTheRepositoryRoot, the TQ-0047 case the redo dropped. No CLI test caught this mutant before.
  - Config.TaskDir drops the marker's path -> TestCLIInitDoesNotAdoptAQueueOutsideTheRepository fails on its new precondition. Without it the test passed while there was no queue outside to refuse.

  tqtest gained RootWithoutAnchor: a temp directory with neither anchor, for the one configuration where both absences are the premise. Read-only by contract, and it fails rather than trusting the machine — it asks discovery's own two questions about what sits above.
- 2026-08-26T21:10:59+02:00 — Code review (high) raised one medium on this diff: TestCLIInitFindsTheQueueAbove asserted its no-Git premise in a comment only. tqtest.Root plants a marker but has no repository guard, so TMPDIR inside a developer's checkout would give a lost init somewhere to fall back to and the fork assertion would be unable to fail again — the same false green, moved from the fixture out to the machine. The premise is asserted now, and verified: running the package with TMPDIR pointed at a directory holding .git fails on the guard.

  Two low notes addressed as well. RootWithoutAnchor's contract is reworded to say what it is about — what the code under test may do with the directory, not what the test puts in it, since the first caller writes a marker into it. And the t.Chdir subtest says why the package stays sequential.

  One finding was outside this diff and is left alone: tq move accepts an empty status through Columns.Check, which returns nil for it by design, so the failure surfaces four layers down.
- 2026-08-26T21:22:00+02:00 — Second review round, high effort, found one HIGH this diff had missed and it was the ticket's own failure mode: the integration subtest had no premise guard. newProject is a bare t.TempDir plus a marker, so with TMPDIR inside a checkout the repository bound comes back and the subtest passes with the ShadowedProjectMarker mutant alive — reproduced, ok in 0.8s. The store-side twin failed loudly under the same TMPDIR. Guarded now by a local walk for .git in harness_test.go; internal/integration links none of tq's own code, so it cannot borrow config.RepositoryRoot.

  The premise is one fixture now rather than three spellings: tqtest.RootWithoutGit is Root plus the guard, and tqtest.RequireNoRepositoryAbove is exported so the wording lives in one place. All four guards were verified to trip together under a TMPDIR holding .git.

  Three comments were corrected: 'only a real process can show this' was false once the store test existed, RootWithGit guarantees no marker at the fixture rather than none above it, and the enclosing marker bounds the config walk only — RepositoryRoot is not bounded by a marker at all, which is why the guard is load-bearing and not belt-and-braces.

  Kept against the review's suggestion: the second precondition in TestCLIInitDoesNotAdoptAQueueOutsideTheRepository. Under the mutant that drops the marker's path, InitStore returns the project directory itself, which exists and is a directory — so the os.Stat check passes and only the path check fails. The stat's error message is split in two, since a path that exists but is not a directory left err nil and printed <nil>.

---
id: TQ-0024
title: Nothing typechecks the frontend
status: done
priority: normal
labels:
  - component/ci
  - component/frontend
  - tests
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-26T11:57:28+02:00
---

## Finding

Nothing in the repo ever typechecks the frontend — no tsconfig.json, no `tsc` script, no typescript dependency — and `bun build` strips types without checking them, so the `frontend` CI job can only catch formatting drift in public/, never a frontend regression.

Source: `package.json:5`

## How it fails

VERIFIED: appending `const probe: number = "not a number"; console.log(probe.toFixed(2));` to frontend/app.ts and running `bun run build` exits 0 and emits the broken code straight into public/app.js. CI's frontend job (ci.yml:67-68) runs exactly that build and then only diffs public/ — clean, because the committed output matches the broken source — so a TypeError that fires on page load passes every gate and ships embedded in the release binary. `make test` never touches app.ts.

## Suggested fix

Add a tsconfig and a `tsc --noEmit` step. This needs a TypeScript dev dependency, which is an explicit exception to the Bun-only rule and worth deciding on.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-26T11:14:03+02:00 — Researched while settling TQ-0076's build decisions.

  This is doable now, independently of the Vue migration, and is worth doing
  before it rather than after: tsc --noEmit --strict already passes clean on
  app.ts, board.ts and notes.ts, so the work is a tsconfig plus a CI step, not a
  round of fixing.

  Two things for whoever picks it up:

  - Pin typescript to 6.0.3, the newest JavaScript-implemented release. npm's
    'latest' is now 7.0.2, the Go-native port, which ships no programmatic API.
    Plain tsc on 7.x is fine today, but vue-tsc cannot run on it, and TQ-0076 has
    settled on SFCs — so pinning now avoids installing a version that has to be
    rolled back the moment templates arrive.
  - Once TQ-0076 lands, swap tsc for vue-tsc rather than adding a second step. It
    is a drop-in superset: one invocation checks .ts and .vue alike, and it is the
    only thing in the pipeline that can see a typo inside a template. Bun.build
    compiles and ships those without complaint, which is this ticket's finding
    extended into markup.
- 2026-08-26T11:57:28+02:00 — Done. tsconfig.json at the repository root covering frontend/ and browser/, a make typecheck target running tsc --noEmit, and a CI step ahead of the existing frontend job.

  The sources passed clean at strict:true with no code changes, so this is purely
  a gate rather than a round of fixes. Verified it earns its place the way the
  finding was written: seeding 'const probe: number = "not a number"' into
  board.ts makes typecheck fail with TS2322, while bun build exits 0 and emits the
  broken code — which is exactly what CI used to accept.

  Two dependencies, both toolchain-only and neither reachable from public/:
  typescript pinned at exactly 6.0.3, and @types/bun for the bun:test module, the
  Bun globals and the node: builtins that build.ts and the browser harness use.
  AGENTS.md's dependency rule now says the toolchain sits in the same carve-out as
  the browser driver, and says why the pin is exact.

  Deliberately not taken: noUncheckedIndexedAccess. It is a good flag and it finds
  real things — 12 sites, mostly indexed access in notes.ts — but fixing them is a
  hardening pass with its own risk, not this ticket, and bundling it would have
  turned a zero-churn gate into a code change. Worth filing separately.

  Next for this: once TQ-0076 lands, swap tsc for vue-tsc in the same make target
  rather than adding a second one.

---
id: TQ-0024
title: Nothing typechecks the frontend
status: todo
priority: normal
labels:
  - component/ci
  - component/frontend
  - tests
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

Nothing in the repo ever typechecks the frontend — no tsconfig.json, no `tsc` script, no typescript dependency — and `bun build` strips types without checking them, so the `frontend` CI job can only catch formatting drift in public/, never a frontend regression.

Source: `package.json:5`

## How it fails

VERIFIED: appending `const probe: number = "not a number"; console.log(probe.toFixed(2));` to frontend/app.ts and running `bun run build` exits 0 and emits the broken code straight into public/app.js. CI's frontend job (ci.yml:67-68) runs exactly that build and then only diffs public/ — clean, because the committed output matches the broken source — so a TypeError that fires on page load passes every gate and ships embedded in the release binary. `make test` never touches app.ts.

## Suggested fix

Add a tsconfig and a `tsc --noEmit` step. This needs a TypeScript dev dependency, which is an explicit exception to the Bun-only rule and worth deciding on.

Filed from a `/code-review` pass at max effort.

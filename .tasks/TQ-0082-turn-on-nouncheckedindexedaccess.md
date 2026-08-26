---
id: TQ-0082
title: Turn on noUncheckedIndexedAccess
status: todo
priority: low
labels:
  - tests
  - component/frontend
created: 2026-08-26T13:12:58+02:00
updated: 2026-08-26T13:12:58+02:00
---

## Finding

TQ-0024 landed the frontend typechecker at `strict: true` deliberately without
`noUncheckedIndexedAccess`, so that it was a gate with no code churn. The flag
finds 12 real sites, most of them indexed access in `frontend/notes.ts`.

## Why it was deferred rather than skipped

Fixing them is a hardening pass with its own risk of introducing a bug, and
bundling it into the ticket that added the gate would have turned a zero-change
commit into a code change nobody asked for.

## Suggested fix

Turn the flag on and work through the sites, preferring a real guard over a
`!` assertion — the point of the flag is the cases where the guard was missing,
and an assertion just tells the compiler to stop asking.

Note `tsconfig.json` also has `noUnusedLocals`/`noUnusedParameters` off, for a
different and documented reason: `bun-plugin-vue3` ships TypeScript source
rather than declarations, so `build.ts`'s import of it pulls three of its files
into the program. `frontend/build.ts` is the plugin's only importer, so
excluding that one file would let both flags back on at the cost of `build.ts`
going unchecked, or of a second config and a second vue-tsc run.

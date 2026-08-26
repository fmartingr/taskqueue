---
id: TQ-0076
title: Migrate the board to Vue 3
status: backlog
priority: normal
labels:
  - refactor
  - component/frontend
created: 2026-08-26T08:14:45+02:00
updated: 2026-08-26T08:14:59+02:00
---

## Where the frontend is today

`frontend/app.ts` is 641 lines of imperative DOM against one mutable `state`
object. Every refresh rebuilds the whole board with `replaceChildren`, guarded
by comparing a serialized copy of the last response, and any interactive state
that must survive a rebuild is kept by hand: `state.composing` and
`state.draft` exist only so the quick-add composer is not yanked away, and
polling is skipped entirely while dragging or while a dialog is open.

That pattern is the cause of three open bugs, not a coincidence: TQ-0010 (a
dialog saves a body captured when it opened), TQ-0027 (a re-render detaches the
button that was just clicked), TQ-0019 (implicit form submit). The DOM and the
data are two sources of truth reconciled by hand.

It also gets worse soon: TQ-0030, TQ-0032 and TQ-0035 make labels, severities
and columns come from the config, and `index.html` hard-codes all three as
static `<select>` options.

Already framework-agnostic and staying that way: `board.ts` (81 lines) and
`notes.ts` (174), with 448 lines of Bun unit tests, plus the `browser/` suite
that drives the real binary — the migration's safety net.

## Target

Vue 3 with TypeScript, bundled by Bun into `public/app.js` exactly as now:
committed output, `go:embed` unchanged, the CI staleness gate unchanged, no CDN
and nothing fetched at runtime.

## Three things to settle before starting

1. **Bun does not compile `.vue` SFCs on its own.** Either add a Bun bundler
   plugin around `@vue/compiler-sfc`, or skip SFCs and write components with
   `defineComponent` and render functions. SFCs are most of why people reach for
   Vue; the plugin is the smaller compromise. Decide first — everything below is
   the same either way.
2. **Vue becomes the first runtime dependency.** `bun.lock` gets committed,
   `node_modules` stays ignored, and `public/app.js` goes from ~20 KB to roughly
   120 KB raw, which ships inside the binary. `playwright-core` already made
   this a devDependency-shaped repository, so this is a smaller step than it
   would have been.
3. **Typechecking arrives with it** (TQ-0024): `vue-tsc` for SFC templates,
   plain `tsc` otherwise.

## Order

1. Add Vue and the build plugin. `app.ts` keeps building until the last step.
2. Shell: `App` -> `FilterBar`, `Board` -> `Column` -> `Card`, reusing
   `board.ts` for filtering and readiness.
3. Dialogs: `TaskDialog` (notes panel on `notes.ts`) and `CreateDialog`.
4. Drag-and-drop and the quick-add composer — the two places the current code
   fights re-renders. Both become local component state.
5. Delete `app.ts`, switch `build.ts`'s entrypoint, rebuild `public/`, commit.

Keep `style.css` as it is at first; splitting it into scoped component styles is
a separate change and would hide regressions inside this one.

## Acceptance criteria

- Parity, checked in a browser: four columns, drag between them, quick add
  (Enter files and stays open, blur files, empty discards, Escape cancels),
  task dialog with the separate notes panel and per-note editing, create dialog,
  filters including "ready only", blocked indicator, note counts, polling,
  toasts, the footer status line.
- The `browser/` suite passes unchanged. It drives the real binary, so it should
  not need to know the page was rewritten; if a test has to change, that is a
  behaviour change and needs saying out loud.
- TQ-0010, TQ-0019 and TQ-0027 are fixed **deliberately**, each with a
  regression test — not assumed fixed because the framework re-renders
  differently.
- `make frontend` output is deterministic and committed; `make build`, `DEV=1`
  serving and the CI staleness gate all behave as before.
- `bun test frontend/` still passes: `board.ts` and `notes.ts` keep their tests.

## Sequencing

Best done **before** TQ-0030, TQ-0032 and TQ-0035, which turn labels,
severities and columns into config-driven UI. Doing those first means building
that UI imperatively and then porting it.

---

## Notes

- 2026-08-26T08:14:59+02:00 — TQ-0068 (global search bar), TQ-0069 (Trello-like modal) and TQ-0070 (body editor on click) are all UI work sitting in the same backlog. Each one built against app.ts is work that gets ported again; each one built after this lands is a component. Worth deciding the order for the four together.

---
id: TQ-0076
title: Migrate the board to Vue 3
status: done
priority: normal
labels:
  - refactor
  - component/frontend
created: 2026-08-26T08:14:45+02:00
updated: 2026-08-26T13:13:31+02:00
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

Vue 3 with TypeScript, bundled by Bun into `internal/web/public/app.js`
exactly as now: committed output, `go:embed` unchanged, the CI staleness gate
unchanged, no CDN and nothing fetched at runtime.

## Settled

Measured on Bun 1.3.14 / Vue 3.5.41 / `bun-plugin-vue3` 1.1.0, in a throwaway
project rather than argued from estimates.

1. **SFCs, via `bun-plugin-vue3`.** Bun does not compile `.vue` on its own, and
   this is the maintained plugin: MIT, 412 lines, peer-depending only on `vue`,
   so it uses the `@vue/compiler-sfc` that already ships with it. Output is
   byte-identical across repeated builds, scoped-style hash included, which is
   the determinism this ticket already asks for. Two things come with it:

   - It needs `typescript` installed to compile a `<script setup lang="ts">`
     block, so the dependency TQ-0024 wants to debate is forced by this choice
     rather than optional. Pin it to 6.x — see the note on typechecking below.
   - A `<style>` block in an SFC emits a CSS asset, which collides with
     `build.ts`'s `naming: "[dir]/[name].js"` and aborts the build. Keeping
     styles in `style.css` avoids it entirely, which the Order below already
     wanted for unrelated reasons. That preference is now a constraint.

2. **Vue becomes the first runtime dependency, unminified, with its feature
   flags defined.** `bun.lock` gets committed and `node_modules` stays ignored.

   Measured, for the same two-component app: 176.9 KB raw at today's
   `minify: false`, 156.4 KB with the flags defined, 59.2 KB minified. The Vue
   floor alone is 145.2 KB, so application code is rounding error against it —
   which is also why render functions would have saved only 7.7 KB and were
   never really a size decision.

   Defining `__VUE_OPTIONS_API__`, `__VUE_PROD_DEVTOOLS__` and
   `__VUE_PROD_HYDRATION_MISMATCH_DETAILS__` as false is 20 KB for three lines
   of config; the Options API is dead code here. Minification is deliberately
   **not** taken: `public/app.js` is committed and reviewed, and 59 KB on one
   unreadable line is a worse artifact than 156 KB that still diffs. The binary
   is not the constraint — `tq` is 9.4 MB, so this moves `app.js` from 0.27% of
   it to 1.6%.

3. **Typechecking, on TypeScript pinned to 6.0.3.** `tsc --noEmit --strict`
   already passes on today's `app.ts`, `board.ts` and `notes.ts`, so TQ-0024 is
   a `tsconfig.json`, a pinned dependency and a CI step — no code to fix. Once
   SFCs exist, `vue-tsc` replaces `tsc` in that step rather than joining it: it
   is a drop-in superset that checks `.ts` and `.vue` in one run, and it is the
   only thing in the pipeline that can see a `{{ props.titel }}` typo inside a
   template. `Bun.build` compiles and ships those without complaint.

   The pin is the whole trick. TypeScript 7 is the Go-native port and ships no
   programmatic API, so every Volar-based checker still needs the
   JavaScript-implemented line; `vue-tsc` resolves `typescript/lib/tsc`, which
   7.x no longer exports, and dies with `ERR_PACKAGE_PATH_NOT_EXPORTED`. Nothing
   warns about it — its peer range is `>=5.0.0`, and `bun add -d typescript`
   resolves `latest`, which is 7.0.2. So pin **6.0.3**, the newest
   JavaScript-implemented release. Verified: `vue-tsc` 3.3.11 on 6.0.3 reports a
   template error and a plain `.ts` error from one invocation, and exits 0 clean.

   Temporary, and not a Vue quirk — Svelte, Astro and MDX are all stuck on the
   same line. Upstream is building content mappers so `tsc` can check `.vue`
   directly, targeted at TypeScript 7.1, and Vue's language-tools maintainer has
   said `vue-tsc` is deprecated once that lands. Lift the pin then.

Hot reload is **out of scope**. Restarting the server and rebuilding the
frontend by hand stays the dev loop for now.

## Order

1. Add Vue and `bun-plugin-vue3`, with the feature flags defined in
   `build.ts`. `app.ts` keeps building until the last step.
2. Shell: `App` -> `FilterBar`, `Board` -> `Column` -> `Card`, reusing
   `board.ts` for filtering and readiness.
3. Dialogs: `TaskDialog` (notes panel on `notes.ts`) and `CreateDialog`.
4. Drag-and-drop and the quick-add composer — the two places the current code
   fights re-renders. Both become local component state.
5. Delete `app.ts`, switch `build.ts`'s entrypoint, rebuild `public/`, commit.

Keep `style.css` as it is: splitting it into scoped component styles is a
separate change, it would hide regressions inside this one, and SFC `<style>`
blocks collide with the build's output naming as noted above.

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
- `make frontend` output is deterministic and committed to
  `internal/web/public/`; `make build`, `DEV=1`
  serving and the CI staleness gate all behave as before.
- `public/app.js` stays unminified with the three Vue feature flags defined, and
  carries no dev-only code.
- `bun test frontend/` still passes: `board.ts` and `notes.ts` keep their tests.

## Sequencing

Two of the three tickets this was meant to precede have already shipped against
`app.ts`: TQ-0030 (labels) and TQ-0032 (priorities) both built their
config-driven UI imperatively, and that UI is now work this migration will port.
The argument held; the timing did not.

TQ-0035 (columns from config) is the one still ahead, and the same reasoning
applies to it: built first it is imperative `<select>` wiring that gets ported,
built after it is a component. That ordering is still open — TQ-0035 is not
blocked on this ticket, because most of it is config, store and CLI.

TQ-0068, TQ-0069 and TQ-0070 are settled the other way: all three now depend on
this ticket, so they land after it as components rather than being written twice.

TQ-0010, TQ-0019 and TQ-0027 also depend on this ticket, since they are the
three bugs its acceptance criteria claim.

---

## Notes

- 2026-08-26T08:14:59+02:00 — TQ-0068 (global search bar), TQ-0069 (Trello-like modal) and TQ-0070 (body editor on click) are all UI work sitting in the same backlog. Each one built against app.ts is work that gets ported again; each one built after this lands is a component. Worth deciding the order for the four together.
- 2026-08-26T10:35:19+02:00 — Added hot reload as decision 4 and an acceptance criterion. The dev loop today rebuilds on save but the browser is reloaded by hand, and every reload discards the state that made the bug reproducible, so it is worth settling with the rest.

  It is a decide-first item rather than a detail because it is entangled with decision 1: Vue's component-level HMR is emitted by the SFC compiler, so whichever plugin compiles .vue files has to emit the accept block too. Live reload is the fallback that costs almost nothing and needs no bundler change; Vite-for-dev-only is the option that works out of the box but puts a second bundler in the loop, which AGENTS.md only permits for things that never reach the shipped public/ — a call to make out loud.
- 2026-08-26T10:57:19+02:00 — Decisions 1 and 2 settled, and hot reload dropped.

  1. SFCs via bun-plugin-vue3. Size was never the differentiator between SFCs and
     render functions — 7.7 KB raw between them, against a 145 KB Vue floor — so
     it came down to templates being the reason to pick Vue at all. Two things
     the research turned up are now written into the ticket as constraints rather
     than discovered later: the plugin needs typescript installed to compile a
     lang="ts" block, and an SFC <style> block emits a CSS asset that collides
     with build.ts's naming and aborts the build.
  2. Unminified with the Vue feature flags defined. 156 KB rather than the 177 KB
     the current settings would give or the 59 KB minification would. The binary
     is not the constraint (tq is 9.4 MB); the committed, reviewed public/app.js
     is, and 59 KB on one unreadable line is a worse artifact than 156 KB that
     still diffs.

  Hot reload is out of scope: server and frontend get restarted by hand for now.
  Removed from the body rather than left as an open question, so it is not
  re-litigated by whoever picks this up. Recording why it was cheap to drop: the
  option worth wanting does not exist — Bun ships React Fast Refresh and no Vue
  equivalent, bun-plugin-vue3 contains no HMR code at all, and a dev-mode build
  registers no component with Vue's HMR runtime. So the realistic choices were
  live reload (a page refresh the developer did not have to trigger) or a second
  bundler, neither of which is worth blocking the migration on.

  Typechecking is the one still open, and belongs with TQ-0024: vue-tsc 3.3.11
  does not run on TypeScript 7, which is what bun add -d typescript installs
  today, so whatever lands has to pin 5.x.
- 2026-08-26T11:14:03+02:00 — Correcting the pin I wrote earlier: TypeScript 6.0.3, not 5.x.

  TypeScript 6.0.3 is the newest JavaScript-implemented release; 7.0.2 is the
  Go-native port and is what npm's 'latest' now points at. vue-tsc needs the JS
  line, so 6.x is the right pin rather than 5.x — same fix, newer floor. Verified
  vue-tsc 3.3.11 on 6.0.3 catching both a template error and a plain .ts error in
  one run.

  Worth knowing why this is a pin and not a workaround: TS 7 ships no programmatic
  API, so every Volar-based checker is stuck on the JS line — Svelte, Astro and
  MDX included. Upstream is building content mappers (microsoft/typescript-go)
  that would let tsc check .vue directly, targeted at TS 7.1, and Vue's
  language-tools maintainer has said vue-tsc gets deprecated once that lands.

  The trap worth repeating: vue-tsc's peer range is typescript >=5.0.0, so
  installing 7.x raises no warning at all. It installs clean and then dies at
  runtime with ERR_PACKAGE_PATH_NOT_EXPORTED.
- 2026-08-26T11:20:21+02:00 — All four decisions are now closed in the body; nothing technical is blocking a start.

  Typechecking folded into Settled as item 3 rather than left in its own section,
  so the ticket no longer reads as having anything pending: TypeScript pinned to
  6.0.3, tsc now and vue-tsc once SFCs exist, with the reason the pin exists and
  the condition for lifting it.

  Also refreshed Sequencing, which still told the reader to do this before TQ-0030
  and TQ-0032. Both shipped against app.ts in the meantime, so that advice had
  gone stale and the UI they added is now work this migration will port. TQ-0035
  is the only one of the three still ahead, and its ordering is genuinely open —
  it is deliberately not blocked on this ticket, since most of it is config, store
  and CLI rather than board.
- 2026-08-26T13:13:31+02:00 — Done. app.ts is gone; the board is 13 single-file components over frontend/state.ts, with board.ts and notes.ts unchanged, framework-free and still the safety net.

  Numbers worth correcting in this ticket's own record: public/app.js is 217 KB, not the 156 KB the Settled section projected. That figure came from a two-component prototype, and the claim that 'application code is rounding error against the Vue floor' did not survive a real 13-component app. Every criterion still holds — unminified, three feature flags applied and folded, deterministic, staleness gate clean — but nobody should treat 156 KB as a budget.

  One decision taken during the work that was not in the Settled list: a fourth define, process.env.NODE_ENV = production. Without it Bun bundles Vue's development runtime — component warnings, prop validators — into the release binary at 274 KB. Documented in build.ts.

  Five parallel reviews ran at the end. Security found nothing and confirmed the migration removes the codebase's last innerHTML. Architecture found no rule violation and verified the flags are folded rather than merely passed. The correctness review found one real regression against the old board, fixed in the follow-up commit: filing a card closed a composer opened in another column, because the shared composing ref was written unconditionally after an await. Its unmount blur then filed the partial text, which at typing speed is a card titled 't'.

  The most valuable finding was that three tests pinned less than they looked like. Mutation testing showed the append path to the TQ-0010 bug was unprotected, the pencil's keyboard half was unprotected, and matching notes on text alone passed every mergeNotes test because none of them reordered the file — the one property matching-by-content exists for. All now covered; the merge rule's real limit (two notes identical in both fields) is written down as a test.

  Filed rather than fixed here: TQ-0079 (save still overwrites body content, predates this), TQ-0080 (CreateDialog, Toasts, filter-bar wiring and keyboard card-open are untested at any layer), TQ-0081 (a dialog reopened within a millisecond is swallowed, and a poll-gate guard), TQ-0082 (noUncheckedIndexedAccess).

  TQ-0068, TQ-0069 and TQ-0070 are unblocked by this.

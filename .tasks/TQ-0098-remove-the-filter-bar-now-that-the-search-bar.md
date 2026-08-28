---
id: TQ-0098
title: Remove the filter bar now that the search bar replaces it
status: done
priority: normal
labels:
  - chore
  - component/frontend
created: 2026-08-28T23:57:50+02:00
updated: 2026-08-29T00:32:57+02:00
---

## Decision

The search bar (TQ-0068) covers everything the filter bar does, so the filter
bar goes. `FilterBar.vue` is removed and the search bar becomes the only way to
narrow the board.

## What it replaces

| Filter bar control | Search bar |
|---|---|
| `#filter-status`   | `status=todo` |
| `#filter-priority` | `priority=high` |
| `#filter-assignee` | `assignee=agent-api` (substring) |
| `#filter-label`    | `label=bug` |
| `#filter-ready`    | `ready` / `-ready` |
| `#filter-reset`    | the `×` clear button |

The search bar also does what the bar never could: free text over id, title and
body, AND-of-words, quoted phrases, negation, and a shareable `?q=` URL.

## What this simplifies

With nothing else writing `filters`, the search bar is the sole writer. The
query becomes the input and `filters` a derived value, so the filters-to-query
direction (`formatQuery`, and the watcher that keeps the two in step) may no
longer be needed. Check before deleting — `?q=` round-tripping on load still has
to work.

`FilterBar.vue` is imported only by `App.vue` (`:18`, `:29`); nothing else
references it.

## What is lost, stated deliberately

The label select grouped labels by their scope, so opening it showed **what
labels exist** without knowing any of them. The search bar's autocomplete offers
the same values but only once `label=` has been typed. That is a real loss of
discoverability for a new user, accepted in exchange for one control instead of
six.

## Test rework

`#filter-*` appears **43 times across five browser files**:

- `browser/filters.test.ts` — 17 hits; the whole file is about the removed
  controls (TQ-0080). It goes, and what it pinned — that a control actually
  narrows the board — must survive as search-bar coverage.
- `browser/search.test.ts` — 12 hits, all asserting the search bar moves the
  selects. Rewrite against the board and the URL instead.
- `browser/labels.test.ts` — 10 hits.
- `browser/priorities.test.ts` — 3 hits.
- `browser/config.test.ts` — 1 hit.

`frontend/board.test.ts`'s `visibleTasks` cases are unaffected — the rules do not
change, only what drives them.

## Acceptance

- The board can be narrowed by every field the filter bar offered, through the
  search bar alone.
- `?q=` still round-trips on load.
- No `#filter-*` id remains in the app or the suite.
- Nothing that `filters.test.ts` pinned is left unpinned.

---

## Notes

- 2026-08-29T00:13:11+02:00 — FilterBar.vue is gone, and the search bar is now the only writer of `filters`.

  formatQuery did NOT survive. With one writer the filters-to-query direction was
  already unreachable: the `watch(filters, ...)` guard compared
  `equalFilters(parseQuery(query), filters)`, and the only thing writing `filters`
  was the parsed watcher, whose value differs from `parseQuery(query)` by case
  alone — which `equalFilters` ignores. So it never rewrote the line. The other
  call site, `queryFromURL(...) || formatQuery(filters)`, was `... || ""`, since
  SearchBar mounts once, before anything can have filtered. Both are gone, and
  with them `quoteText` and `readsAsTerm`, which nothing else called. `quoteValue`
  stays: the autocomplete builds its inserts with it.

  `?q=` still round-trips: the line is read out of the address on load, `parsed`
  canonicalises it against the vocabularies as they land, and the parsed watcher
  writes it through — covered end to end in browser/search.test.ts.

  The `sameFilters` / `equalFilters` split from the TQ-0068 review is kept and its
  reasoning restated rather than reused: the guard still has to see a correction
  `canonicalValues` made, because `filters` is meant to hold exactly what the query
  parses to, spelling included. `equalFilters` alone would call the correction
  equal and skip the write.

  Also removed as dead: `groupLabels`/`LabelGroup`/`GroupedLabel` in board.ts (the
  optgroups were the select's alone), the `.filters`, `label.checkbox` and
  `input[type=checkbox]` rules in style.css, and the tests for both.

  Test rework: browser/filters.test.ts is gone; every control it pinned is now a
  search term in browser/search.test.ts, asserted against the board and the footer
  (status, priority, assignee-as-substring, label, ready/-ready, and the x
  clearing all five at once). labels.test.ts, priorities.test.ts and
  config.test.ts read the `label=` / `priority=` suggestion menu where they read
  `<option>` lists before; the scoped-label " | " spelling is still pinned there.

  Documentation followed: README.md and internal/guide/AGENTS.tmpl.md both said
  the box moved the controls below it. Regenerated .tasks/AGENTS.md.
- 2026-08-29T00:31:33+02:00 — Checks: make typecheck, make frontend, make test-frontend (256 pass), make test,
  make lint (0 issues), make build, and make test-integration all clean.

  make test-browser: 98/98 on the first full run, with a leak delta of zero — the
  temp directory held 1536 tq-browser-* dirs before and after, all of them from
  runs that predate TQ-0092. Two later full runs wedged, once in live.test.ts and
  once in notes.test.ts, both while the machine was carrying a review agent and a
  make dev watcher; each of those runs left three dirs behind, which is the
  teardown being killed rather than the harness leaking. Every file passes
  file-by-file, all eleven of them, and that includes both files that wedged.

  The code review was invoked once at effort high and had returned nothing after
  eighteen minutes, so it was abandoned per the brief and the diff was
  self-reviewed instead. Two things that pass found and fixed: the exclusion step
  of the browser minus test had no distinguishable count between its two queries,
  so it could have passed against a query that never landed (a clear to "3 tasks"
  now sits between them); and three test comments still explained themselves in
  terms of a select or a checkbox.

  Not done, deliberately: the README screenshot still shows the filter bar.
- 2026-08-29T00:32:57+02:00 — The code review landed just after the commit, at eighteen and a half minutes.
  Five findings, none of them in the changed lines.

  Four are in browser/harness.ts, which TQ-0092 committed and this task was told
  not to touch: page.setDefaultTimeout applied after browser.newPage() leaves the
  first await of every test on Playwright's own 30s, tied with the test timeout;
  giving up on page.close() after 5s orphans a live page that goes on polling a
  server about to be killed, which is a good explanation for the cascades seen
  here; CLOSE_TIMEOUT_MS of 5s is short for a Chromium close under load, so the
  next file's browser starts on top of the old one; and the one shared readiness
  deadline can make the probe blame the half it never reached. The reviewer's own
  control run has origin/main's harness passing 98/98 too, so these are about
  behaviour under load rather than a regression. Left for a harness ticket.

  The fifth is adjacent to this diff and worth someone's judgement: the suggestion
  menu resets its highlight to row 0 whenever `sources` changes, which is whenever
  the task listing does. The filter bar used to freeze its label options while the
  select had focus for exactly this reason, and that guard is what this task
  deletes — but the search menu never had one and its code is unchanged here.
  Narrow (the listing has to genuinely change while the menu is open and the
  highlight is off row 0) and rated low. It wants its own ticket and its own
  browser test rather than a rider on this commit.

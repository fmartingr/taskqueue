---
id: TQ-0068
title: Global search bar
status: done
priority: normal
depends_on:
  - TQ-0076
created: 2026-08-25T18:37:33+02:00
updated: 2026-08-28T20:30:43+02:00
---

A global seearch bar that allows us to either search by text or by setting the attributes using priority=XXX with autocomplete.

---

## Notes

- 2026-08-26T10:31:16+02:00 — Blocked on TQ-0076 (migrate the board to Vue 3). TQ-0076's own note names this ticket: built against app.ts it is imperative DOM that gets ported again; built after, it is a component.
- 2026-08-28T20:06:28+02:00 — Finished the PoC. The syntax it settled on:

  - Free text is AND-of-words: each bare word is its own term and all of them
    have to be found in the id, title or body. A quoted run is one phrase term,
    so "global search" is the old behaviour and is now how a phrase is asked
    for. Word order does not matter; a phrase's does.
  - Negation is a leading unquoted dash. -word excludes free text,
    -"two words" excludes a phrase, -priority=low excludes a value, and an
    exclusion is repeatable — the positive key keeps one slot because a select
    has one, the negative ones are a list because nothing shows them.
  - Quoting wins over the dash: "-word" searches for the dash, which is the
    only way to. A lone - is a negation being typed, not a term.
  - -ready is ready=false. The board's ready control is a checkbox with two
    states and no third, so a tri-state would have meant rewriting FilterBar for
    a filter nobody asked for. Stated in the guide and the README.
  - Values now match case-insensitively everywhere (status, priority, whole
    label, assignee substring, free text), and the component canonicalises a
    mis-cased value against the project's own vocabulary so the select follows
    it. equalFilters compares without case, which is what keeps the line from
    being rewritten under the cursor when it does.
  - Persistence is ?q= in the address, written with history.replaceState so
    typing leaves no history entries, read back on load. No router, no
    localStorage; a filtered board is a link.

  Also: a clear button in the box, / to focus it (standing down for inputs,
  textareas, contenteditable and any open dialog), and the menu scrolls its
  active row into view.

  Checks: typecheck, frontend, test-frontend (262), test-browser (99 across 13
  files, each file green on its own), test, lint, build. The full back-to-back
  browser run times a file out — notes.test.ts at HEAD, live.test.ts with this
  change — which is TQ-0092: I ran the whole suite on a stashed tree at HEAD and
  it failed the same way.
- 2026-08-28T20:30:12+02:00 — Code review (high) found one real bug, fixed here.

  The canonicalisation was dead. The watcher that pushes the parsed line into
  the filters guarded on equalFilters, which ignores case — and canonicalValues
  only ever changes case, so the correction could never be assigned. It looked
  fine only because the board seeds FALLBACK_COLUMNS and FALLBACK_PRIORITIES, so
  a project using the built-in vocabulary is canonical on the first parse.
  A project with its own columns, and any project at all for labels (which start
  empty), got a blank Status select beside a board that was hiding cards, and a
  duplicate 'not in the project's label set' option beside the real one.

  The fix is two questions instead of one. equalFilters still answers 'does the
  line have to be rewritten', where case must not count — correcting a value
  under the cursor is the thing to avoid. sameFilters answers 'does the control
  have to be moved', where case does count, because a select is an exact list.
  Only the three fields canonicalValues rewrites are compared exactly: including
  the assignee would trim what someone was still typing into it.

  browser/search.test.ts now opens ?q=status=SPOTTED+label=Glitch against a
  project declaring its own columns and labels, and asserts the selects land on
  'spotted' and 'glitch' while the line stays as the address carried it. Checked
  that it fails without the fix.

  Also from the review: NO_FILTERS held mutable arrays that every { ...NO_FILTERS }
  spread aliased. It is frozen now, arrays included, so a stray push throws where
  it is written rather than poisoning the module's idea of 'no filters'.

---
id: TQ-0083
title: Scoped labels hide their scope in the UI
status: todo
priority: normal
labels:
  - bug
  - component/frontend
  - component/config
created: 2026-08-26T13:20:40+02:00
updated: 2026-08-26T13:22:41+02:00
---

## Symptom

A chip for `component/api` reads **API**. `component/cli` reads **CLI**,
`component/frontend` reads **Frontend**. The scope — the half that says what
kind of label it is — never reaches the board, so a card carrying
`component/api` looks the same as one carrying a hypothetical `type/api`, and
the grouping the `/` was introduced for is invisible outside the filter bar.

## Cause

`labelDisplay` in `frontend/board.ts:112` is `display_name || name`, and the
seeded defaults give every scoped label a display name that is only the tail:
`internal/config/labels.go:53-54` has `{"component/cli", {"#0052cc", "CLI"}}`
and `{"component/api", {"#006b75", "API"}}`. So the configured name replaces the
whole label rather than decorating it. `labelChip` is single-tone by design and
has nowhere to put a second half.

## Wanted: a scoped chip, GitLab style

One pill, two halves:

```
[ Component | API ]
   ^ label colour    ^ contrasting half
```

- Scope half: the **scope's** colour as background, readable text on it.
- Value half: the contrasting surface, with the scope colour as its text, and
  the pill's border in that colour so the two halves read as one object.
- A label with no `/` keeps the single-tone chip it has now.
- "White" has to be a theme token, not a literal: the board has a dark palette,
  and the existing chips already compute readable text rather than assuming.

Applies to card chips, the filter bar and the task dialog. The CLI keeps
printing raw keys — that is what agents match on.

## The colour belongs to the scope

Every label in a group is drawn in the same colour: all `component/*` chips
share one, and the value half is what tells them apart. That is the point of a
scope — it should be recognisable across a board at a glance, and today
`component/cli` is `#0052cc` while `component/api` is `#006b75`, so the group
reads as unrelated labels that happen to share a prefix.

So the colour stops being per-label for scoped labels and becomes a property of
the scope:

```yaml
labels:
  bug:
    color: "#d73a4a"
  component/api:
    display_name: API
  component/cli:
    display_name: CLI

label_scopes:
  component:
    color: "#1d76db"
    display_name: Component
```

- A scoped label takes its colour from its scope. A `color` on a scoped label is
  only read when the scope has none — decide whether that is a silent fallback
  or a config warning; a config that carries a colour nobody uses will drift.
- Unscoped labels keep their own `color`, unchanged.
- A scope with no entry in `label_scopes` still has to render. Recommendation:
  derive a colour deterministically from the scope name, so freeform groups stay
  visually distinct without anyone configuring them; the alternative is the
  neutral chip, which makes every unconfigured group look identical.
- `display_name` on the scope covers the same casing problem as on a label:
  nothing derives `CI` from `ci`.

This changes the seeded defaults and the base-set table in TQ-0030: the
`component/*` rows lose their individual colours and the group gains one.

## What happens to `display_name`

Make it **optional**, and define it as overriding the **value half only**. With
no display name, both halves come from the key by title-casing each segment:
`component/api` -> `Component` / `Api`.

It cannot simply be deleted, and that is worth being explicit about: nothing
derives `API` from `api`, or `CLI` from `cli`. Casing is the entire remaining
job of that field, which is why the seeded defaults should keep supplying it
while the scope stops coming from it.

The alternative, if the field really should go: let the key carry its own
casing — `component/API` — the way a GitLab label name *is* its display string.
That costs a rewrite of the label on every task that carries one (70+ today) and
makes `tq list --label component/api` case-sensitive. Not recommended, but it is
the version with no `display_name` at all.

## Tests that will have to change, deliberately

- `frontend/board.test.ts:235` asserts `labelDisplay("component/backend")` is
  `"Backend"`. That expectation is the bug, so it changes to a scope/value pair.
- `browser/labels.test.ts:38`, "a configured label is drawn with its colour and
  display name", should assert both halves are drawn and that the raw key is
  still what the task carries.
- New unit coverage for splitting a key into scope and value, including a label
  with several separators (`a/b/c` — decide: first separator splits, the rest
  stay in the value) and one that starts or ends with a separator.

## Acceptance criteria

- `component/api` renders as `Component | API` on cards, in the filter bar and
  in the task dialog.
- Every `component/*` chip is the same colour, and the value half is what
  distinguishes them; changing the scope's colour recolours the whole group in
  one edit.
- A label with no `/` is unchanged.
- `display_name` is optional; absent, the halves derive from the key; present,
  it sets the value half.
- Both themes checked, not just light.
- The stored label and every CLI surface are untouched.

---
id: TQ-0083
title: Scoped labels hide their scope in the UI
status: in-progress
priority: normal
labels:
  - bug
  - component/frontend
  - component/config
created: 2026-08-26T13:20:40+02:00
updated: 2026-08-27T16:26:12+02:00
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

- Scope half: the label's colour as background, readable text on it — what the
  chip does today.
- Value half: the contrasting surface, with the label colour as its text, and
  the pill's border in the label colour so the two halves read as one object.
- A label with no `/` keeps the single-tone chip it has now.
- "White" has to be a theme token, not a literal: the board has a dark palette,
  and the existing chips already compute readable text rather than assuming.

Applies to card chips, the filter bar and the task dialog. The CLI keeps
printing raw keys — that is what agents match on.

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
  in the task dialog, with the scope on the label colour.
- A label with no `/` is unchanged.
- `display_name` is optional; absent, the halves derive from the key; present,
  it sets the value half.
- Both themes checked, not just light.
- The stored label and every CLI surface are untouched.

---

## Notes

- 2026-08-27T16:26:12+02:00 — Implemented the two-half scoped chip.

  frontend/board.ts: labelHalves(name, labels) splits a key at its FIRST separator — scope title-cased always, value title-cased only when it comes from the key. display_name now names the value half alone, so component/api reads 'Component | API' instead of losing the half that says it is a component. A display name equal to the key is treated as absent, because internal/config fills an empty one in with the key (that is what tq label list prints) and it would otherwise land in the value half. A key with an empty half (/x, x/) is not scoped: half a pill says less than the whole key. labelDisplay now returns the halves joined with ' | ', for the one place that has a single line of text — the filter bar's options, which would otherwise read 'Backend' beside a chip reading 'Component | Backend'.

  LabelChip.vue draws one pill in two spans: the colour on .label-scope with text picked to contrast, .label-value on var(--surface) with the colour as its text, and the pill's border inline in the colour so the halves read as one object. The contrasting half is a theme token, not white — checked in both palettes in the browser test. A flat label is byte-identical to before; a scoped label the project does not declare keeps both halves neutral (--surface-alt scope, --border ring).

  Noted for the record: a very pale colour (component/frontend, #c5def5) as the value half's text on the light surface is faint, and a very dark one is faint on the dark surface. That is the specified design — GitLab's scoped pill has the same property — and it is only the value half; the scope half still computes contrast.

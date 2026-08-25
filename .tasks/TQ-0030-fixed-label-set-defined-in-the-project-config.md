---
id: TQ-0030
title: Fixed label set defined in the project config
status: todo
priority: normal
labels:
  - component/config
  - component/frontend
  - feature
depends_on:
  - TQ-0029
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-25T18:37:29+02:00
---

## Proposal

Store the project's label set in `.taskqueue.yaml` so labels are a fixed,
reviewable vocabulary instead of whatever string was typed last.

```yaml
version: 1
labels:
  bug:
    color: "#d73a4a"
    display_name: Bug
  component/backend:
    color: "#1d76db"
    display_name: Backend
```

- The map key is the label exactly as it is stored in task frontmatter.
- A `/` in the key is **display grouping for the frontend only**, the way
  GitLab groups scoped labels. Storage stays one flat string, so
  `tq list --label component/backend` keeps working unchanged.
- `color` must be a quoted hex string (unquoted `#ff0000` is a YAML comment).
- `display_name` is what the board shows; the key is what the CLI takes.

## Behaviour

- **Labels stay freeform**, unlike columns and severities. The configured set
  is a reference, not a restriction: it supplies colours, display names,
  grouping and autocomplete, and gives humans and agents a shared vocabulary,
  but `tq add --label`, `tq update --add-label` and the API all accept a label
  that is not in it.
- A label in use but not configured renders in a neutral colour and is worth
  surfacing, so it either joins the set or gets cleaned up.
- **Reading never fails on an unknown label.** Tasks that already carry labels
  outside the set keep them and render in a neutral colour; otherwise adopting
  a config would break every task already filed.
- Board: group labels in the filter bar by their prefix, colour the chips, show
  `display_name` with the raw key as the tooltip.

## Base set to ship

Types stay flat, components are grouped, which matches how the labels already
in this repository divide up. Colours are a starting point, not a decision.

| Label | Display | Colour |
| --- | --- | --- |
| `bug` | Bug | `#d73a4a` |
| `feature` | Feature | `#0e8a16` |
| `chore` | Chore | `#8b949e` |
| `docs` | Docs | `#0075ca` |
| `refactor` | Refactor | `#6f42c1` |
| `security` | Security | `#b60205` |
| `performance` | Performance | `#fbca04` |
| `tests` | Tests | `#5319e7` |
| `component/backend` | Backend | `#1d76db` |
| `component/frontend` | Frontend | `#c5def5` |
| `component/cli` | CLI | `#0052cc` |
| `component/api` | API | `#006b75` |
| `component/store` | Store | `#5319e7` |
| `component/ci` | CI | `#bfd4f2` |
| `component/build` | Build | `#d4c5f9` |
| `component/config` | Config | `#c2e0c6` |

## Migration

**Already done.** Every ticket (TQ-0001 to TQ-0037) was relabelled onto this
set, so the vocabulary in `.tasks/` is now exactly the table above: each ticket
carries one type and at least one component, and nothing else is in use.

The mapping applied:

- `store`, `frontend`, `cli`, `api`, `ci`, `build` became `component/*`.
- `tests` and `security` were already type labels and stayed.
- `config` became `component/config`, which is why the table gained a row.
- Types were added where there were none: `bug` for the review findings,
  `feature` for the config and board work, `chore` for the GitHub migration,
  `performance` for the favicon, `docs` for the identity move.

Four labels were **dropped**: `review` (23 tickets), `data-loss` (5),
`concurrency` (4) and `ux` (3). They carried real signal — provenance for the
code-review batch, and impact class for the destructive bugs — so if any of them
is worth keeping, add it to the table and relabel; the review findings are the
contiguous range TQ-0006 to TQ-0028 and each one says so in its body, and the
destructive ones are the tickets already marked urgent.

Doing this by hand showed the gap: relabelling meant scripting
`tq update --add-label/--remove-label` per ticket, because there is no way to
set a label set outright and no rename. A `tq label rename old new` would have
made this one command.

## Open questions

- Strict by default once a config exists, or an escape hatch such as
  `--allow-new-label` for quick capture?
- Is a `tq label list [--json]` worth having so agents can discover the
  vocabulary without parsing YAML?
- Both themes need readable chips: check the colours against the dark palette
  in `style.css`, not just the light one.

## Acceptance criteria

- Configured labels supply colours, display names and grouping; an
  unconfigured label is accepted everywhere and renders neutral.
- `tq label list [--json]` prints the configured vocabulary and flags labels
  that are in use but not configured.
- The board groups by prefix, colours the chips and shows display names.
- `tq init` seeds the base set into a new config.

## Notes

- 2026-08-25T12:02:00+02:00 — Strict by default once a config exists, or an escape hatch such as `--allow-new-label` for quick capture? -> Allow new labels when creating tasks, those serve as reference for the human, agent and future autocomplete.
- 2026-08-25T12:02:12+02:00 — Is a `tq label list [--json]` worth having so agents can discover the vocabulary without parsing YAML? -> Yes
- 2026-08-25T12:02:28+02:00 — - Both themes need readable chips: check the colours against the dark palette in `style.css`, not just the light one. -> Yes

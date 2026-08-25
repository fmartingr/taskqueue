---
id: TQ-0030
title: Fixed label set defined in the project config
status: todo
priority: normal
labels:
  - config
  - frontend
  - cli
depends_on:
  - TQ-0029
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-25T11:53:19+02:00
---

## Proposal

Store the project's label set in `.tasks/config.yaml` so labels are a fixed,
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

- With a config present, `tq add --label`, `tq update --add-label`, `POST
  /api/tasks` and `PATCH /api/tasks/{id}` reject labels that are not in the
  set, and the error lists the valid ones.
- Without a config, any label is accepted — today's behaviour, so nothing
  breaks before a project opts in.
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

## Migration

This repository currently uses, by frequency: `review` (23), `store` (9),
`data-loss` (5), `frontend` (5), `concurrency` (4), `tests` (3), `ux` (3),
`ci` (3), `api` (2), `cli` (2), `security` (1), `build` (1). Most map onto the
set above (`store` to `component/store`, and so on); `review`, `data-loss`,
`concurrency` and `ux` are genuinely useful and should either join the base set
or be dropped deliberately. Decide before turning validation on, and mention
whether a rename helper is worth it (there is no `tq delete`, and renaming a
label today means editing frontmatter by hand).

## Open questions

- Strict by default once a config exists, or an escape hatch such as
  `--allow-new-label` for quick capture?
- Is a `tq label list [--json]` worth having so agents can discover the
  vocabulary without parsing YAML?
- Both themes need readable chips: check the colours against the dark palette
  in `style.css`, not just the light one.

## Acceptance criteria

- Labels come from the config; an unconfigured label is rejected on write with
  a useful message, and tolerated on read.
- No config still means no restriction.
- The board groups by prefix, colours the chips and shows display names.
- `tq init` seeds the base set into a new config.

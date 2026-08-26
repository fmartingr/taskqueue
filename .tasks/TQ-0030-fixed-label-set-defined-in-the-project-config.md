---
id: TQ-0030
title: Fixed label set defined in the project config
status: done
priority: normal
labels:
  - component/config
  - component/frontend
  - feature
depends_on:
  - TQ-0029
created: 2026-08-25T11:53:19+02:00
updated: 2026-08-26T08:38:06+02:00
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

---

## Notes

- 2026-08-25T12:02:00+02:00 — Strict by default once a config exists, or an escape hatch such as `--allow-new-label` for quick capture? -> Allow new labels when creating tasks, those serve as reference for the human, agent and future autocomplete.
- 2026-08-25T12:02:12+02:00 — Is a `tq label list [--json]` worth having so agents can discover the vocabulary without parsing YAML? -> Yes
- 2026-08-25T12:02:28+02:00 — - Both themes need readable chips: check the colours against the dark palette in `style.css`, not just the light one. -> Yes
- 2026-08-26T08:25:34+02:00 — Implemented. internal/config/labels.go holds the vocabulary, its validation and the base set; `tq label list` is in internal/cli/label.go; GET /api/config carries the set; frontend/board.ts got the pure label rules (display name, chip colours, prefix grouping) and app.ts the DOM.
- 2026-08-26T08:25:34+02:00 — Decision on where the base set is seeded. The ticket says `tq init` seeds it, but init in a fresh repository never reaches the CLI's own WriteConfigIfMissing call — store.InitStore writes the marker first. So the seed lives in WriteConfigIfMissing, which means any command that creates a queue writes the vocabulary with it. That is the consistent answer: if `tq add` created a minimal marker, a later `tq init` would not rewrite it and the project would silently never get a set.
- 2026-08-26T08:25:34+02:00 — Decision on an absent labels key. Absent means the base set, matching TQ-0029's rule that a missing key means the default; `labels: {}` means the project wants no vocabulary. yaml.v3 distinguishes the two (nil vs empty map), so both are expressible, and Config.LabelSet reads through a nil *Config so a project with no config file at all gets the same set.
- 2026-08-26T08:25:34+02:00 — Decision on chip colours in both themes. Each chip carries its own background — the configured hex — with its text picked as black or white by WCAG contrast against that colour, which is how GitHub draws labels. Nothing is themed, so one set of colours in .taskqueue.yaml is readable on both palettes by construction rather than by choosing colours that happen to work twice. Checked against a real board in both schemes: the pale ones (#c5def5 Frontend, #c2e0c6 Config) get dark text, the saturated ones white.
- 2026-08-26T08:25:34+02:00 — The label filter changed from a substring search box to a select grouped by prefix, which is what "group labels in the filter bar" asks for. That makes the filter exact rather than substring — with a list of the labels that exist there is nothing to search, and substring matching would let "backend" also select "component/backend". frontend/board.ts:visibleTasks and its test moved with it.
- 2026-08-26T08:25:45+02:00 — Not done, deliberately. (1) No `tq label rename`: the ticket raises it in the migration prose but it is not in the acceptance criteria, so `tq label` takes a subcommand and has room for it. (2) No autocomplete on the dialogs' Labels field: it is a comma-separated text input, and a <datalist> there suggests replacing the whole value rather than the token being typed, which is worse than nothing. Both are worth their own ticket.
- 2026-08-26T08:25:45+02:00 — This repository's own .taskqueue.yaml now carries the base set explicitly, copied from what a fresh `tq init` seeds, so the vocabulary is reviewable in the repo rather than implied by the binary's defaults. It changes nothing behaviourally — the set was already the default — and can be dropped back to an absent key if you would rather the file stayed minimal.
- 2026-08-26T08:25:46+02:00 — `tq label list` on this repository surfaces one label in use that the set does not declare: `wontfix`, on 5 tickets. Left alone — adopting it into the table or relabelling those tickets is a call about the vocabulary, not part of this change. This is exactly the surfacing the ticket asked for.
- 2026-08-26T08:36:33+02:00 — Code review found six things; five fixed, one kept deliberately.

  Fixed: tq label list resolved the vocabulary from the working directory while counting tasks from TQ_DIR, so the CLI and the board could disagree about which labels are configured — it now resolves from the store's directory, as the server does. The TASKS column counted label occurrences, so a task carrying one label twice counted as two. The board's `name in labels` reached Object.prototype, so a task labelled constructor or toString was reported as configured; it uses Object.hasOwn now. The 3s poll rebuilt the filter's option list under an open dropdown when an agent added a task with a new label; the rebuild now stands down while the select has focus and resumes on blur, the way the poll already stands down for drag and dialogs.
- 2026-08-26T08:36:33+02:00 — Also fixed, not from this ticket but made visible by it: the generated guide carried `# task.Task queue` and `- task.Statuses:`, and tq help carried the same, from a package-qualifier rename that landed in a string literal. .tasks/AGENTS.md had been stale so nobody saw it; regenerating the guide here would have committed the corruption. Fixed at the source in guide/agents.go and cli.go and regenerated.
- 2026-08-26T08:36:33+02:00 — Kept as is, and worth a second opinion: a bad label colour fails every command, not just the drawing. validateLabels runs in loadConfig, which every command reaches through discovery, so an unquoted hex — the exact mistake the error text anticipates — makes the queue unreadable until the file is fixed. The review argued that colour is presentation and should degrade to a neutral chip.

  Kept strict because it matches the contract the marker already has: a future version, malformed YAML and the .yml typo all fail the same way and name the file, and TQ-0029 chose that deliberately. The alternative fails silently at the one thing the config exists to do — a chip that never gets its colour and nobody notices. If you would rather it degraded, the change is one branch in validateLabels plus a warning on stderr.

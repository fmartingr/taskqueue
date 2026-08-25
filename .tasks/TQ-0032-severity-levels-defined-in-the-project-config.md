---
id: TQ-0032
title: Severity levels defined in the project config
status: todo
priority: normal
labels:
  - component/config
  - feature
depends_on:
  - TQ-0029
created: 2026-08-25T11:59:41+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Proposal

Move the severity vocabulary into `.tasks/config.yaml`, the same treatment
labels get in TQ-0030. Today it is hard-coded as the four `priority` values
(`urgent`, `high`, `normal`, `low`) in `task.go:46`.

Unlike labels, severities are **ordered**, and a YAML map does not preserve
order through a plain `map[string]T` decode. So this key is a sequence, most
severe first, and the file itself is the ranking:

```yaml
version: 1
severities:
  - name: urgent
    color: "#b60205"
    display_name: Urgent
  - name: high
    color: "#c2410c"
    display_name: High
  - name: normal
    color: "#4b5563"
    display_name: Normal
    default: true
  - name: low
    color: "#6b7280"
    display_name: Low
```

- `name` is what is stored in task frontmatter, so `priority: high` keeps
  working and nothing about the file format changes.
- Exactly one entry carries `default: true` (today: `normal`); zero or two is a
  config error with a clear message.
- Colours are quoted hex, as for labels.

## Behaviour

- No config, or no `severities` key: the built-in four, in today's order, with
  `normal` as the default. Nothing changes for a project that never opts in.
- With a config, writes validate against the set: `tq add --priority`,
  `tq update --priority`, `POST /api/tasks`, `PATCH /api/tasks/{id}` reject
  anything else and list what is valid.
- Reads stay tolerant, as with labels: a task carrying a value that is no longer
  configured keeps it, sorts last, and renders in a neutral colour. Otherwise
  editing the config would break every task already filed.
- `SortTasks`/`priorityRank` (`task.go:56`) rank by position in the configured
  sequence instead of the hard-coded slice.

## Everything that currently hard-codes the list

- `task.go:46` `Priorities`, plus `ValidPriority`, `priorityRank`, and the two
  error messages at `task.go:78` and `task.go:151`.
- `cli.go:92` and `cli.go:160`: the usage text and the `--priority` flag help.
- `agents.go:214`: the generated `.tasks/AGENTS.md` prints the priority list to
  agents, so it has to print the configured one or it becomes a lie.
- `frontend/index.html:27,85,155`: three hard-coded `<select>` blocks (filter
  bar, task dialog, create dialog) that must be built from `GET /api/config`.
- `frontend/style.css:15-16,37-38,324-329`: `--urgent`/`--high`/... variables and
  `.priority-*` rules, replaced by colours from the config.

## Open question: severity or priority?

Settle this before implementing, because it decides whether this is additive or
breaking.

1. **Rename the existing field** `priority` to `severity` in frontmatter, JSON,
   CLI flags and API. That breaks the file format and the stable agent JSON
   contract, so it needs a config `version` bump, a migration that accepts both
   keys for a release, and a deprecation note. This ticket assumes we do *not*
   do this: the field stays `priority`, only its vocabulary becomes configurable.
2. **Add severity as a second dimension** alongside priority, the way GitLab and
   Jira separate impact from order-of-work. That is a bigger, separate ticket: a
   new frontmatter key, a new filter, a new board affordance, and a decision
   about what `tq ready` sorts by.

Recommendation: keep one field, make its vocabulary configurable, and only
rename if the semantics genuinely change.

## Acceptance criteria

- Without a config, behaviour is identical to today, including sort order and
  the default.
- A configured set drives validation on write, sort order, CLI help, the
  generated agent guide, and the board's selects and badge colours.
- A value that is no longer configured is tolerated on read, sorts last, and
  renders neutral.
- Exactly one default is enforced with a useful error.
- Round-trip test with a custom set, for example `p0`..`p3`, covering create,
  filter, sort and the board.

## Notes

- 2026-08-25T12:12:37+02:00 — Columns and severities are the strict sets, labels are freeform (see TQ-0035 and TQ-0030). Open question for severities: should a task carrying a severity that is no longer configured snap to the default the way an unknown column snaps to the first column, or keep the value and sort last as this ticket currently says?

---
id: TQ-0032
title: Severity levels defined in the project config
status: done
priority: normal
labels:
  - component/config
  - feature
depends_on:
  - TQ-0029
created: 2026-08-25T11:59:41+02:00
updated: 2026-08-26T09:24:43+02:00
---

## Proposal

Move the severity vocabulary into `.taskqueue.yaml`, the same treatment
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

---

## Notes

- 2026-08-25T12:12:37+02:00 — Columns and severities are the strict sets, labels are freeform (see TQ-0035 and TQ-0030). Open question for severities: should a task carrying a severity that is no longer configured snap to the default the way an unknown column snaps to the first column, or keep the value and sort last as this ticket currently says?
- 2026-08-26T08:53:03+02:00 — Settled the open question: the frontmatter field stays 'priority' and the config key is 'priorities' too, not 'severities'. One name for one concept — the config key matches the field, the --priority flag and the JSON, so nobody has to learn that one word configures the other.
- 2026-08-26T09:15:56+02:00 — Implemented. Both open questions in the ticket, settled:

  1. Severity or priority — kept one field. The frontmatter key, the --priority
     flag and the JSON are unchanged, so nothing about the file format moves and
     the agent JSON contract holds. The config key is 'priorities' too, not
     'severities': one name for one concept, so nobody has to learn that one word
     configures the other. No version bump, no migration.
  2. A value the project has dropped — kept, not snapped to the default, as the
     ticket body and acceptance criteria say. It sorts last and renders neutral.

  Where that landed in code, since it is the part worth knowing:

  - internal/task gets a Priorities value type (ordered names + default) whose
    zero value is the built-in set. internal/task still imports nothing of ours;
    internal/config builds the value from the file. SortTasks and Filter.Validate
    take it.
  - Task.Validate no longer checks the priority at all, so reads are tolerant:
    a task filed under a value the project has since dropped still parses, lists
    and shows. This is a behaviour change with no config present — a file with a
    nonsense priority used to fail the whole listing and now does not.
  - Writes are checked where a caller supplies a value: Store.Create, and
    Store.Patch only when the patch would CHANGE the priority. That last part
    matters: the board's dialog sends every field at once, so checking the value
    rather than the change made every task under a dropped priority impossible to
    move, retitle or close. The browser suite caught it.
  - Store.Priorities() re-reads the config each call, so an edit to .taskqueue.yaml
    reaches a running server on its next request, like an edit to a task file.

  Also found by the browser suite: the create dialog resets its form on open,
  which restores markup defaults — so the configured default had to become the
  option's own defaultSelected rather than a value assigned after filling, or the
  second open silently fell back to the most severe level.

  The badge is now a filled chip like a label chip, for the same reason: the text
  has to contrast with the configured colour, not with the page, so one set of
  colours in .taskqueue.yaml stays readable in both themes. The --urgent/--high/
  --normal/--low CSS variables and .priority-* rules are gone.
- 2026-08-26T09:24:43+02:00 — Reviewed. No correctness issues; four of the reviewer's smaller observations were worth acting on:

  - The generated guide can now go stale. It used to print compile-time
    constants, so it could not; now editing 'priorities:' leaves .tasks/AGENTS.md
    asserting the old set until someone re-runs 'tq init'. The CLI help and the
    board both re-read on every run, so this is the one surface that lags.
    Documented in the README rather than made automatic — writing files from
    arbitrary commands is init's job, not every command's.
  - Dropped the badge's 'text-transform: uppercase'. The text is now the display
    name the project chose, so shouting it back is the same mistake as ignoring
    its colour would be. Label chips never uppercased; the two match now.
  - Priorities.Default() now falls back to the most severe when no entry is
    marked, matching defaultPriority in board.ts. The config loader refuses such
    a set so neither branch is reachable, but these are two implementations of
    one rule and that is exactly how they drift.
  - README says where the stragglers are after a vocabulary edit: they sort last,
    with the old value still in the column, since --priority cannot filter for a
    value the project no longer declares.

  Kept as-is: the guide's examples use the most severe level rather than the
  default, because it is the only choice guaranteed to exist that also
  demonstrates the flag doing something.

  Verified: make test (also -count=1 and -race), test-integration, test-frontend
  (110), test-browser (31, real Chromium), lint (0 issues), build. The reviewer
  separately ran tsc --strict out-of-tree over the frontend (clean, this project
  has no typechecker yet — TQ-0024) and diffed a fresh Bun build against the
  committed internal/web/public/ (identical).

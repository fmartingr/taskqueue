---
id: TQ-0102
title: A quick start that teaches the workflow, in the README and after tq init
status: done
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-31T17:18:07+02:00
updated: 2026-08-31T17:26:05+02:00
---

## Why

A new user's first two encounters with tq are the README's Quick start and
whatever `tq init` prints. Both currently show *commands*; neither teaches the
*workflow* — that a task is claimed before the first edit, noted as it goes,
and closed when the work is verified, and that the CLI and the board are two
views of the same Markdown files.

`.tasks/AGENTS.md` already teaches that, but it is written for agents and is
only reached by someone who opens the file `tq init` names.

## Current state

`README.md` "Quick start" (line 47) is a ten-line block of commands with no
narrative: `init`, two `add`s, `ready`, `move`, `note`, `done`, `serve`. The
prose under it is entirely about `tq init` — that it is mandatory, where it
creates the queue, and to re-run it after editing `.taskqueue.yaml` — so the
loop the commands sketch is never explained.

`runInit` in `internal/cli/cli.go` (the tail of the function) prints what it
wrote, `tq serve` and its address, and the guide's path with a line about
including it in an agent context file. It says nothing about creating or
working a task, so the next step after `tq init` is only obvious to someone
who already read the README.

## Scope

**README** — turn Quick start into a short guided tour a first-time reader can
follow end to end:

- The mental model in a sentence or two: Markdown files in `.tasks/`, the CLI
  and the board over the same files, committed with the code.
- The loop, named: `add` → `ready` → `move in-progress` → `note` → `done`, with
  a line each on *why* a step exists (claim before editing, note as you go,
  close when verified), not just what it types.
- Where dependencies fit, since `tq ready` is meaningless without them.
- The board as the other view of the same tasks, and the search box as the way
  to narrow it.
- Keep the existing `tq init` prose — mandatory, no searching, re-run after a
  `.taskqueue.yaml` edit — but stop it being the whole section.

**`tq init`** — end with next steps rather than only the guide's path:

- The first task: an `tq add` line the reader can paste.
- `tq ready` / `tq move` / `tq done` as the loop, in two or three lines.
- Keep `tq serve` and its address, and keep the guide path and the
  "include it in your agent context file" line — that one is doing real work.
- Say it in the same voice as the rest of the command's output, and keep it
  short: this prints on every re-run of an idempotent command, so it must not
  become a wall of text someone re-initialising a queue has to scroll past.
- `--json` output is a stable contract: the next-steps text is human output
  only and must not change the JSON shape.

## Constraints

- Do not duplicate `.tasks/AGENTS.md` into either place. That guide is
  generated from `internal/guide/AGENTS.tmpl.md` and is the agent's reference;
  the README is the human's and `tq init`'s output is a signpost.
- The README's other sections (The human workflow, The agent workflow, CLI
  reference) already cover the detail — the quick start links to them rather
  than restating them.
- Statuses, priorities and the default column are the project's, declared in
  `.taskqueue.yaml`. Anything `tq init` prints about them has to come from the
  board it just resolved, the way the `add` command's status flag help already
  does, rather than being hardcoded.

## Acceptance

- A reader who follows the README's Quick start alone creates a task, finds it
  with `tq ready`, works it through `in-progress` to `done`, and opens the
  board — without needing another section.
- `tq init` on a fresh directory prints next steps that get someone to their
  first task without opening the README.
- `tq init --json` output is unchanged.
- CLI output tests cover the new lines.

---

## Notes

- 2026-08-31T17:26:05+02:00 — Done. Two changes:

  - README: Quick start is now four numbered steps — create the queue, work one task through the loop, let dependencies order the work, open the board — each with the *why* (inbox is intake, claim before the first edit, note as you go, close when verified) and links out to The human workflow, The agent workflow and Columns rather than restating them. Every command block was run end to end against the built binary; the dependency example uses --status todo so tq ready demonstrably shows one task before the dependency is closed and the other after.
  - tq init: new printNextSteps prints the loop after what it wrote and before the tq serve line. It reads the board through st.Columns(), so a project's own column names are used: the queueing step is dropped when the default column already offers work, and the tq done step is dropped when no column is consider_done (the same condition tq done itself errors on). The claim step is not spelled as a command — no column flag says which column holds claimed work — so it is named in the sentence under the list.

  --json is untouched: the JSON return is above this, and internal/integration/cli_contract_test.go already asserts stdout is JSON alone for init --json.

  Tests: three in internal/cli/cli_test.go — the built-in board's steps (including that TQ-0001 is really the first ID handed out, so the lines paste in order), the custom board's, and the board that can take neither the queueing nor the done step.

  make test, go vet, make build and make test-integration pass. internal/web's TestAScanFailureIsReportedAgainToAFreshBoard fails, but it fails the same way on a clean checkout of this commit — unrelated to this task. golangci-lint is not installed here, so make lint could not run.

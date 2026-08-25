---
id: TQ-0044
title: Document that agents edit a task body by editing the file directly
status: todo
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-25T13:45:10+02:00
updated: 2026-08-25T18:42:55+02:00
---

## Finding

The guide in `.tasks/AGENTS.md` opens with:

    Use the CLI rather than editing files by hand — it validates, keeps
    timestamps and filenames in sync, and writes atomically.

Read literally, that forbids the only way to revise a task body. `tq update`
covers title, status, priority, assignee, labels and dependencies; there is no
`--body`, and `tq note` only appends to the notes section. An agent asked to
correct a ticket's Finding or Suggested fix has no sanctioned move.

## How it fails

Observed while working TQ-0041: revising that ticket's own Suggested fix meant
editing `.tasks/TQ-0041-*.md` by hand, in direct contradiction of the guide the
same task was rewriting. The edit was fine — bodies are free text, and
`tq show` read it straight back — but nothing in the documentation says so, so
the next agent either does it and feels it is cheating, or does not do it and
leaves the ticket wrong.

## Suggested fix

In `agents.go`, narrow the claim to what it is actually about: the CLI owns
frontmatter — IDs, statuses, timestamps, filenames — and hand edits there are
what break things. Say plainly that the body is edited by editing the file, and
that the ID in the frontmatter is what identifies the task, so the filename
must not be renamed to match a new title (see the existing rule in the root
`AGENTS.md`).

Then regenerate `.tasks/AGENTS.md`, which is where the workflow lives; the root
`AGENTS.md` keeps only its minimal pointer.

Alternatively, close the gap in the CLI instead by giving `tq update` a
`--body` flag, and document that. Editing the file is the smaller change and
matches how bodies are already authored, but the CLI route keeps the "use the
CLI" rule whole.

---

## Notes

- 2026-08-25T13:53:59+02:00 — Line one. - bullet a - bullet b
- 2026-08-25T18:42:55+02:00 — Once TQ-0029 lands the guide should also point at .taskqueue.yaml and explain path:, since that is where an agent learns which directory holds the tasks.

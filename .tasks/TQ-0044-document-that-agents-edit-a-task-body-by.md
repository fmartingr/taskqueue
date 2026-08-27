---
id: TQ-0044
title: Document that agents edit a task body by editing the file directly
status: done
priority: normal
labels:
  - docs
  - component/cli
created: 2026-08-25T13:45:10+02:00
updated: 2026-08-28T00:00:39+02:00
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
- 2026-08-27T23:40:45+02:00 — Took the CLI route the ticket offered second: `tq update --body`, so the guide's "use the CLI" rule stays whole rather than being narrowed into an exception for bodies.

  --body replaces the CONTENT half and keeps the notes. Wholesale replacement was rejected: the notes are appended one at a time and reconstructible from nothing, while the content is a document its author has in front of them. task.ReplaceContent reuses notesSection, so the split is the one internal/task already owns — no second implementation of the rule.

  It rides on a new TaskPatch.Content field (json:"-"), applied through Store.Patch/Mutate, so the read-modify-write of the notes happens under the store's lock. TaskPatch.Body is untouched and still replaces the whole body: that is what the board's dialog sends, and it has the notes on screen.

  Did do stdin: --body - reads standard input, on update and on add alike, so one flag does not mean two things. cli.Run now takes an io.Reader.
- 2026-08-28T00:00:18+02:00 — Code review at effort high found two confirmed defects in this diff, both fixed here.

  1. ReplaceContent duplicated the record on a round trip. tq show hands out the WHOLE body, notes included, so the natural revision — read it, edit it, hand it back — passed the notes straight back in and got the file's appended to them: one pass gave two copies, the next three. The replacement is now read as a body and only its content half is kept, which makes the loop the guide describes converge. Notes are still never settable this way; tq note appends them and the file keeps them.

  2. --body - read stdin inside holdSignals, whose whole trade is that a write command returns in milliseconds. A pipe nobody closes never ends, so SIGINT was held and only SIGKILL got out. The read now happens before the hold, from an argv scan in runCLI. Pinned by an integration test that was checked to fail without the fix (10s hang) and pass with it.

  Out of scope and left alone, reported upward: the guide's hardcoded ready rule and in-progress claim step (already on the deferred list); tq help calling done a shorthand for move <id> done on a custom board; config.Path is unvalidated so a committed marker with path: .. makes tq init write outside the repo; validateServer running on every store read; the events.go subscribe/close race; and four frontend defects. None are in this diff.

---
id: TQ-0020
title: Notes are appended in the wrong place when a heading follows
status: todo
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:43:09+02:00
---

## Finding

AppendNote terminates the "## Notes" section only on a level-2 heading (`if strings.HasPrefix(strings.TrimSpace(lines[i]), "## ")`), so a level-1 `# ` heading — which must end a level-2 section — and a `### ` sub-heading do not stop the scan, and the note is appended at the very bottom of the body under the wrong heading.

Source: `task.go:215`

## How it fails

Reproduced with body `## Notes` / `- old note` / `# Appendix` / `text`: `tq note TQ-0001 "new note"` (and POST /api/tasks/{id}/notes) appends the bullet after `text`, inside `# Appendix`. Every subsequent note piles up there too, so an agent's progress log silently detaches from the Notes section the board reads — splitBody (app.ts:138) counts notes by section, so the card's note badge is wrong from then on. The same single-prefix test also matches a `## Notes` line inside a fenced code block in the body, appending to the end of the document and never creating a real Notes section. agents.go:158 gets the heading-depth comparison right (`depth > 0 && depth <= hashes`); task.go does not.

## Suggested fix

End the Notes section at any heading of level 2 or shallower, not only at `## `.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T12:43:09+02:00 — Fixed by TQ-0031: notes are now the body's last section, so AppendNote no longer scans for the first '## Notes' at all — a heading of any level after it, and one inside a fenced code block, both make it content. task.go:notesStart covers the level-1/level-3 and fenced cases this ticket reported, with tests in TestAppendNote and TestNotesSection.

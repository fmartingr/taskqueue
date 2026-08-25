---
id: TQ-0031
title: Notes section must be last and introduced by a horizontal rule
status: todo
priority: urgent
labels:
  - data-loss
  - frontend
  - cli
created: 2026-08-25T11:57:34+02:00
updated: 2026-08-25T11:57:34+02:00
---

## Symptom

A task whose *content* contains a `## Notes` heading — TQ-0029 does — has that
section torn out of the body and rendered as if it were timestamped notes, and
saving the task from the board then corrupts the file.

Reproduced against a copy of TQ-0029:

- The dialog's body editor stops at `## Contract`; the prose under `## Notes`
  disappears from it, and `## Acceptance criteria` disappears too (it becomes
  the hidden `trailing` part).
- The notes panel lists **one note per wrapped line**: "YAML gotcha for whoever
  implements the labels ticket: `color: #ff0000` is a" / "**comment** in YAML,
  so that value parses as null. Hex colours have to be" / "quoted, and the
  loader should reject an empty colour rather than render it." Each continuation
  line of a hard-wrapped bullet becomes its own entry.
- Pressing **Save** without touching anything rewrites the file: every
  continuation line comes back as its own top-level bullet.

```diff
-  **comment** in YAML, so that value parses as null. Hex colours have to be
-  quoted, and the loader should reject an empty colour rather than render it.
+- **comment** in YAML, so that value parses as null. Hex colours have to be
+- quoted, and the loader should reject an empty colour rather than render it.
```

This is not frontend-only: `tq note TQ-0029 "..."` appends its bullet into that
prose section too, in the middle of the document, above `## Acceptance
criteria`.

## Cause

`splitBody` (frontend/app.ts) and `AppendNote` (task.go) both identify the notes
block as *the first `## Notes` heading anywhere in the body*, and the frontend's
`parseNote` treats every non-empty line in it as a note.

## Fix

Notes are the **last** section of the body and are introduced by a horizontal
rule, so a `## Notes` heading in ordinary content is just content:

```markdown
Task content, which may itself contain a Notes section.

---

## Notes

- 2026-08-25T09:42:00+02:00 — the actual note
```

- The blank line before the `---` is required: `text` immediately followed by
  `---` is a setext level-2 heading in Markdown, not a rule.
- A `## Notes` heading counts as the notes block only when it is preceded by
  that rule and nothing but notes follows it. Otherwise it is content.
- `AppendNote` writes the rule when it creates the section, and always at the
  very end of the body.
- The board's `splitBody`/`joinBody` follow the same rule and round-trip it
  unchanged, and `parseNote` keeps continuation lines attached to their bullet
  instead of promoting each line to a note.

Verified that this is safe to store: a `---` line inside the body survives
ParseTask/RenderTask, because the frontmatter scan stops at the real closing
delimiter before it. Worth a regression test, and worth keeping in mind
alongside TQ-0007, which is about an *indented* `---` produced by a multi-line
title.

## Migration

Every task written by `tq note` so far has a plain trailing `## Notes` with no
rule. Treat a trailing `## Notes` section as notes when reading, and write the
rule in on the next update. Do **not** rewrite a `## Notes` that is followed by
other sections — that is content, which is exactly the TQ-0029 case.

## Acceptance criteria

- TQ-0029 renders with its full body in the editor, no notes in the panel, and
  Save leaves the file byte-identical.
- `tq note` on TQ-0029 appends at the end of the document, under a new rule.
- A task with real notes keeps working on both surfaces, and existing files
  without the rule are read correctly and upgraded when next written.
- Round-trip test: body containing a `## Notes` heading and a `---` rule, parsed
  and rendered, comes back unchanged.

---
id: TQ-0007
title: Multi-line titles write task files tq cannot read back
status: done
priority: urgent
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T17:52:37+02:00
---

## Finding

The closing-delimiter scan trims each line before comparing (`if strings.TrimSpace(lines[i]) == fmDelimiter {`), so an indented `---` inside a YAML block scalar ends the frontmatter early — and RenderTask emits exactly such a file for any multi-line title, so tq writes task files it can never read back.

Source: `frontmatter.go:32`

## How it fails

Reproduced: `tq add "Fix the parser"` then `tq update TQ-0001 --title "$(printf 'line1\n---\nline2')"` prints `Updated TQ-0001` and exits 0. yaml.v3 emits `title: |-` / `  line1` / `  ---` / `  line2`; on the next read the scan stops at the indented `  ---`, so status/created/updated leak into the body and `tq list` dies with `error: invalid task file: TQ-0001-line1-line2.md: status is required` (exit 1) for the WHOLE directory. Store.Update already removed the old file at store.go:340, so the only copy is the corrupted one — unrecoverable loss from a command that reported success. Equally reachable via PATCH /api/tasks/{id}; server.go:158 does no newline validation.

## Suggested fix

Accept only an unindented `---` as the closing delimiter, and reject newlines in a title during validation so the file can never be written in the first place.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T17:52:36+02:00 — Two fixes, as the ticket asked. ParseTask now closes the frontmatter only on an unindented delimiter, so an indented --- inside a YAML block scalar is data. Validation refuses a line break in a title, assignee or label, so such a file is never written.
- 2026-08-25T17:52:36+02:00 — The two fixes conflicted at first: a tolerant parser is useless if ParseTask then rejects the same file. Split them — Validate stays the read-time contract, and a new ValidateForWrite adds the single-line rules for RenderTask and Store.Update.
- 2026-08-25T17:52:37+02:00 — That split buys recovery, which neither fix alone gives. A file already corrupted by the old tq now loads: tq list works and shows the broken title, and tq update repairs it. Before, one such file made tq list fail for the whole directory.
- 2026-08-25T17:52:37+02:00 — Correction to the ticket: the claim that Store.Update had already removed the old file is stale. TQ-0006 changed it to write the new file first and retire the old one only after, so the loss was that the surviving copy was unreadable, not that no copy existed.
- 2026-08-25T17:52:37+02:00 — Verified: the reproduction now exits 1 with 'title must be a single line' and leaves the task intact, and the same title over PATCH /api/tasks/{id} returns 400 with the task unchanged. All three fixes are mutation-checked.

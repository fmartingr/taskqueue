---
id: TQ-0007
title: Multi-line titles write task files tq cannot read back
status: todo
priority: urgent
labels:
  - review
  - data-loss
  - store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T11:30:21+02:00
---

## Finding

The closing-delimiter scan trims each line before comparing (`if strings.TrimSpace(lines[i]) == fmDelimiter {`), so an indented `---` inside a YAML block scalar ends the frontmatter early — and RenderTask emits exactly such a file for any multi-line title, so tq writes task files it can never read back.

Source: `frontmatter.go:32`

## How it fails

Reproduced: `tq add "Fix the parser"` then `tq update TQ-0001 --title "$(printf 'line1\n---\nline2')"` prints `Updated TQ-0001` and exits 0. yaml.v3 emits `title: |-` / `  line1` / `  ---` / `  line2`; on the next read the scan stops at the indented `  ---`, so status/created/updated leak into the body and `tq list` dies with `error: invalid task file: TQ-0001-line1-line2.md: status is required` (exit 1) for the WHOLE directory. Store.Update already removed the old file at store.go:340, so the only copy is the corrupted one — unrecoverable loss from a command that reported success. Equally reachable via PATCH /api/tasks/{id}; server.go:158 does no newline validation.

## Suggested fix

Accept only an unindented `---` as the closing delimiter, and reject newlines in a title during validation so the file can never be written in the first place.

Filed from a `/code-review` pass at max effort.

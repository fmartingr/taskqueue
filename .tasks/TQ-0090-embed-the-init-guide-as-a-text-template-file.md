---
id: TQ-0090
title: Embed the init guide as a text/template file
status: done
priority: normal
labels:
  - component/cli
created: 2026-08-28T11:10:25+02:00
updated: 2026-08-28T11:24:42+02:00
---

## Finding

The agent guide `tq init` writes is a giant `fmt.Appendf` string in `internal/guide/agents.go`. Editing the prose means fighting Go string concatenation around backticks, and the document is not a document until it is generated.

## Suggested fix

Keep the guide as a regular Markdown file next to the package, embed it with `go:embed`, and fill project-specific values with `text/template`. Change the guide by editing that file; `tq init` still writes `.tasks/AGENTS.md`.

---

## Notes

- 2026-08-28T11:12:10+02:00 — The guide is now internal/guide/AGENTS.md, embedded and filled with text/template. missingkey=error so a renamed field fails the write. TestTheCommittedGuideIsCurrent still matches the committed file byte for byte. Change the template, then tq init.
- 2026-08-28T11:24:42+02:00 — Renamed the source to AGENTS.tmpl.md so agents walking the tree do not take the unfilled template as instructions.

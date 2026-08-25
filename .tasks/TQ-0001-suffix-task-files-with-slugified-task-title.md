---
id: TQ-0001
title: Suffix task files with slugified task title
status: done
priority: normal
created: 2026-08-25T10:24:04+02:00
updated: 2026-08-25T10:30:08+02:00
---

## Proposal

Add a suffix to task titles so we can easily find them using editors or commands in the CLI by filename.

For example, TQ-0001 with title "Suffix tasks" would be named `TQ-0001-suffix-tasks.md`.

## Notes

- 2026-08-25T10:29:39+02:00 — Implemented: files are now named <id>-<slug>.md, with lookups by ID and renames on retitle.

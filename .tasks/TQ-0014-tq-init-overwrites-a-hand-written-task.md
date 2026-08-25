---
id: TQ-0014
title: tq init overwrites a hand-written Task management section
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

`tq init` replaces any section titled "Task management" in the repo's own AGENTS.md/CLAUDE.md with a three-line stub, with no check that tq authored it (the generatedNotice marker is never consulted for root docs), no warning, and a non-atomic os.WriteFile (agents.go:110).

Source: `agents.go:135`

## How it fails

I reproduced it. An AGENTS.md containing `## Task management` / `We track work in Jira. Never file tickets here.` / `See the runbook at docs/process.md...` / `## Build` became, after one `tq init`: `## Task management` / `See [AGENTS.md](.tasks/AGENTS.md)` / `## Build` — the human-written policy is destroyed in place with no backup and no prompt, and the CLI reports only `Wrote /.../AGENTS.md`. The blank line before the next heading is eaten too (lines[end:] is spliced on directly while the separator at end-1 sits inside the replaced range). findSection (agents.go:147-165) has no fenced-code-block awareness, so a doc that merely shows `## Task management` inside a ``` fence loses the closing fence AND everything to end of file. Note the asymmetry: store.write goes to real trouble (CreateTemp/Chmod/Sync/Rename) for task files, while the file with the highest blast radius — one tq did not author — gets a bare truncating WriteFile.

## Suggested fix

Rewrite a section only when tq authored it (look for the generated marker or an existing link into the task directory); otherwise leave it alone and say so. Write the file atomically.

Filed from a `/code-review` pass at max effort.

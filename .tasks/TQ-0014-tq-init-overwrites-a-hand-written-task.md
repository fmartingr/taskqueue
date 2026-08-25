---
id: TQ-0014
title: tq init overwrites a hand-written Task management section
status: done
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T16:09:21+02:00
---

## Finding

`tq init` replaces any section titled "Task management" in the repo's own AGENTS.md/CLAUDE.md with a three-line stub, with no check that tq authored it (the generatedNotice marker is never consulted for root docs), no warning, and a non-atomic os.WriteFile (agents.go:110).

Source: `agents.go:135`

## How it fails

I reproduced it. An AGENTS.md containing `## Task management` / `We track work in Jira. Never file tickets here.` / `See the runbook at docs/process.md...` / `## Build` became, after one `tq init`: `## Task management` / `See [AGENTS.md](.tasks/AGENTS.md)` / `## Build` — the human-written policy is destroyed in place with no backup and no prompt, and the CLI reports only `Wrote /.../AGENTS.md`. The blank line before the next heading is eaten too (lines[end:] is spliced on directly while the separator at end-1 sits inside the replaced range). findSection (agents.go:147-165) has no fenced-code-block awareness, so a doc that merely shows `## Task management` inside a ``` fence loses the closing fence AND everything to end of file. Note the asymmetry: store.write goes to real trouble (CreateTemp/Chmod/Sync/Rename) for task files, while the file with the highest blast radius — one tq did not author — gets a bare truncating WriteFile.

## Suggested fix

Rewrite a section only when tq authored it (look for the generated marker or an existing link into the task directory); otherwise leave it alone and say so. Write the file atomically.

Filed from a `/code-review` pass at max effort.

---

## Notes

- 2026-08-25T14:47:28+02:00 — Widened by TQ-0045 (commit 004aa72), confirmed by /code-review: the pointer guard was narrowed from the whole document to only the lines inside the found section, so a document that links to the guide from anywhere else — an intro paragraph, a reference-style link, a ./-prefixed path — now falls through to the rewrite branch and loses the entire body of a hand-written Task management section. Those documents were protected by the old whole-document guard and are not now. Recording here rather than as a new ticket, since this ticket already covers the shape.
- 2026-08-25T16:09:20+02:00 — Fixed. withTaskSection now rewrites a Task management section only when it holds nothing but the stub tq writes — the generated See [AGENTS.md](...) line whatever it points at, or an @-include. Anything else is a person's and the document is left untouched.
- 2026-08-25T16:09:20+02:00 — SyncAgentsDocs returns a SyncReport with Written and Skipped, and tq init names the skipped files on stderr so the refusal is visible without polluting --json stdout. The JSON gains a skipped key alongside written, which is additive and keeps the agent API compatible.
- 2026-08-25T16:09:20+02:00 — Two other halves of the ticket: the blank line before the following heading is put back when the section is spliced, and root docs are now written through writeAtomic, the same CreateTemp/Chmod/Sync/Rename dance Store.write already used for task files. The fenced-block half was already fixed by TQ-0045.
- 2026-08-25T16:09:20+02:00 — Deliberate contract change: TQ-0045 asserted that a fence inside the section is destroyed with the rest of it, since the section was regenerated wholesale. A fence is something a person wrote, so it now makes the section off limits instead. That test was rewritten to assert the document is left untouched.

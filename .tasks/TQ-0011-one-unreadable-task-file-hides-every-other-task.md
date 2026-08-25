---
id: TQ-0011
title: One unreadable task file hides every other task
status: todo
priority: normal
labels:
  - bug
  - component/store
created: 2026-08-25T11:30:21+02:00
updated: 2026-08-25T12:19:31+02:00
---

## Finding

Store.List aborts the whole scan on the first unreadable file (`return nil, err` inside the per-entry loop), and ParseTask rejects any extra frontmatter key (frontmatter.go:43 `dec.KnownFields(true)`) or a UTF-8 BOM, so one hand-edited or merge-conflicted file hides every other task on both surfaces.

Source: `store.go:245`

## How it fails

Reproduced with two healthy tasks plus one file carrying an extra `epic: platform` key: `tq list` -> exit 1 with no tasks printed, `tq ready` -> exit 1, `GET /api/tasks` -> HTTP 500, `GET /api/status` -> 500 (the board loses even its task_dir line), while `tq show TQ-0001` still works. Git conflict markers and a BOM (strings.TrimSpace does not strip U+FEFF, so the error misleadingly says the file does not start with `---`) produce the same outage. Two agents each `tq add` on their own branch is the ordinary path to a conflicted .tasks/ — a directory the generated guide tells users to commit. Rejecting unknown keys is defensible; taking the whole queue down instead of skipping one file is not.

## Suggested fix

Decide the policy deliberately: either skip unreadable files and report them alongside the good ones, or keep failing fast (today's behaviour, chosen so a broken file cannot be missed). `describeDeps` already degrades gracefully, so the two paths disagree.

Filed from a `/code-review` pass at max effort.

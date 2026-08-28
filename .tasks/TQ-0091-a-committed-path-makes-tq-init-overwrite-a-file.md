---
id: TQ-0091
title: 'A committed path: .. makes tq init overwrite a file outside the project'
status: todo
priority: urgent
labels:
  - bug
  - security
  - component/config
created: 2026-08-28T12:57:31+02:00
updated: 2026-08-28T12:57:31+02:00
---

## Finding

`Config.Path` is never validated. A `.taskqueue.yaml` is committed and travels
with the repository, so a `path:` that escapes its own directory is under the
control of whoever wrote the repo, not whoever runs `tq`.

    version: 1
    path: ..

`tq init` then resolves the task directory to the parent of the marker's
directory and writes its guide there — overwriting `../AGENTS.md`, exit 0, no
warning.

## Why this is not theoretical

The attack is: clone an untrusted repository and run the first command the
README tells you to run. `tq init` is documented as **mandatory** and is the
opening step of the quick start, so a user following the instructions correctly
is the delivery mechanism.

`path:` escaping the marker's directory is a *supported* feature — TQ-0087 made
it work deliberately, and `tq init` under `TQ_CONFIG_PATH` writes such a marker
itself. So the fix is not to forbid the shape; it is to stop an arbitrary
committed value deciding which files tq writes.

Source: `internal/config/config.go:85`.

## Suggested fix

Decide what a marker may point at. Candidates, not exclusive:

- Refuse a `path:` that resolves outside the marker's own directory **unless**
  the user opted in for this invocation (an env var or a flag), so a committed
  file cannot make the choice on its own.
- Refuse to write the guide anywhere the task directory does not contain.
- Name the resolved task directory before writing anything, so an unexpected
  destination is visible rather than silent.

Whatever is chosen, `tq init` must not write outside the tree the user invoked
it in without saying so.

## Acceptance

- A cloned repository whose marker says `path: ..` cannot make `tq init`
  overwrite a file above the project without an explicit opt-in.
- The legitimate escaping-`path:` case TQ-0087 established still works.
- A test that fails if the guard is removed.

Found by the code review during TQ-0044; not independently reproduced yet —
**confirm the reproduction before fixing.**

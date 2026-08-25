---
id: TQ-0060
title: tq init exit code drifted from 1 to 3 when .tasks is unusable
status: todo
priority: high
labels:
  - bug
  - component/cli
created: 2026-08-25T17:44:51+02:00
updated: 2026-08-25T18:42:55+02:00
---

## Finding

`runInit` now resolves through `OpenStore`, which wraps every `InitStore`
failure as `ErrProjectNotFound`. That maps to exit 3 where the bare error
mapped to exit 1.

## How it fails

Reported by `/code-review` from an A/B against a binary built at bc9b1f6, not
re-verified here. With `.tasks` present as a regular file:

    now:    error: no .tasks directory found: <repo>/.tasks exists and is not a directory   (exit 3)
    before: error: <repo>/.tasks exists and is not a directory                              (exit 1)

The message also contradicts itself: it says the directory was not found, then
says it exists. The root `AGENTS.md` requires exit codes to stay stable, and
`cli.go` repeats that they are part of the agent-facing contract.
`TestCLIReportsUncreatableTaskDir` covers list, add and ready, never init.

## Suggested fix

Decide which code is right — 3 is arguably correct for "no usable task
directory" — then make the message agree with it and pin init with a test.
Whichever way it goes, record the decision, because this is a contract.

Found by `/code-review` over 20b06d2.

---

## Notes

- 2026-08-25T18:42:55+02:00 — tq init will also write .taskqueue.yaml (TQ-0029), which adds a second failure to classify: an unusable task directory and an unwritable config. Settle the exit-code mapping for both in one go.

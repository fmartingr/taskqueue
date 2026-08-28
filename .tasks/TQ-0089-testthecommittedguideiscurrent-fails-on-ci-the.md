---
id: TQ-0089
title: 'TestTheCommittedGuideIsCurrent fails on CI: the guide embeds the checkout path'
status: done
priority: normal
labels:
  - component/cli
created: 2026-08-28T10:19:30+02:00
updated: 2026-08-28T11:07:44+02:00
---

## Finding

`make test` fails on GitHub Actions in `TestTheCommittedGuideIsCurrent`:

    .tasks/AGENTS.md is stale: run `tq init` … and commit the result

It passes on a developer machine. The generator prints the resolved task directory under "Where the tasks live", and the committed file holds whoever last ran `tq init`'s absolute path. The test regenerates with `cfg.TaskDir()`, which is a different absolute path on CI, so the files never match there. Regenerating and committing cannot fix it: the next runner has another path.

The task directory is already declared in `.taskqueue.yaml` as `path`. That is the value the guide should print. Masking the checkout path in the comparison leaves the home directory in a committed file.

## Suggested fix

Pass the marker's `path` into `taskGuide`, not `Store.Dir`. The ls/grep examples and the "Where the tasks live" block then name the same string the config file stores, which is portable. `TestTheCommittedGuideIsCurrent` compares against `cfg.Path`. A write of the guide must not contain the absolute checkout path.

---

## Notes

- 2026-08-28T10:20:08+02:00 — The test now masks the indented path under Where the tasks live before comparing. TestMaskTaskDirIgnoresTheCheckoutPath pins two generators that differ only in that path. Regenerating the committed file cannot fix CI: the next runner has another path.
- 2026-08-28T11:07:44+02:00 — The generator now prints the marker's path, not Store.Dir. The committed guide says .tasks, matching .taskqueue.yaml. The comparison in TestTheCommittedGuideIsCurrent uses cfg.Path; a write of the guide must not contain the absolute checkout path. Masking is gone.

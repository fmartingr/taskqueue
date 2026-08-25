---
id: TQ-0002
title: '`tq init` should handle AGENTS.md'
status: done
priority: normal
labels:
  - component/cli
  - feature
created: 2026-08-25T10:33:50+02:00
updated: 2026-08-25T16:19:26+02:00
---

## Proposal

`tq init` should make sure we have an up to date `AGENTS.md` in the tasks folder that let the agent know how to handle tasks by having a concise list of CLI commands for listing, searching, retrieving, etc.

It can also make sure the `AGENTS.md`/`CLAUDE.md` on the repository contains a section pointing to the tasks folder AGENTS.md file like so:

```md
# Task management

See [AGENTS.md](.tasks/AGENTS.md)
```

Ensure the path to the tasks folder is the correct one by retrieving it from the settings.

---

## Notes

- 2026-08-25T16:19:26+02:00 — Reverted by TQ-0055. The feature this ticket added — tq init pointing the repository's AGENTS.md/CLAUDE.md at the guide — is removed. Editing a document tq did not author produced seven separate defects (TQ-0014, TQ-0042, TQ-0043, TQ-0045, TQ-0046, TQ-0049, TQ-0052), and the value was one line a person can write once. tq init now prints that line instead.

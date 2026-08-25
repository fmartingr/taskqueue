---
id: TQ-0002
title: '`tq init` should handle AGENTS.md'
status: done
priority: normal
created: 2026-08-25T10:33:50+02:00
updated: 2026-08-25T10:42:46+02:00
---

## Proposal

`tq init` should make sure we have an up to date `AGENTS.md` in the tasks folder that let the agent know how to handle tasks by having a concise list of CLI commands for listing, searching, retrieving, etc.

It can also make sure the `AGENTS.md`/`CLAUDE.md` on the repository contains a section pointing to the tasks folder AGENTS.md file like so:

```md
# Task management

See [AGENTS.md](.tasks/AGENTS.md)
```

Ensure the path to the tasks folder is the correct one by retrieving it from the settings.

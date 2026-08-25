---
id: TQ-0005
title: min-height is not required for columns
status: done
priority: normal
labels:
  - bug
  - component/frontend
created: 2026-08-25T10:52:50+02:00
updated: 2026-08-25T12:19:31+02:00
---

The min-height values in the column-tasks and columns themselves are leaving weird gaps when columns are empty. We can safely remove those so columns have a natural min-height with the "add a card" button.

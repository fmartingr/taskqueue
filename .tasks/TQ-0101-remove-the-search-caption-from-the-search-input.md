---
id: TQ-0101
title: Remove the Search caption from the search input
status: done
priority: normal
labels:
  - component/frontend
created: 2026-08-29T10:09:15+02:00
updated: 2026-08-29T10:11:23+02:00
---

The search box in the top bar still carries a visible Search label above the input. The placeholder already says what the line is for, and the caption makes the bar taller than the brand and the New task button for no remaining reason.

Drop the visible label. Keep an accessible name on the input so the box is still named for assistive tech.

---

## Notes

- 2026-08-29T10:11:23+02:00 — Dropped the visible Search caption above the input. The box keeps aria-label="Search" so it is still named for assistive tech. The top bar now aligns to the centre, since the caption was the only reason it used flex-end.

  No new test, as asked.

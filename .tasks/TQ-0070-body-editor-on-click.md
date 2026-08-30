---
id: TQ-0070
title: Body editor on click
status: done
priority: normal
depends_on:
  - TQ-0076
created: 2026-08-25T18:37:55+02:00
updated: 2026-08-30T20:49:00+02:00
---

The task body should be shown rendered rather than as a textarea, and clicking
it should open the editor — the same click-to-edit the title got.

Delivered as part of TQ-0069, which drew the body this way in its own mock and
could not have been built any other way: a dialog that holds no draft of its
task has to render the body from the file, and an editor you open deliberately
is what replaces the textarea that was always open.

---

## Notes

- 2026-08-26T10:31:16+02:00 — Blocked on TQ-0076 (migrate the board to Vue 3). TQ-0076's own note names this ticket: built against app.ts it is imperative DOM that gets ported again; built after, it is a component.
- 2026-08-30T20:49:00+02:00 — Closed as done, delivered by TQ-0069 rather than on its own.

  What landed:

  - frontend/markdown.ts renders the body. Hand-written rather than a dependency:
    it escapes the document before it emits a tag, so there is nothing left from
    the file to sanitise, and public/app.js is committed so a library would be a
    permanent block in every diff of it. Every way a body could try to become
    markup has a case in markdown.test.ts.
  - frontend/components/Markdown.vue is the only place in the board that puts
    built HTML into the page.
  - frontend/components/InlineText.vue is the click-to-edit control, shared with
    the title and the assignee; the body is its multiline form, with the rendered
    Markdown as the display and a textarea plus Save/Cancel as the editor.
  - Clicking #task-body opens #task-body-edit on the text the file holds. Save or
    losing focus writes it, Escape drops it, and a paragraph both sides rewrote is
    refused rather than merged (frontend/edit.ts, commitContent).

  Nothing in the ticket is left over: it had no body of its own beyond the title,
  and the title is satisfied literally.

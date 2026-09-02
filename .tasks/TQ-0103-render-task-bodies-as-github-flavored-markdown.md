---
id: TQ-0103
title: Render task bodies as GitHub Flavored Markdown
status: done
priority: normal
labels:
  - component/frontend
  - feature
created: 2026-09-02T21:42:31+02:00
updated: 2026-09-02T22:39:21+02:00
---

The board renders a task body as GitHub Flavored Markdown (GFM), through
markdown-it. A task body is written in a GitHub editor as often as in this
board, so the two agree on what one means.

`frontend/markdown.ts` is the configuration and the extensions, not a parser.

## Options that are load-bearing

- `html: false`. Raw HTML in a body is text. GFM passes a limited set of tags
  through; this refuses, which is what removes the need for a sanitiser after
  the renderer.
- `breaks: true`. A soft line break is a `<br>`, where GFM folds it into a
  space. Task bodies are hand-wrapped findings as often as they are prose.
- `linkify` with `fuzzyLink` on, for the bare `www.` host GFM links and
  markdown-it does not.

## Extensions

- `validateLink` is replaced outright. markdown-it's own check is a deny-list;
  a scheme is on the allow-list — `http`, `https`, `mailto`, or relative — or
  the link stays the text it was written as.
- A task list item draws its checkbox through a token of ours. GFM has task
  lists and markdown-it does not.
- A table is wrapped in a box that scrolls, so a wide one cannot widen the
  dialog.
- Every link leaves this tab, and every image loads lazily.

## What this closed

Tables, autolinks in angle brackets, the `www.` and email autolink extension,
backslash escapes, setext headings, closed ATX headings, character references,
indented code blocks, and loose lists. Each was a gap against GFM in the
hand-written renderer that came before.

## Non-goal

Raw HTML stays unsupported, and that is `html: false` above.

## Done when

- `make test-frontend` covers the dialect, each extension, and every way a body
  could try to become markup.
- `make typecheck` and `make frontend` pass, and `internal/web/public/` is
  committed.
- `make test-browser` passes, with a case for the one thing only a browser can
  show: a table too wide for the dialog scrolls inside its own box.

---

## Notes

- 2026-09-02T22:39:09+02:00 — Two passes. The first closed all nine GFM gaps by hand: the renderer grew from 341 to 721 lines, cost app.js 7.7KB, and passed 57 unit tests plus the browser suite.

  Felipe then asked whether a library with an extension API would do the job instead, and chose markdown-it over marked. The numbers behind that: markdown-it costs app.js 190KB unminified (265KB to 455KB) and needs no sanitiser, since html:false makes a tag in a body text; marked would have been 40KB but emits raw HTML from the document and dropped its sanitiser in v5, so it would have pulled in DOMPurify and a DOM, ending the DOM-free bun tests.

  markdown-it 15 defaults fuzzyLink off, so a bare www. host needed it turned back on to match GFM. Everything else in the dialect was default. It is pinned exactly, like vue, because app.js is committed.

/**
 * The Markdown a task body is rendered with, in the dialog.
 *
 * The dialect is GitHub Flavored Markdown, and markdown-it is what reads it.
 * A task body is written in a GitHub editor as often as in this board, so the
 * two have to agree on what one means — tables, autolinks, escapes, character
 * references, setext headings, loose lists, the lot — and that agreement is a
 * parser's job rather than a regular expression's.
 *
 * markdown-it is bundled into `app.js` like Vue, not fetched: the built file
 * is committed, so a dependency here is a permanent block in every diff of it,
 * and that is the price this pays for the dialect. It is configured for two
 * things this board needs that the defaults do not give:
 *
 * - `html: false`. Raw HTML in a body is text. GFM passes a limited set of
 *   tags through; a task file is written by agents and by merges, and the
 *   board puts this string into the page, so a `<script>` in a body is text
 *   and always was. Nothing downstream sanitises, because nothing has to.
 * - `breaks: true`. A soft line break is a `<br>`, where GFM folds it into a
 *   space. Task bodies are hand-wrapped findings as often as they are prose,
 *   and joining those into one line loses the shape they were written in.
 *
 * What is extended here, rather than configured, is below: the scheme
 * allow-list a link has to pass, the checkbox a task list item draws, the box
 * a table scrolls inside, and the two attributes every link and image carries.
 *
 * No Vue here, and no DOM: it takes a string and returns a string, so its
 * cases are `bun test` cases (see markdown.test.ts).
 */

import MarkdownIt from "markdown-it";
import type { StateCore } from "markdown-it";

/** The schemes a link may carry. Anything else — `javascript:`, `data:` — is
 *  left as the text it was written as. */
const SAFE_SCHEME = /^(https?:|mailto:)/i;
/** A bare scheme at the start of a href: what has to be recognised before the
 *  allow-list above can mean anything. Anything with no scheme at all is a
 *  relative link and is allowed. */
const HAS_SCHEME = /^[a-z][a-z0-9+.-]*:/i;
/** Dropped from a link before its scheme is read, in both spellings it can
 *  arrive in: a browser drops these first, so `java&#10;script:` would
 *  otherwise pass the check above, and markdown-it has already turned that one
 *  into `java%0ascript:` by the time the link is validated. */
const INVISIBLE = /[\u0000-\u001f\u007f]|%(?:[01][0-9a-f]|7f)/gi;
/** `[ ] ` and `[x] ` opening a list item, which is what makes it a task. */
const CHECKBOX = /^\[([ xX])\]\s+/;

const md = MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
  typographer: false,
});

md.validateLink = safeLink;
// A bare `www.` host is a link in GFM, and markdown-it leaves that one off by
// default. Bare addresses are on already, and a bare IP is off in both.
md.linkify.set({ fuzzyLink: true });
md.core.ruler.after("inline", "task_lists", taskLists);

/** The box a task list item draws. It is a token of our own rather than
 *  injected HTML, so the one thing that emits markup from a document is still
 *  the renderer's own rules. */
md.renderer.rules.task_checkbox = (tokens, at) =>
  `<input type="checkbox" disabled${tokens[at].meta?.checked ? " checked" : ""}> `;

/** A table is the one block a body can make wider than the sheet, so it is
 *  wrapped in something that can scroll without widening the dialog. */
md.renderer.rules.table_open = () => '<div class="table-scroll"><table>';
md.renderer.rules.table_close = () => "</table></div>\n";

/** Every link leaves the board, so every link leaves this tab. */
md.renderer.rules.link_open = (tokens, at, options, _env, self) => {
  tokens[at].attrSet("target", "_blank");
  tokens[at].attrSet("rel", "noreferrer noopener");
  return self.renderToken(tokens, at, options);
};

const drawImage = md.renderer.rules.image;
md.renderer.rules.image = (tokens, at, options, env, self) => {
  tokens[at].attrSet("loading", "lazy");
  return drawImage
    ? drawImage(tokens, at, options, env, self)
    : self.renderToken(tokens, at, options);
};

export function renderMarkdown(source: string): string {
  return md.render(source);
}

/**
 * Whether a link may be written at all.
 *
 * This replaces markdown-it's own check rather than adding to it: the default
 * refuses `javascript:`, `vbscript:`, `file:` and most of `data:`, which is a
 * deny-list, and a deny-list is the wrong shape for a document an agent wrote.
 * A scheme is either on the list or the link stays the text it was written as.
 *
 * The URL arrives decoded and normalised, so a scheme spelled as a character
 * reference reaches this as the scheme it is.
 */
function safeLink(url: string): boolean {
  const cleaned = url.replace(INVISIBLE, "");
  return !HAS_SCHEME.test(cleaned) || SAFE_SCHEME.test(cleaned);
}

/**
 * Task list items, which are GFM's and are not markdown-it's.
 *
 * A checkbox is the first thing in the first paragraph of a list item, and
 * nowhere else: that is the whole rule, and it is why this looks at the two
 * tokens in front of the text rather than at the text alone. The item carries
 * the class the stylesheet takes its bullet off through, and the box itself is
 * disabled — the file is the source of truth, and a click here would write
 * nothing to it.
 */
function taskLists(state: StateCore): void {
  const tokens = state.tokens;
  for (let at = 2; at < tokens.length; at++) {
    if (tokens[at].type !== "inline") continue;
    if (tokens[at - 1].type !== "paragraph_open") continue;
    if (tokens[at - 2].type !== "list_item_open") continue;

    const box = CHECKBOX.exec(tokens[at].content);
    if (!box) continue;
    const children = tokens[at].children;
    if (!children || children.length === 0 || children[0].type !== "text") continue;

    tokens[at - 2].attrJoin("class", "task-item");
    tokens[at].content = tokens[at].content.slice(box[0].length);
    children[0].content = children[0].content.slice(box[0].length);

    const checkbox = new state.Token("task_checkbox", "", 0);
    checkbox.meta = { checked: box[1] !== " " };
    children.unshift(checkbox);
  }
}

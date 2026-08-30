/**
 * The Markdown a task body is rendered with, in the dialog.
 *
 * Hand-written rather than a dependency, for the reason that keeps the rest of
 * the stack thin: the built `app.js` is committed, so a renderer is a
 * permanent block in every diff of it, and the two things a library would buy
 * here — the long tail of CommonMark, and sanitising — are either unused by
 * task bodies or better answered directly. This escapes first and emits its
 * own tags, so there is no HTML from the document left to sanitise afterwards:
 * a `<script>` in a body is text, and always was.
 *
 * What it covers is what `tq` bodies are written in: ATX headings, paragraphs,
 * bullet and numbered lists (nested, with task-list checkboxes), fenced code,
 * blockquotes, thematic breaks, and the inline set — code spans, links,
 * images, bold, italic, strikethrough. Anything else degrades to the text it
 * was written as, which is the right failure for a renderer sitting over an
 * editor: what it cannot draw, a click still opens.
 *
 * No Vue here, and no DOM: it takes a string and returns a string, so its
 * cases are `bun test` cases (see markdown.test.ts).
 */

/** The schemes a link may carry. Anything else — `javascript:`, `data:` — is
 *  left as the text it was written as. */
const SAFE_SCHEME = /^(https?:|mailto:)/i;
/** A bare scheme at the start of a href: what has to be recognised before the
 *  allow-list above can mean anything. Anything with no scheme at all is a
 *  relative link and is allowed. */
const HAS_SCHEME = /^[a-z][a-z0-9+.-]*:/i;
/** Stripped from a href before its scheme is read: a browser drops these
 *  first, so `java&#10;script:` would otherwise pass the check above. */
const INVISIBLE = /[\u0000-\u001f\u007f]/g;

const FENCE = /^(`{3,}|~{3,})\s*(\S*)/;
const HEADING = /^(#{1,6})\s+(.*)$/;
const RULE = /^(-{3,}|\*{3,}|_{3,})$/;
const QUOTE = /^>\s?(.*)$/;
const BULLET = /^([-*+])\s+(.*)$/;
const NUMBER = /^(\d{1,9})[.)]\s+(.*)$/;
/** `- [ ] ` and `- [x] `, once the marker has been taken off. */
const CHECKBOX = /^\[([ xX])\]\s+(.*)$/;
const INDENT = /^[ \t]*/;

/** A tab is worth this many columns when a list's nesting is measured. */
const TAB_WIDTH = 4;

export function renderMarkdown(source: string): string {
  return blocks(source.replace(/\r\n?/g, "\n").split("\n"));
}

/** How far into the line the text starts, in columns. */
function indentOf(line: string): number {
  let width = 0;
  for (const character of INDENT.exec(line)?.[0] ?? "") {
    width += character === "\t" ? TAB_WIDTH - (width % TAB_WIDTH) : 1;
  }
  return width;
}

/** How many characters of `line` make up `column` columns of indentation. */
function prefixLength(line: string, column: number): number {
  let width = 0;
  let at = 0;
  while (at < line.length && width < column) {
    const character = line[at];
    if (character !== " " && character !== "\t") break;
    width += character === "\t" ? TAB_WIDTH - (width % TAB_WIDTH) : 1;
    at++;
  }
  return at;
}

/**
 * Reads a run of lines as blocks. Every caller hands in lines that already
 * start at column zero — a list item re-indents its own body before recursing
 * — so nesting is a property of what is passed in rather than a parameter
 * every block type would have to carry.
 */
function blocks(lines: string[]): string {
  const out: string[] = [];
  let at = 0;

  while (at < lines.length) {
    const trimmed = lines[at].trim();
    if (trimmed === "") {
      at++;
      continue;
    }

    const fence = FENCE.exec(trimmed);
    if (fence) {
      const closed = fenceEnd(lines, at + 1, fence[1][0]);
      out.push(codeBlock(lines.slice(at + 1, closed), fence[2]));
      // Past the closing fence, or to the end when there is none: an unclosed
      // fence takes the rest of the document, the way every renderer reads it.
      at = closed < lines.length ? closed + 1 : closed;
      continue;
    }

    const heading = HEADING.exec(trimmed);
    if (heading) {
      const level = heading[1].length;
      out.push(`<h${level}>${inline(heading[2].trim())}</h${level}>`);
      at++;
      continue;
    }

    if (RULE.test(trimmed)) {
      out.push("<hr>");
      at++;
      continue;
    }

    if (QUOTE.test(trimmed)) {
      const end = runOf(lines, at, (line) => QUOTE.test(line.trim()));
      const inner = lines.slice(at, end).map((line) => QUOTE.exec(line.trim())?.[1] ?? "");
      out.push(`<blockquote>${blocks(inner)}</blockquote>`);
      at = end;
      continue;
    }

    if (BULLET.test(trimmed) || NUMBER.test(trimmed)) {
      const list = listAt(lines, at, indentOf(lines[at]));
      out.push(list.html);
      at = list.next;
      continue;
    }

    const end = runOf(lines, at, (line) => line.trim() !== "" && !opensABlock(line.trim()));
    const text = lines
      .slice(at, end)
      .map((line) => line.trim())
      .join("\n");
    out.push(`<p>${inline(text)}</p>`);
    at = end;
  }

  return out.join("");
}

/** Whether a line would start something other than the paragraph above it. */
function opensABlock(trimmed: string): boolean {
  return (
    FENCE.test(trimmed) ||
    HEADING.test(trimmed) ||
    RULE.test(trimmed) ||
    QUOTE.test(trimmed) ||
    BULLET.test(trimmed) ||
    NUMBER.test(trimmed)
  );
}

/** The end of the run of lines from `at` that all answer `holds`. */
function runOf(lines: string[], at: number, holds: (line: string) => boolean): number {
  let end = at + 1;
  while (end < lines.length && holds(lines[end])) end++;
  return end;
}

/** Where the closing fence is, or `lines.length` when it is never closed. */
function fenceEnd(lines: string[], from: number, marker: string): number {
  for (let at = from; at < lines.length; at++) {
    const found = FENCE.exec(lines[at].trim());
    if (found && found[1][0] === marker) return at;
  }
  return lines.length;
}

function codeBlock(lines: string[], language: string): string {
  // The language is the author's word, so it goes in a class rather than
  // anywhere it could be read as markup; escaped like everything else.
  const marked = language === "" ? "" : ` class="language-${escapeHTML(language)}"`;
  return `<pre><code${marked}>${escapeHTML(lines.join("\n"))}</code></pre>`;
}

/** A list and where it ends, so the caller can carry on after it. */
interface List {
  html: string;
  /** The first line the list does not own. */
  next: number;
}

/**
 * One list, and every item and nested list under it.
 *
 * Items are separated by their marker at this level of indentation; anything
 * indented further belongs to the item above and is read as blocks of its own,
 * which is what makes a nested list, a fenced block or a second paragraph
 * inside an item work without a case each.
 */
function listAt(lines: string[], at: number, column: number): List {
  const first = lines[at].trim();
  const ordered = !BULLET.test(first) && NUMBER.test(first);
  const start = ordered ? Number(NUMBER.exec(first)?.[1] ?? "1") : 1;
  const items: string[] = [];
  let position = at;

  while (position < lines.length) {
    if (lines[position].trim() === "") {
      // A blank line ends the list only when what follows is not part of it.
      const next = runOf(lines, position, (line) => line.trim() === "");
      if (next >= lines.length || indentOf(lines[next]) < column) break;
      position = next;
      continue;
    }
    if (indentOf(lines[position]) !== column) break;

    const marker = (ordered ? NUMBER : BULLET).exec(lines[position].trim());
    if (!marker) break;

    // Everything up to the next line at this column or further out belongs to
    // this item, blank lines included.
    let end = position + 1;
    while (end < lines.length) {
      if (lines[end].trim() !== "" && indentOf(lines[end]) <= column) break;
      end++;
    }
    // Trailing blank lines belong to the list, not to the item.
    while (end > position + 1 && lines[end - 1].trim() === "") end--;

    const text = column + marker[0].length - marker[2].length;
    items.push(item([marker[2], ...lines.slice(position + 1, end)], text));
    position = end;
  }

  const tag = ordered ? "ol" : "ul";
  const from = ordered && start !== 1 ? ` start="${start}"` : "";
  return { html: `<${tag}${from}>${items.join("")}</${tag}>`, next: position };
}

/**
 * One list item. Its first line has had its marker taken off already, so the
 * lines under it are re-indented to line up with that text before being read
 * as blocks — which is what lets a nested list start at the item's own column.
 */
function item(body: string[], column: number): string {
  const [head = "", ...rest] = body;
  const box = CHECKBOX.exec(head);
  const checked = box ? ` <input type="checkbox" disabled${box[1] === " " ? "" : " checked"}>` : "";

  const inner = blocks([
    box ? box[2] : head,
    ...rest.map((line) => line.slice(prefixLength(line, column))),
  ]);
  // An item whose prose is one paragraph is unwrapped, which is what keeps a
  // plain list — and a list whose items carry a nested one — from being a
  // column of paragraphs with a paragraph's margins. An item that really does
  // hold two paragraphs keeps both, because then the spacing is the point.
  const single = (inner.match(/<p>/g) ?? []).length === 1;
  const only = single ? inner.replace(/^<p>([\s\S]*?)<\/p>/, "$1") : inner;
  return `<li${box ? ' class="task-item"' : ""}>${checked}${only}</li>`;
}

// ── Inline ──────────────────────────────────────────────────────

export function escapeHTML(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** What a code span is held under while the rest of the inline rules run. It
 *  cannot collide with the document: `inline` drops these before it starts. */
const HELD = "\u0000";

/**
 * The inline pass, and the order is the whole of it.
 *
 * Code spans come out first and go back last, so nothing inside a backtick is
 * read as markup and nothing the other rules emit can be swallowed by one.
 * Everything else runs over escaped text, which is why a pattern can be
 * written without minding what the document held: by the time it runs, the
 * only `<` in the string is one this file wrote.
 */
export function inline(text: string): string {
  const code: string[] = [];
  const held = escapeHTML(text.replace(new RegExp(HELD, "g"), "")).replace(
    /(`+)([\s\S]*?)\1/g,
    (_whole, _ticks: string, inner: string) => {
      code.push(`<code>${trimCodeSpan(inner)}</code>`);
      return `${HELD}${code.length - 1}${HELD}`;
    },
  );

  const marked = held
    .replace(/!\[([^\]]*)\]\(([^)\s]+)\)/g, (whole, alt: string, href: string) => {
      const url = safeHref(href);
      return url === null ? whole : `<img src="${url}" alt="${alt}" loading="lazy">`;
    })
    .replace(/\[([^\]]+)\]\(([^)\s]+)\)/g, (whole, label: string, href: string) => {
      const url = safeHref(href);
      return url === null ? whole : `${anchor(url)}${label}</a>`;
    })
    // A bare URL. Anchored to a line start, whitespace or an opening bracket,
    // which is also what keeps it off the href= the rule above just wrote.
    .replace(
      /(^|[\s(])(https?:\/\/[^\s<)]+)/g,
      (_whole, before: string, url: string) => `${before}${anchor(url)}${url}</a>`,
    )
    .replace(/(\*\*|__)(?=\S)([\s\S]*?\S)\1/g, "<strong>$2</strong>")
    .replace(/(?<![*\w])\*(?=\S)([^*]*?\S)\*(?![*\w])/g, "<em>$1</em>")
    .replace(/(?<!\w)_(?=\S)([^_]*?\S)_(?!\w)/g, "<em>$1</em>")
    .replace(/~~(?=\S)([\s\S]*?\S)~~/g, "<del>$1</del>")
    // A soft line break inside a paragraph, rendered rather than collapsed to a
    // space: task bodies are hand-wrapped findings as often as they are prose,
    // and joining those into one line loses the shape they were written in.
    .replace(/\n/g, "<br>");

  return marked.replace(
    new RegExp(`${HELD}(\\d+)${HELD}`, "g"),
    (_whole, at: string) => code[Number(at)],
  );
}

/** A code span drops one space at each end, so `` ` `x` ` `` holds a backtick. */
function trimCodeSpan(text: string): string {
  return /^ [\s\S]*[^ ] $/.test(text) ? text.slice(1, -1) : text;
}

/**
 * Whether a href may be emitted at all, and the escaped form if it may.
 *
 * The URL is the one place escaping is not the answer: `&quot;` keeps it
 * inside the attribute, but `javascript:alert(1)` needs no quote to run. So a
 * scheme is either on the list or the link stays the text it was written as.
 *
 * It is handed text `inline` has already escaped — every rule below the first
 * line of that function is — so escaping it again here would write `&amp;quot;`
 * into the href and break every URL carrying a query string.
 */
function safeHref(href: string): string | null {
  const cleaned = href.replace(INVISIBLE, "");
  return HAS_SCHEME.test(cleaned) && !SAFE_SCHEME.test(cleaned) ? null : cleaned;
}

/** Every link leaves the board, so every link leaves this tab. */
function anchor(url: string): string {
  return `<a href="${url}" target="_blank" rel="noreferrer noopener">`;
}

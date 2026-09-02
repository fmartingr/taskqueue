/**
 * The Markdown renderer the task dialog shows a body with.
 *
 * markdown-it is what reads the document, so these are not a parser's tests.
 * They are three things: that a task body renders the way the people and the
 * agents writing one expect — headings, lists, fenced code, links; that the
 * dialect really is GitHub Flavored Markdown, which is the reason for the
 * dependency, so a table, an autolink, an escape and a character reference
 * have a case each; and that nothing in a body can become markup, which is
 * ours rather than markdown-it's, because the dialog puts this string into the
 * page. The four extensions in markdown.ts — the scheme allow-list, the task
 * list checkbox, the scrolling box a table sits in and the attributes on a
 * link — carry the cases they have to.
 */

import { describe, expect, test } from "bun:test";

import { renderMarkdown } from "./markdown";

/** The rendered body, without the newline every block ends with. */
const render = (source: string) => renderMarkdown(source).trimEnd();

/** What one paragraph renders as, without the paragraph around it. */
const inline = (source: string) => {
  const html = render(source);
  return html.startsWith("<p>") && html.endsWith("</p>") ? html.slice(3, -4) : html;
};

describe("blocks", () => {
  test("a paragraph keeps the line breaks it was written with", () => {
    expect(render("one line\nand another")).toBe("<p>one line<br>\nand another</p>");
  });

  test("a blank line separates paragraphs", () => {
    expect(render("first\n\nsecond")).toBe("<p>first</p>\n<p>second</p>");
  });

  test("headings become their own level", () => {
    expect(render("## Finding\n\ntext")).toBe("<h2>Finding</h2>\n<p>text</p>");
    expect(render("###### deep")).toBe("<h6>deep</h6>");
    // Seven hashes is not a heading, so it stays the text it was written as.
    expect(render("####### too deep")).toBe("<p>####### too deep</p>");
  });

  test("a heading drops the run of hashes that closes it", () => {
    expect(render("## Finding ##")).toBe("<h2>Finding</h2>");
    expect(render("## ##")).toBe("<h2></h2>");
    // No space in front of it, so it is part of the text.
    expect(render("## C#")).toBe("<h2>C#</h2>");
  });

  test("an underlined line is a heading of its own", () => {
    expect(render("Title\n=====")).toBe("<h1>Title</h1>");
    expect(render("Title\n-")).toBe("<h2>Title</h2>");
    // The underline wins over the thematic break the same characters would be.
    expect(render("Title\n---")).toBe("<h2>Title</h2>");
  });

  test("a thematic break is a rule", () => {
    expect(render("a\n\n---\n\nb")).toBe("<p>a</p>\n<hr>\n<p>b</p>");
    expect(render("- - -")).toBe("<hr>");
  });

  test("four columns of indentation is a code block", () => {
    expect(render("    tq list --json\n    tq ready")).toBe(
      "<pre><code>tq list --json\ntq ready\n</code></pre>",
    );
    // Under a paragraph it is that paragraph carrying on, not a block.
    expect(render("text\n    still text")).toBe("<p>text<br>\nstill text</p>");
  });

  test("a bullet list is tight", () => {
    expect(render("- one\n- two")).toBe("<ul>\n<li>one</li>\n<li>two</li>\n</ul>");
  });

  test("a list with a blank line in it keeps every paragraph", () => {
    expect(render("- one\n\n- two")).toBe(
      "<ul>\n<li>\n<p>one</p>\n</li>\n<li>\n<p>two</p>\n</li>\n</ul>",
    );
  });

  test("a numbered list keeps the number it started at", () => {
    expect(render("3. three\n4. four")).toBe(
      '<ol start="3">\n<li>three</li>\n<li>four</li>\n</ol>',
    );
    expect(render("1. one")).toBe("<ol>\n<li>one</li>\n</ol>");
  });

  test("a nested list sits inside the item above it", () => {
    expect(render("- one\n  - inner\n- two")).toBe(
      "<ul>\n<li>one\n<ul>\n<li>inner</li>\n</ul>\n</li>\n<li>two</li>\n</ul>",
    );
  });

  test("a wrapped list item stays one item", () => {
    expect(render("- one that\n  wraps")).toBe("<ul>\n<li>one that<br>\nwraps</li>\n</ul>");
  });

  test("a fenced block keeps its text verbatim, and its language", () => {
    expect(render("```go\nif x < 1 {\n}\n```")).toBe(
      '<pre><code class="language-go">if x &lt; 1 {\n}\n</code></pre>',
    );
  });

  test("a longer fence holds the fences inside it", () => {
    expect(render("````\n```go\nx\n```\n````")).toBe("<pre><code>```go\nx\n```\n</code></pre>");
  });

  test("an unclosed fence takes the rest of the document", () => {
    expect(render("```\nnever closed")).toBe("<pre><code>never closed</code></pre>");
  });

  test("markup inside a fence is not markup", () => {
    expect(render("```\n- not a list\n## not a heading\n```")).toBe(
      "<pre><code>- not a list\n## not a heading\n</code></pre>",
    );
  });

  test("a blockquote holds blocks of its own", () => {
    expect(render("> quoted\n> - and a list")).toBe(
      "<blockquote>\n<p>quoted</p>\n<ul>\n<li>and a list</li>\n</ul>\n</blockquote>",
    );
  });

  test("a heading right after a paragraph ends it", () => {
    expect(render("text\n## Heading")).toBe("<p>text</p>\n<h2>Heading</h2>");
  });

  test("an escaped marker opens no block", () => {
    expect(render("\\## not a heading")).toBe("<p>## not a heading</p>");
    expect(render("\\- not a list")).toBe("<p>- not a list</p>");
  });

  test("an empty body renders nothing", () => {
    expect(render("")).toBe("");
    expect(render("\n\n  \n")).toBe("");
  });
});

// The checkbox is one of the four extensions in markdown.ts: GFM has task
// lists and markdown-it does not, so every part of that rule has a case.
describe("task lists", () => {
  test("a task list renders its boxes, and they cannot be clicked", () => {
    expect(render("- [ ] todo\n- [x] done")).toBe(
      '<ul>\n<li class="task-item"><input type="checkbox" disabled> todo</li>\n' +
        '<li class="task-item"><input type="checkbox" disabled checked> done</li>\n</ul>',
    );
  });

  test("an item keeps the markup after its box", () => {
    expect(render("- [x] **done** now")).toContain("checked> <strong>done</strong> now");
  });

  test("a box anywhere but at the start of an item is text", () => {
    expect(render("- text [ ] not a box")).toBe("<ul>\n<li>text [ ] not a box</li>\n</ul>");
    expect(render("[ ] not a box")).toBe("<p>[ ] not a box</p>");
  });
});

// Tables are the one block decided by its second line, so what is and is not
// one has as many cases as what a table renders as. The scrolling box around
// it is ours, and the dialog's width depends on it.
describe("tables", () => {
  test("a pipe table renders inside a box that can scroll", () => {
    expect(render("| a | b |\n| --- | --- |\n| 1 | 2 |")).toBe(
      '<div class="table-scroll"><table><thead>\n<tr>\n<th>a</th>\n<th>b</th>\n</tr>\n' +
        "</thead>\n<tbody>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>\n</tbody>\n</table></div>",
    );
  });

  test("the outer pipes are optional", () => {
    expect(render("a | b\n--- | ---\n1 | 2")).toContain("<th>a</th>\n<th>b</th>");
  });

  test("a colon in the delimiter row aligns its column", () => {
    const rendered = render("| a | b | c |\n| :-- | :-: | --: |\n| 1 | 2 | 3 |");
    expect(rendered).toContain('<th style="text-align:left">a</th>');
    expect(rendered).toContain('<th style="text-align:center">b</th>');
    expect(rendered).toContain('<th style="text-align:right">c</th>');
    expect(rendered).toContain('<td style="text-align:right">3</td>');
  });

  test("a row is drawn to the width of the header", () => {
    expect(render("| a | b |\n| --- | --- |\n| 1 |\n| 1 | 2 | 3 |")).toContain(
      "<tr>\n<td>1</td>\n<td></td>\n</tr>\n<tr>\n<td>1</td>\n<td>2</td>\n</tr>",
    );
  });

  test("a cell holds inline markup of its own", () => {
    expect(render("| a |\n| --- |\n| `code` and *em* |")).toContain(
      "<td><code>code</code> and <em>em</em></td>",
    );
  });

  test("an escaped pipe is a pipe in a cell", () => {
    expect(render("| a | b |\n| --- | --- |\n| x \\| y | 2 |")).toContain("<td>x | y</td>");
  });

  test("a table ends at a blank line and at the next block", () => {
    expect(render("| a |\n| --- |\n| 1 |\n\ntext")).toEndWith("</table></div>\n<p>text</p>");
    expect(render("| a |\n| --- |\n| 1 |\n## Heading")).toEndWith(
      "</table></div>\n<h2>Heading</h2>",
    );
  });

  test("a table takes the paragraph line above it as its header", () => {
    expect(render("text\na | b\n--- | ---\n1 | 2")).toStartWith("<p>text</p>\n<div");
  });

  test("a delimiter row that names no columns is not one", () => {
    // The counts have to match, and a one-column delimiter row needs its pipe,
    // which is what keeps it apart from the heading underline it looks like.
    expect(render("| a | b |\n| --- |")).toBe("<p>| a | b |<br>\n| --- |</p>");
    expect(render("a\n---")).toBe("<h2>a</h2>");
    expect(render("a | b\n--- | x")).toBe("<p>a | b<br>\n--- | x</p>");
  });
});

describe("inline", () => {
  test("bold, italic and strikethrough", () => {
    expect(inline("**bold** and *italic* and _also_ and ~~gone~~")).toBe(
      "<strong>bold</strong> and <em>italic</em> and <em>also</em> and <s>gone</s>",
    );
  });

  test("an underscore inside a word is not emphasis", () => {
    expect(inline("snake_case_name")).toBe("snake_case_name");
  });

  test("a code span is left alone by every other rule", () => {
    expect(inline("call `a**b**c` now")).toBe("call <code>a**b**c</code> now");
    expect(inline("`<script>`")).toBe("<code>&lt;script&gt;</code>");
  });

  test("a code span folds the line it wraps onto", () => {
    expect(inline("a `b\nc` d")).toBe("a <code>b c</code> d");
  });

  test("a backslash takes the markup out of what follows it", () => {
    expect(inline("\\*not italic\\*")).toBe("*not italic*");
    expect(inline("\\`not code\\`")).toBe("`not code`");
    // Only punctuation is escapable, so this is a backslash and a letter.
    expect(inline("\\q")).toBe("\\q");
  });

  test("a character reference becomes the character it names", () => {
    expect(inline("&copy; 2026 &mdash; &#65;&#x42;")).toBe("© 2026 — AB");
    // A reference for a character that means something in HTML comes back out
    // as the reference, which is the same text it arrived as.
    expect(inline("&lt;script&gt;")).toBe("&lt;script&gt;");
    expect(inline("AT&T")).toBe("AT&amp;T");
    expect(inline("&nosuchname;")).toBe("&amp;nosuchname;");
  });

  test("a link carries its label and leaves this tab", () => {
    expect(inline("[the docs](https://example.com/a)")).toBe(
      '<a href="https://example.com/a" target="_blank" rel="noreferrer noopener">the docs</a>',
    );
  });

  test("a relative link is a link", () => {
    expect(inline("[a file](./notes.md)")).toContain('href="./notes.md"');
  });

  test("a label holds its own markup, and never a second link", () => {
    expect(inline("[a *b* `c`](https://example.com)")).toContain("a <em>b</em> <code>c</code>");
    expect(inline("[see https://x.com](https://y.com)").match(/<a /g)).toHaveLength(1);
  });

  test("an image inside a link is the badge it was written as", () => {
    const rendered = inline("[![alt](i.png)](https://y.com)");
    expect(rendered).toContain('<img src="i.png" alt="alt" loading="lazy">');
    expect(rendered.match(/<a /g)).toHaveLength(1);
  });

  test("a bare URL becomes a link, and a link's own href does not become a second one", () => {
    expect(inline("see https://example.com/x now")).toBe(
      'see <a href="https://example.com/x" target="_blank" rel="noreferrer noopener">' +
        "https://example.com/x</a> now",
    );
    expect(inline("[x](https://example.com/x)").match(/<a /g)).toHaveLength(1);
  });

  test("a URL in angle brackets is a link", () => {
    expect(inline("<https://example.com>")).toBe(
      '<a href="https://example.com" target="_blank" rel="noreferrer noopener">' +
        "https://example.com</a>",
    );
    expect(inline("<me@example.com>")).toContain('href="mailto:me@example.com"');
  });

  test("a bare host and a bare address are links", () => {
    expect(inline("see www.example.com now")).toContain('href="http://www.example.com"');
    expect(inline("mail me@example.com now")).toContain('href="mailto:me@example.com"');
  });

  test("an autolink gives the sentence its punctuation back", () => {
    expect(inline("see https://example.com/x.")).toBe(
      'see <a href="https://example.com/x" target="_blank" rel="noreferrer noopener">' +
        "https://example.com/x</a>.",
    );
  });
});

// The renderer is the one place a task file's text reaches the page as markup,
// so every way HTML could arrive from it has a case rather than a comment. The
// document never carries a tag of its own — markdown-it is built with
// `html: false` — and a link is held to an allow-list of our own.
describe("nothing in a body becomes markup", () => {
  test("tags in the text are text", () => {
    expect(render("<script>alert(1)</script>")).toBe("<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>");
    expect(render("a <b>bold</b> tag")).toBe("<p>a &lt;b&gt;bold&lt;/b&gt; tag</p>");
  });

  test("an image's onerror cannot be written", () => {
    expect(render('<img src=x onerror="alert(1)">')).not.toContain("<img");
  });

  test("a javascript: link stays the text it was written as", () => {
    const rendered = inline("[click](javascript:alert(1))");
    expect(rendered).not.toContain("<a ");
    expect(rendered).toBe("[click](javascript:alert(1))");
  });

  test("a scheme cannot be smuggled past the check with a character reference", () => {
    // The link is decoded before it is validated, so this reaches the
    // allow-list as the scheme it is rather than as text that becomes one.
    expect(inline("[click](&#106;avascript:alert(1))")).not.toContain("<a ");
    expect(inline("<&#106;avascript:alert(1)>")).not.toContain("<a ");
  });

  test("a scheme cannot be smuggled past the check with a control character", () => {
    const url = `java${String.fromCharCode(1)}script:alert(1)`;
    expect(inline(`[click](${url})`)).not.toContain("<a ");
  });

  test("an angle autolink is held to the same allow-list", () => {
    expect(inline("<javascript:alert(1)>")).toBe("&lt;javascript:alert(1)&gt;");
  });

  test("a data: URL is refused too", () => {
    expect(inline("[click](data:text/html,<script>alert(1)</script>)")).not.toContain("<a ");
  });

  test("a quote in a URL cannot close the attribute", () => {
    const rendered = inline('[x](https://example.com/?a="onmouseover=alert(1))');
    expect(rendered).not.toContain('="onmouseover');
    expect(rendered).toContain("%22onmouseover");
  });

  test("a fence's language cannot close its own attribute", () => {
    expect(render('```a"onload="alert(1)\nx\n```')).toContain(
      'class="language-a&quot;onload=&quot;alert(1)"',
    );
  });

  test("a table cell cannot carry a tag out of the file", () => {
    expect(render("| a |\n| --- |\n| <img src=x onerror=alert(1)> |")).toContain(
      "<td>&lt;img src=x onerror=alert(1)&gt;</td>",
    );
  });

  test("a reference to a character no document may hold is replaced", () => {
    expect(inline("&#0;")).toBe(String.fromCharCode(0xfffd));
  });
});

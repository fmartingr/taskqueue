/**
 * The Markdown renderer the task dialog shows a body with.
 *
 * Two halves. The first is what task bodies are actually written in — headings,
 * lists, fenced code, links — because a renderer nobody can read a finding
 * through is not worth the click it replaced. The second is that nothing in a
 * body can become markup: the dialog puts this string into the page, so every
 * way HTML could arrive from the file has a case here rather than a comment.
 */

import { describe, expect, test } from "bun:test";

import { escapeHTML, inline, renderMarkdown } from "./markdown";

describe("blocks", () => {
  test("a paragraph keeps the line breaks it was written with", () => {
    expect(renderMarkdown("one line\nand another")).toBe("<p>one line<br>and another</p>");
  });

  test("a blank line separates paragraphs", () => {
    expect(renderMarkdown("first\n\nsecond")).toBe("<p>first</p><p>second</p>");
  });

  test("headings become their own level", () => {
    expect(renderMarkdown("## Finding\n\ntext")).toBe("<h2>Finding</h2><p>text</p>");
    expect(renderMarkdown("###### deep")).toBe("<h6>deep</h6>");
    // Seven hashes is not a heading, so it stays the text it was written as.
    expect(renderMarkdown("####### too deep")).toBe("<p>####### too deep</p>");
  });

  test("a thematic break is a rule", () => {
    expect(renderMarkdown("a\n\n---\n\nb")).toBe("<p>a</p><hr><p>b</p>");
  });

  test("a bullet list is tight", () => {
    expect(renderMarkdown("- one\n- two")).toBe("<ul><li>one</li><li>two</li></ul>");
  });

  test("a numbered list keeps the number it started at", () => {
    expect(renderMarkdown("3. three\n4. four")).toBe(
      '<ol start="3"><li>three</li><li>four</li></ol>',
    );
    expect(renderMarkdown("1. one")).toBe("<ol><li>one</li></ol>");
  });

  test("a nested list sits inside the item above it", () => {
    expect(renderMarkdown("- one\n  - inner\n- two")).toBe(
      "<ul><li>one<ul><li>inner</li></ul></li><li>two</li></ul>",
    );
  });

  test("a wrapped list item stays one item", () => {
    expect(renderMarkdown("- one that\n  wraps")).toBe("<ul><li>one that<br>wraps</li></ul>");
  });

  test("a task list renders its boxes, and they cannot be clicked", () => {
    expect(renderMarkdown("- [ ] todo\n- [x] done")).toBe(
      '<ul><li class="task-item"> <input type="checkbox" disabled>todo</li>' +
        '<li class="task-item"> <input type="checkbox" disabled checked>done</li></ul>',
    );
  });

  test("a fenced block keeps its text verbatim, and its language", () => {
    expect(renderMarkdown("```go\nif x < 1 {\n}\n```")).toBe(
      '<pre><code class="language-go">if x &lt; 1 {\n}</code></pre>',
    );
  });

  test("an unclosed fence takes the rest of the document", () => {
    expect(renderMarkdown("```\nnever closed")).toBe("<pre><code>never closed</code></pre>");
  });

  test("markup inside a fence is not markup", () => {
    expect(renderMarkdown("```\n- not a list\n## not a heading\n```")).toBe(
      "<pre><code>- not a list\n## not a heading</code></pre>",
    );
  });

  test("a blockquote holds blocks of its own", () => {
    expect(renderMarkdown("> quoted\n> - and a list")).toBe(
      "<blockquote><p>quoted</p><ul><li>and a list</li></ul></blockquote>",
    );
  });

  test("a heading right after a paragraph ends it", () => {
    expect(renderMarkdown("text\n## Heading")).toBe("<p>text</p><h2>Heading</h2>");
  });

  test("an empty body renders nothing", () => {
    expect(renderMarkdown("")).toBe("");
    expect(renderMarkdown("\n\n  \n")).toBe("");
  });
});

describe("inline", () => {
  test("bold, italic and strikethrough", () => {
    expect(inline("**bold** and *italic* and _also_ and ~~gone~~")).toBe(
      "<strong>bold</strong> and <em>italic</em> and <em>also</em> and <del>gone</del>",
    );
  });

  test("an underscore inside a word is not emphasis", () => {
    expect(inline("snake_case_name")).toBe("snake_case_name");
  });

  test("a code span is left alone by every other rule", () => {
    expect(inline("call `a**b**c` now")).toBe("call <code>a**b**c</code> now");
    expect(inline("`<script>`")).toBe("<code>&lt;script&gt;</code>");
  });

  test("a link carries its label and leaves this tab", () => {
    expect(inline("[the docs](https://example.com/a)")).toBe(
      '<a href="https://example.com/a" target="_blank" rel="noreferrer noopener">the docs</a>',
    );
  });

  test("a relative link is a link", () => {
    expect(inline("[a file](./notes.md)")).toContain('href="./notes.md"');
  });

  test("a bare URL becomes a link, and a link's own href does not become a second one", () => {
    expect(inline("see https://example.com/x now")).toBe(
      'see <a href="https://example.com/x" target="_blank" rel="noreferrer noopener">' +
        "https://example.com/x</a> now",
    );
    expect(inline("[x](https://example.com/x)").match(/<a /g)).toHaveLength(1);
  });
});

// The renderer is the one place a task file's text reaches the page as markup,
// so every way HTML could arrive from it has a case rather than a comment.
describe("nothing in a body becomes markup", () => {
  test("tags in the text are text", () => {
    expect(renderMarkdown("<script>alert(1)</script>")).toBe(
      "<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>",
    );
  });

  test("an image's onerror cannot be written", () => {
    expect(renderMarkdown('<img src=x onerror="alert(1)">')).not.toContain("<img");
  });

  test("a javascript: link stays the text it was written as", () => {
    const rendered = inline("[click](javascript:alert(1))");
    expect(rendered).not.toContain("<a ");
    expect(rendered).toBe("[click](javascript:alert(1))");
  });

  test("a scheme cannot be smuggled past the check with a control character", () => {
    expect(inline("[click](java\u0001script:alert(1))")).not.toContain("<a ");
  });

  test("a data: URL is refused too", () => {
    expect(inline("[click](data:text/html,<script>alert(1)</script>)")).not.toContain("<a ");
  });

  test("a quote in a URL cannot close the attribute", () => {
    const rendered = inline('[x](https://example.com/?a="onmouseover=alert(1))');
    // Already escaped by the time the href is read, which is why safeHref does
    // not escape it a second time.
    expect(rendered).toContain("&quot;onmouseover");
    expect(rendered).not.toContain('="onmouseover');
  });

  test("a fence's language cannot close its own attribute", () => {
    expect(renderMarkdown('```a"onload="alert(1)\nx\n```')).toContain(
      'class="language-a&quot;onload=&quot;alert(1)"',
    );
  });

  test("escapeHTML covers the four characters the rest of the file relies on", () => {
    expect(escapeHTML(`&<>"`)).toBe("&amp;&lt;&gt;&quot;");
  });
});

import { describe, expect, test } from "bun:test";
import { formatNote, joinBody, parseNotes, splitBody, type Note } from "./notes";

const STAMP = "2026-08-25T09:42:00+02:00";

describe("splitBody", () => {
  test("a body without notes is all content", () => {
    expect(splitBody("Description.\n\n## Acceptance criteria\n\n- something")).toEqual({
      content: "Description.\n\n## Acceptance criteria\n\n- something",
      notes: [],
    });
  });

  test("the canonical section is split off, rule included", () => {
    expect(splitBody(`Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`)).toEqual({
      content: "Description.",
      notes: [{ timestamp: STAMP, text: "A note." }],
    });
  });

  test("a legacy section without a rule is still notes", () => {
    expect(splitBody(`Description.\n\n## Notes\n\n- ${STAMP} — A note.`)).toEqual({
      content: "Description.",
      notes: [{ timestamp: STAMP, text: "A note." }],
    });
  });

  test("a Notes section followed by another section is content", () => {
    const body = "Description.\n\n## Notes\n\nProse about notes.\n\n## Acceptance criteria\n\n- something";
    expect(splitBody(body)).toEqual({ content: body, notes: [] });
  });

  test("a Notes section followed by a level-1 heading is content", () => {
    const body = "## Notes\n\n- a note\n\n# Appendix\n\ntext";
    expect(splitBody(body)).toEqual({ content: body, notes: [] });
  });

  test("a Notes section followed by a sub-heading is content", () => {
    const body = "## Notes\n\n- a note\n\n### Sub\n\ntext";
    expect(splitBody(body)).toEqual({ content: body, notes: [] });
  });

  test("a Notes heading inside a fenced block is content", () => {
    const body = "Description.\n\n```markdown\n## Notes\n\n- an example\n```";
    expect(splitBody(body)).toEqual({ content: body, notes: [] });
  });

  test("an unclosed fence does not hide the notes section", () => {
    const body = "Description.\n\n```go\nfunc x() {}\n\n---\n\n## Notes\n\n- " + STAMP + " — A note.";
    expect(splitBody(body)).toEqual({
      content: "Description.\n\n```go\nfunc x() {}",
      notes: [{ timestamp: STAMP, text: "A note." }],
    });
  });

  test("indentation at the start of the content is kept", () => {
    expect(splitBody(`    indented content\n\n## Notes\n\n- ${STAMP} — A note.`).content).toBe("    indented content");
  });

  test("the last Notes heading wins", () => {
    const body = `## Notes\n\ncontent\n\n## Other\n\nx\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`;
    expect(splitBody(body)).toEqual({
      content: "## Notes\n\ncontent\n\n## Other\n\nx",
      notes: [{ timestamp: STAMP, text: "A note." }],
    });
  });

  test("a setext heading underline is not the notes rule", () => {
    expect(splitBody(`Description\n---\n\n## Notes\n\n- ${STAMP} — A note.`).content).toBe("Description\n---");
  });

  // tq note indents the lines after the first under their bullet, so a note
  // may carry a heading of its own. That heading belongs to the note, and must
  // not end the section the way an unindented one does.
  test("an indented heading inside a note is part of the note", () => {
    expect(splitBody(`Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.\n\n  ## Details\n\n  text`)).toEqual({
      content: "Description.",
      notes: [{ timestamp: STAMP, text: "A note.\n\n## Details\n\ntext" }],
    });
  });

  test("an empty section yields no notes", () => {
    expect(splitBody("Description.\n\n---\n\n## Notes")).toEqual({ content: "Description.", notes: [] });
  });

  test("notes with no content keep an empty content", () => {
    expect(splitBody(`## Notes\n\n- ${STAMP} — A note.`)).toEqual({
      content: "",
      notes: [{ timestamp: STAMP, text: "A note." }],
    });
  });
});

describe("parseNotes", () => {
  test("continuation lines stay attached to their bullet", () => {
    const lines = [`- ${STAMP} — First line`, "  wrapped onto a second line", "- a hand-written bullet"];
    expect(parseNotes(lines)).toEqual([
      { timestamp: STAMP, text: "First line\nwrapped onto a second line" },
      { timestamp: "", text: "a hand-written bullet" },
    ]);
  });

  test("an indented bullet is a sub-bullet, not a note of its own", () => {
    expect(parseNotes([`- ${STAMP} — Findings:`, "  - one", "  - two"])).toEqual([
      { timestamp: STAMP, text: "Findings:\n- one\n- two" },
    ]);
  });

  test("a blank line inside a note is kept", () => {
    expect(parseNotes([`- ${STAMP} — See:`, "", "  ```go", "  func x() {}", "  ```"])).toEqual([
      { timestamp: STAMP, text: "See:\n\n```go\nfunc x() {}\n```" },
    ]);
  });

  test("blank lines between notes are dropped", () => {
    expect(parseNotes([`- ${STAMP} — One.`, "", `- ${STAMP} — Two.`, ""])).toEqual([
      { timestamp: STAMP, text: "One." },
      { timestamp: STAMP, text: "Two." },
    ]);
  });

  test("prose with no bullet at all is a single note", () => {
    expect(parseNotes(["Some prose", "wrapped over two lines"])).toEqual([
      { timestamp: "", text: "Some prose\nwrapped over two lines" },
    ]);
  });

  test("a bullet whose first word is not a timestamp keeps no timestamp", () => {
    expect(parseNotes(["- not a date — with an em dash"])).toEqual([
      { timestamp: "", text: "not a date — with an em dash" },
    ]);
  });

  test("blank lines do not create notes", () => {
    expect(parseNotes(["", `- ${STAMP} — A note.`, "", ""])).toEqual([{ timestamp: STAMP, text: "A note." }]);
  });
});

describe("joinBody", () => {
  test("a body without notes is the content alone", () => {
    expect(joinBody({ content: "Description.", notes: [] })).toBe("Description.");
  });

  test("notes are written last, under a rule", () => {
    expect(joinBody({ content: "Description.", notes: [{ timestamp: STAMP, text: "A note." }] })).toBe(
      `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`,
    );
  });

  test("notes without content need no rule", () => {
    expect(joinBody({ content: "", notes: [{ timestamp: STAMP, text: "A note." }] })).toBe(
      `## Notes\n\n- ${STAMP} — A note.`,
    );
  });

  test("empty notes are dropped rather than written as bare bullets", () => {
    expect(joinBody({ content: "Description.", notes: [{ timestamp: STAMP, text: "  " }] })).toBe("Description.");
  });
});

describe("formatNote", () => {
  test("a note without a timestamp is a plain bullet", () => {
    expect(formatNote({ timestamp: "", text: "Hand-written." })).toBe("- Hand-written.");
  });

  test("continuation lines are indented under their bullet", () => {
    expect(formatNote({ timestamp: STAMP, text: "First line\nsecond line" })).toBe(
      `- ${STAMP} — First line\n  second line`,
    );
  });

  test("nesting inside a note is preserved", () => {
    expect(formatNote({ timestamp: STAMP, text: "Findings:\n- one\n  - deeper" })).toBe(
      `- ${STAMP} — Findings:\n  - one\n    - deeper`,
    );
  });
});

describe("round trip", () => {
  const bodies: Record<string, string> = {
    "plain content": "Description.\n\n## Acceptance criteria\n\n- something",
    "content with its own Notes section":
      "Description.\n\n## Notes\n\nProse about notes.\n\n## Acceptance criteria\n\n- something",
    "content with a rule and its own Notes section":
      "Description.\n\n---\n\n## Notes\n\nProse.\n\n## Acceptance criteria\n\n- something",
    "content plus real notes": `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`,
    "a wrapped note": `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note\n  wrapped onto a second line.`,
    "a note with sub-bullets": `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — Findings:\n  - one\n  - two`,
    "a note with a fenced block": `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — See:\n\n  \`\`\`go\n  func x() {}\n  \`\`\``,
    "two notes": `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — One.\n- ${STAMP} — Two.`,
    "content that opens with an indented block": `    indented\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`,
    "notes only": `## Notes\n\n- ${STAMP} — A note.`,
    "a fenced example of the notes format": "Description.\n\n```markdown\n## Notes\n\n- an example\n```",
    "content whose Notes section is followed by a level-1 heading":
      `## Notes\n\n- old note\n\n# Appendix\n\ntext\n\n---\n\n## Notes\n\n- ${STAMP} — A real note.`,
  };

  for (const [name, body] of Object.entries(bodies)) {
    test(`${name} survives split and join unchanged`, () => {
      expect(joinBody(splitBody(body))).toBe(body);
    });
  }

  test("a legacy section without a rule is upgraded on the way out", () => {
    expect(joinBody(splitBody(`Description.\n\n## Notes\n\n- ${STAMP} — A note.`))).toBe(
      `Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`,
    );
  });

  test("editing the content leaves the notes alone", () => {
    const split = splitBody(`Description.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`);
    expect(joinBody({ ...split, content: "Rewritten." })).toBe(
      `Rewritten.\n\n---\n\n## Notes\n\n- ${STAMP} — A note.`,
    );
  });

  test("editing a note leaves the content alone", () => {
    const body = "Description.\n\n## Notes\n\nProse about notes.\n\n## Acceptance criteria\n\n- something";
    const split = splitBody(body);
    const notes: Note[] = [{ timestamp: STAMP, text: "Added from the board." }];
    expect(joinBody({ ...split, notes })).toBe(
      `${body}\n\n---\n\n## Notes\n\n- ${STAMP} — Added from the board.`,
    );
  });
});

/**
 * What one edit in the task dialog does about the file underneath it.
 *
 * The interesting cases are all the same shape — the file moved between the
 * editor opening and the editor closing — and the point of each is which half
 * of the body, or which field, that movement was in. A note appended under an
 * open textarea must not refuse the paragraph above it; a paragraph rewritten
 * on both sides must refuse, and must write nothing at all.
 */

import { describe, expect, test } from "bun:test";

import { commitContent, commitField, commitNote } from "./edit";
import { splitBody, type SplitBody } from "./notes";

const body = (text: string): SplitBody => splitBody(text);

const CONTENT = "## Finding\n\nAs filed.";
const NOTED = "## Finding\n\nAs filed.\n\n---\n\n## Notes\n\n- 2026-08-25T09:42:00+02:00 — a note";

describe("commitField", () => {
  test("a value the user did not change writes nothing", () => {
    expect(commitField("todo", "todo", "in-progress")).toBe("unchanged");
  });

  test("a value the file still holds is written", () => {
    expect(commitField("todo", "done", "todo")).toBe("write");
  });

  test("a value the file no longer holds is refused", () => {
    expect(commitField("todo", "done", "in-progress")).toBe("conflict");
  });

  test("a file that moved to the very value being written is still refused", () => {
    // Deliberate: the dialog cannot tell this apart from two people typing
    // different things, and the ticket's rule is that it does not try.
    expect(commitField("todo", "done", "done")).toBe("conflict");
  });
});

describe("commitContent", () => {
  test("an untouched textarea writes nothing", () => {
    expect(commitContent(CONTENT, "## Finding\n\nAs filed.", body(NOTED))).toEqual({
      outcome: "unchanged",
    });
  });

  test("a rewritten paragraph is written, and keeps the notes the file has", () => {
    const current = body(`${NOTED}\n- 2026-08-26T09:42:00+02:00 — appended since`);
    const commit = commitContent(CONTENT, "## Finding\n\nRewritten.", current);

    expect(commit.outcome).toBe("write");
    if (commit.outcome !== "write") return;
    expect(commit.body).toContain("Rewritten.");
    expect(commit.body).toContain("a note");
    // The note that arrived under the open textarea survives the write.
    expect(commit.body).toContain("appended since");
    expect(commit.body).not.toContain("As filed.");
  });

  test("a paragraph rewritten on both sides is refused", () => {
    const current = body("## Finding\n\nRevised by an agent.");
    expect(commitContent(CONTENT, "## Finding\n\nRewritten here.", current)).toEqual({
      outcome: "conflict",
    });
  });
});

describe("commitNote", () => {
  test("a note left alone writes nothing", () => {
    const opened = body(NOTED);
    expect(commitNote(opened.notes[0], "a note", opened)).toEqual({ outcome: "unchanged" });
  });

  test("a reworded note is written, and the content half is the file's", () => {
    const opened = body(NOTED);
    const current = body(NOTED.replace("As filed.", "Revised by an agent."));
    const commit = commitNote(opened.notes[0], "reworded", current);

    expect(commit.outcome).toBe("write");
    if (commit.outcome !== "write") return;
    expect(commit.body).toContain("- 2026-08-25T09:42:00+02:00 — reworded");
    // The paragraph is the file's: a note edit is not an edit of the content.
    expect(commit.body).toContain("Revised by an agent.");
  });

  test("a note is found by what it said, not by where it was", () => {
    const opened = body(NOTED);
    const current = body(
      "## Finding\n\nAs filed.\n\n---\n\n## Notes\n\n- 2026-08-24T09:00:00+02:00 — an earlier one\n" +
        "- 2026-08-25T09:42:00+02:00 — a note",
    );
    const commit = commitNote(opened.notes[0], "reworded", current);

    expect(commit.outcome).toBe("write");
    if (commit.outcome !== "write") return;
    expect(commit.body).toContain("an earlier one");
    expect(commit.body).toContain("— reworded");
  });

  test("a note that has left the file is refused rather than dropped", () => {
    const opened = body(NOTED);
    expect(commitNote(opened.notes[0], "reworded", body("## Finding\n\nAs filed."))).toEqual({
      outcome: "conflict",
    });
  });

  test("a note reworded on both sides is refused", () => {
    const opened = body(NOTED);
    const current = body(NOTED.replace("— a note", "— reworded by an agent"));
    expect(commitNote(opened.notes[0], "reworded here", current)).toEqual({ outcome: "conflict" });
  });
});

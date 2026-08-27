import { describe, expect, test } from "bun:test";
import { adoptBody, adoptField } from "./adopt";
import { joinBody, mergeBody, type Note, type SplitBody } from "./notes";

const note = (timestamp: string, text: string): Note => ({ timestamp, text });

const first = note("2026-08-25T09:00:00+02:00", "the note the dialog opened with");
const later = note("2026-08-25T11:00:00+02:00", "written by an agent");

const body = (content: string, ...notes: Note[]): SplitBody => ({ content, notes });

describe("adoptField", () => {
  test("a value the file has not moved is nothing to do", () => {
    expect(adoptField("todo", "todo", "todo", false)).toBe("unchanged");
    // Even under the caret, and even with an edit in the field: what the file
    // holds is still what this field was last given, so there is no news.
    expect(adoptField("todo", "in-progress", "todo", true)).toBe("unchanged");
  });

  test("a field nobody is in takes what the file holds", () => {
    expect(adoptField("todo", "todo", "in-progress", false)).toBe("take");
  });

  test("a field the user typed in is kept, and the change is reported", () => {
    expect(adoptField("As filed", "Retitled here", "Retitled by an agent", false)).toBe("keep");
  });

  test("a field the caret is in is deferred rather than replaced", () => {
    // Nothing has been typed yet, so this is not an edit — but a value that
    // changes under someone's caret is the same surprise, and the caller runs
    // the pass again when focus leaves.
    expect(adoptField("As filed", "As filed", "Retitled by an agent", true)).toBe("defer");
  });

  test("an edit outranks the caret: it is kept, not merely deferred", () => {
    expect(adoptField("As filed", "Retitled here", "Retitled by an agent", true)).toBe("keep");
  });
});

describe("adoptBody", () => {
  test("a note appended on disk reaches the panel and the baseline at once", () => {
    const opened = body("As filed.", first);
    const adoption = adoptBody(opened, opened, body("As filed.", first, later), false);

    expect(adoption.edited).toEqual(body("As filed.", first, later));
    expect(adoption.baseline).toEqual(body("As filed.", first, later));
    expect(adoption.content).toBe("taken");
  });

  test("content nobody is editing is taken, baseline and all", () => {
    const opened = body("As filed.", first);
    const adoption = adoptBody(opened, opened, body("Revised by an agent.", first), false);

    expect(adoption.edited.content).toBe("Revised by an agent.");
    expect(adoption.baseline.content).toBe("Revised by an agent.");
    expect(adoption.content).toBe("taken");
  });

  test("content the user rewrote is kept, and the baseline stays behind with it", () => {
    const opened = body("As filed.", first);
    const edited = body("Rewritten in the dialog.", first);
    const adoption = adoptBody(opened, edited, body("Revised by an agent.", first), false);

    expect(adoption.edited.content).toBe("Rewritten in the dialog.");
    // Left at the body the edit was made against, so the save can still see
    // that the file moved and refuse rather than pick a winner.
    expect(adoption.baseline.content).toBe("As filed.");
    expect(adoption.content).toBe("overridden");
  });

  test("content under the caret is held without being reported", () => {
    const opened = body("As filed.", first);
    const adoption = adoptBody(opened, opened, body("Revised by an agent.", first), true);

    expect(adoption.edited.content).toBe("As filed.");
    expect(adoption.baseline.content).toBe("As filed.");
    // Nothing was typed, so there is nothing to report: the next pass, once the
    // caret has gone, takes it.
    expect(adoption.content).toBe("held");
  });

  test("an edit of a note here survives a note arriving beside it", () => {
    const opened = body("As filed.", first);
    const reworded = note(first.timestamp, "reworded in the dialog");
    const edited = body("As filed.", reworded);
    const adoption = adoptBody(opened, edited, body("As filed.", first, later), false);

    expect(adoption.edited.notes).toEqual([reworded, later]);
    // The baseline is what the *file* holds, which is what makes the note above
    // recognisable as an edit of `first` rather than a note nobody wrote.
    expect(adoption.baseline.notes).toEqual([first, later]);
  });

  test("nothing handed in is written to", () => {
    // Load-bearing, not tidiness. buildPatch captures the dialog's snapshots by
    // reference before its round trip and merges from them after it, so an
    // adoption landing in between must leave those captures alone — merging in
    // place would move them out from under the merge and hand mergeNotes two
    // lists that no longer line up.
    const opened = body("As filed.", first);
    const edited = body("As filed.", first);
    const current = body("Revised by an agent.", first, later);

    const adoption = adoptBody(opened, edited, current, false);

    expect(opened).toEqual(body("As filed.", first));
    expect(edited).toEqual(body("As filed.", first));
    expect(current).toEqual(body("Revised by an agent.", first, later));
    expect(adoption.edited.notes).not.toBe(edited.notes);
    expect(adoption.baseline.notes).not.toBe(current.notes);
  });

  test("the two snapshots are independent: editing one cannot move the other", () => {
    // The panel edits its notes in place, so the baseline has to be a copy or
    // an edit would rewrite the very snapshot it is supposed to be measured
    // against — and the save would see no edit at all.
    const arrived = note(later.timestamp, later.text);
    const opened = body("As filed.", first);
    const adoption = adoptBody(opened, opened, body("As filed.", first, arrived), false);

    adoption.edited.notes[1].text = "reworded after adopting";
    expect(adoption.baseline.notes[1].text).toBe("written by an agent");
  });
});

// The point of the whole exercise: whatever the dialog adopts while it is open,
// the save-time merge has to come out of it with the same answer it would have
// given had nothing arrived at all. These drive adoptBody and mergeBody in
// sequence, which is the only place the two are proved to agree.
describe("adopting, and then saving", () => {
  test("an adopted note is not added a second time by the save", () => {
    const opened = body("As filed.", first);
    const current = body("As filed.", first, later);
    const adoption = adoptBody(opened, opened, current, false);

    // Nothing else changed, so the save has no body to write: the note is on
    // disk already and the dialog is holding the same one, not a copy of it.
    expect(mergeBody(adoption.baseline, adoption.edited, current)).toEqual({ outcome: "unchanged" });
  });

  test("a note adopted and then reworded is written once, reworded", () => {
    const opened = body("As filed.", first);
    const current = body("As filed.", first, later);
    const adoption = adoptBody(opened, opened, current, false);

    // The user edits the note that arrived while they were reading.
    const reworded = note(later.timestamp, "reworded after it arrived");
    const edited = body(adoption.edited.content, adoption.edited.notes[0], reworded);

    expect(mergeBody(adoption.baseline, edited, current)).toEqual({
      outcome: "write",
      body: joinBody(body("As filed.", first, reworded)),
    });
  });

  test("content adopted and then edited is written, not refused", () => {
    // The trap this exists to catch: a baseline left at the open-time body
    // would make the adoption itself look like a second edit of the content,
    // and every save from here on would refuse over a conflict nobody had.
    const opened = body("As filed.", first);
    const current = body("Revised by an agent.", first);
    const adoption = adoptBody(opened, opened, current, false);

    const edited = body("Revised by an agent, then by me.", first);
    expect(mergeBody(adoption.baseline, edited, current)).toEqual({
      outcome: "write",
      body: joinBody(edited),
    });
  });

  test("a content edit the adoption had to hold is still refused on save", () => {
    const opened = body("As filed.", first);
    const edited = body("Rewritten in the dialog.", first);
    const current = body("Revised by an agent.", first);
    const adoption = adoptBody(opened, edited, current, false);

    expect(mergeBody(adoption.baseline, adoption.edited, current)).toEqual({ outcome: "conflict" });
  });

  test("a note arriving under a rewritten body still does not block the save", () => {
    // Notes and content are separate halves, and adopting the notes must not
    // drag the content half into a refusal (TQ-0079).
    const opened = body("As filed.", first);
    const edited = body("Rewritten in the dialog.", first);
    const current = body("As filed.", first, later);
    const adoption = adoptBody(opened, edited, current, false);

    expect(mergeBody(adoption.baseline, adoption.edited, current)).toEqual({
      outcome: "write",
      body: joinBody(body("Rewritten in the dialog.", first, later)),
    });
  });

  test("two adoptions in a row leave the save with nothing to write", () => {
    const opened = body("As filed.", first);
    const once = adoptBody(opened, opened, body("Revised once.", first), false);
    const current = body("Revised twice.", first, later);
    const twice = adoptBody(once.baseline, once.edited, current, false);

    expect(twice.edited).toEqual(current);
    expect(mergeBody(twice.baseline, twice.edited, current)).toEqual({ outcome: "unchanged" });
  });
});

// A field the file is holding back is named in the dialog, so the dialog has to
// stop naming it once it can be adopted after all — which is what a user
// putting a field back the way they found it does.
describe("giving a held field back", () => {
  test("content reverted here is taken on the next pass", () => {
    const opened = body("As filed.", first);
    const current = body("Revised by an agent.", first);
    const held = adoptBody(opened, body("Rewritten in the dialog.", first), current, false);
    expect(held.content).toBe("overridden");

    // The user undoes their rewrite, back to what the dialog was holding.
    const back = adoptBody(held.baseline, body("As filed.", first), current, false);
    expect(back.content).toBe("taken");
    expect(back.edited.content).toBe("Revised by an agent.");
  });
});

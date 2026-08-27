/**
 * What an open task dialog does about its own task changing on disk.
 *
 * The board stays live while a dialog is open (TQ-0084), so the task under one
 * can move while it is being edited — an agent claiming it, appending a note,
 * rewriting the body. Two rules decide what happens, and both live here rather
 * than in the component so they can be reasoned about, and tested, without a
 * DOM:
 *
 * - a field the user is not in the middle of takes what the file holds;
 * - a field they are editing is never overwritten, and the dialog says so.
 *
 * The body is the third case and the delicate one. The dialog holds a baseline
 * — the body the save-time merge in notes.ts is defined against — so taking a
 * note into the panel or a paragraph into the textarea without moving that
 * baseline with it would make the next save read the dialog's own adoption as
 * somebody else's edit: a note it did not write, or a content half changed on
 * both sides. adoptBody moves the two together, which is the whole of keeping
 * the live adoption and mergeBody on one snapshot.
 */

import { mergeNotes, type Note, type SplitBody } from "./notes";

/** What a dialog field should do about a value that changed on disk. */
export type FieldAdoption =
  /** Nothing moved: the file still holds what this field was last given. */
  | "unchanged"
  /** Take the file's value. */
  | "take"
  /** The caret is in the control: leave it, and ask again once it has gone. */
  | "defer"
  /** The user has edited it: leave it, and say the file changed underneath. */
  | "keep";

/**
 * Decides what one field does with the value the file now holds.
 *
 * `taken` is what the field was last given from the file — the value it opened
 * with, or the last one it adopted — so `local` differing from it is exactly
 * "the user typed here", with no dirty flag to keep in step per input.
 *
 * "defer" is the refinement TQ-0084 flagged rather than smuggled in: a field
 * that is merely focused has not been edited, but replacing a value under
 * someone's caret is the same surprise as overwriting their text. It costs
 * nothing, because it is only a deferral — the caller runs the pass again when
 * focus leaves — so the value is adopted a moment later rather than dropped.
 */
export function adoptField(
  taken: string,
  local: string,
  incoming: string,
  focused: boolean,
): FieldAdoption {
  if (incoming === taken) return "unchanged";
  if (local !== taken) return "keep";
  return focused ? "defer" : "take";
}

/**
 * What became of the content half of a body being adopted.
 *
 * "held" and "overridden" both leave the user's text alone; they differ in
 * whether there is anything to tell them about. Nothing moved on disk under a
 * held one — it is simply not the file's turn — while an overridden one is the
 * dialog declining to write down what the file now says.
 */
export type ContentAdoption = "taken" | "held" | "overridden";

/** The dialog's two body snapshots after taking in a body from the file. */
export interface BodyAdoption {
  /** The body the user is now editing: the textarea and the notes panel. */
  edited: SplitBody;
  /** The snapshot the save-time merge is defined against. */
  baseline: SplitBody;
  content: ContentAdoption;
}

const copy = (notes: Note[]): Note[] => notes.map((note) => ({ ...note }));

/**
 * Takes a body the file has changed into a dialog that is open on it.
 *
 * The notes half always adopts, through the same mergeNotes the save uses: a
 * note appended by `tq note` appears in the panel, a note being reworded here
 * keeps its wording, and the file decides which notes exist. Going through that
 * one function is also what keeps the two lists lined up index for index, which
 * is what mergeNotes asks of the pair it is handed.
 *
 * The content half adopts only when the user is not in it — `focused` is the
 * caller's answer for the caret, and an edit is visible here as `edited`
 * differing from `baseline`. Whether it did is reported, so the dialog can say
 * the file changed underneath rather than quietly showing stale text — and can
 * stop saying it once a later pass has taken the file's content after all.
 *
 * The baseline moves with whatever was adopted and with nothing else. Leaving
 * it behind an adoption invents a conflict the next save would refuse over;
 * moving it past an edit the dialog is still holding would drop the real one.
 *
 * Nothing handed in is written to, and that is load-bearing rather than tidy:
 * buildPatch captures the dialog's snapshots by reference before its round trip
 * and merges from them after it, so an adoption that landed in between has to
 * leave those captures alone. Merging in place would move them out from under
 * the merge and hand mergeNotes two lists that no longer line up.
 */
export function adoptBody(
  baseline: SplitBody,
  edited: SplitBody,
  current: SplitBody,
  focused: boolean,
): BodyAdoption {
  const notes = mergeNotes(baseline.notes, edited.notes, current.notes);
  const typed = edited.content !== baseline.content;
  const keep = typed || focused;

  return {
    edited: { content: keep ? edited.content : current.content, notes },
    baseline: {
      content: keep ? baseline.content : current.content,
      notes: copy(current.notes),
    },
    content: !keep ? "taken" : typed && current.content !== baseline.content ? "overridden" : "held",
  };
}

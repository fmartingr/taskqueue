/**
 * What one edit in the task dialog does about the file it is about to write to.
 *
 * The dialog writes a field the moment the user finishes with it (TQ-0069),
 * against the file as it stands at that instant rather than as it stood when
 * the dialog opened — so every commit here is defined by three values: what the
 * editor was opened with, what the user made of it, and what the file holds
 * now. There are only ever three answers, and the third is the point of the
 * ticket:
 *
 * - `unchanged` — nothing to write, so nothing is written.
 * - `write` — the file still holds what the editor started from, so the user's
 *   value goes in.
 * - `conflict` — it does not. The dialog says so and writes nothing at all.
 *   It does not merge, does not pick a winner and does not reload the field
 *   under the user's hands: their text stays where it is, and the change they
 *   collided with is theirs to look at in the VCS. That is a deliberate
 *   narrowing of what the board used to do, where a save merged the notes and
 *   refused only a body both sides had rewritten.
 *
 * The body is where "the same field" needs saying carefully, and it is why
 * `commitContent` and `commitNote` are separate from `commitField`. A body is
 * two things in one string — the content half and the notes under the rule —
 * and `tq note` writing a note is not a change to the paragraph someone is
 * rewriting. Each of the two commits below therefore compares only its own
 * half and rebuilds the body around whatever the file holds for the other.
 *
 * No Vue and no DOM: three functions over three values, so the cases are
 * `bun test` cases (see edit.test.ts).
 */

import { joinBody, type Note, type SplitBody } from "./notes";

/** What a commit decided, and the body to write when there is one. */
export type Commit =
  | { outcome: "unchanged" }
  | { outcome: "write"; body: string }
  | { outcome: "conflict" };

/** What a plain field's commit decided. A field is written whole, so there is
 *  no body to carry — the caller already has the value. */
export type FieldCommit = "unchanged" | "write" | "conflict";

/**
 * A field the dialog holds as one string: the title, the assignee, the status,
 * the priority, and the two lists once they are joined.
 *
 * `opened` is what the file held when this editor was opened — or, for a
 * control with no editor to open, what the dialog was showing when it was
 * used. A select acted on by someone reading a stale value is the same
 * collision as an input, and gets the same answer.
 */
export function commitField(opened: string, edited: string, current: string): FieldCommit {
  if (edited === opened) return "unchanged";
  return current === opened ? "write" : "conflict";
}

/**
 * The content half of the body — everything above the notes rule.
 *
 * The notes are taken from the file rather than from the dialog: they are not
 * what is being edited, and a note that landed while the textarea was open has
 * to survive the write that closes it. This is TQ-0010's property, kept, and
 * TQ-0079's refusal, made the ordinary case rather than the last resort.
 */
export function commitContent(opened: string, content: string, current: SplitBody): Commit {
  if (content === opened) return { outcome: "unchanged" };
  if (current.content !== opened) return { outcome: "conflict" };
  return { outcome: "write", body: joinBody({ content, notes: current.notes }) };
}

/**
 * One note's wording.
 *
 * The note is found in the file by the timestamp and the text the editor was
 * opened with, never by its position: the panel is live, so a position is a
 * name that can come to mean a different note between two keystrokes (TQ-0084).
 * A note that is no longer there at all is a conflict rather than a silent
 * no-op — somebody rewrote the body under the editor, and dropping the reword
 * without a word is exactly the "we won't intervene" this ticket refuses.
 */
export function commitNote(opened: Note, text: string, current: SplitBody): Commit {
  if (text === opened.text) return { outcome: "unchanged" };

  const at = current.notes.findIndex(
    (note) => note.timestamp === opened.timestamp && note.text === opened.text,
  );
  if (at === -1) return { outcome: "conflict" };

  const notes = current.notes.map((note, position) =>
    position === at ? { timestamp: note.timestamp, text } : note,
  );
  return { outcome: "write", body: joinBody({ content: current.content, notes }) };
}

/**
 * Notes inside the Markdown body.
 *
 * Notes are the *last* section of a task body and are introduced by a
 * horizontal rule, exactly as `tq note` writes them — the file stays the source
 * of truth:
 *
 *     Task content, which may itself contain a Notes section.
 *
 *     ---
 *
 *     ## Notes
 *
 *     - 2026-08-25T09:42:00+02:00 — the actual note
 *
 * The board splits that section out — the dialog shows the content half as
 * rendered Markdown and the notes as a list — and puts the two back together
 * whenever it writes either. A "## Notes" heading anywhere else — in the middle
 * of a document, or inside a fenced code block — is ordinary content and is
 * left alone. What a write does about the file having moved underneath it is
 * edit.ts's question, not this file's.
 *
 * This mirrors notesSection in task.go, which the CLI and the API write
 * through: the two have to agree or a board save would move an agent's notes.
 */

export const NOTES_HEADING = "## Notes";
export const NOTES_RULE = "---";

const FENCE_PATTERN = /^(```|~~~)/;
const HEADING_PATTERN = /^#{1,6}\s/;
const INDENT_PATTERN = /^[ \t]/;
/** Opens a Markdown list item, so indented lines below it are its content. */
const LIST_MARKER_PATTERN = /^([-*+]|\d{1,9}[.)])\s/;
const BULLET_PATTERN = /^[-*]\s+/;
/** The run of spaces and tabs a line opens with. */
const INDENT_RUN_PATTERN = /^[ \t]*/;
/** A bullet tq wrote: "<timestamp> — <text>", once the marker is stripped. */
const NOTE_PATTERN = /^(\S+)\s+—\s+([\s\S]*)$/;
/** The indentation a continuation line owes to the bullet it belongs to. */
const CONTINUATION_INDENT = "  ";

export interface Note {
  /** RFC 3339 timestamp, or "" for a bullet tq did not write. */
  timestamp: string;
  /** The note itself; continuation lines are kept, joined by newlines. */
  text: string;
}

export interface SplitBody {
  content: string;
  notes: Note[];
}

/** Trims blank lines only, the way the store trims a body it reads or writes. */
function trimBlankLines(text: string): string {
  return text.replace(/^\n+|\n+$/g, "");
}

export function splitBody(body: string): SplitBody {
  const lines = body.split("\n");
  const start = notesStart(lines);
  if (start === -1) {
    return { content: trimBlankLines(body), notes: [] };
  }

  let end = start;
  while (end > 0 && lines[end - 1].trim() === "") end--;
  // A "---" that does not follow a blank line underlines the text above it as a
  // setext heading, so it is part of the content and not the notes rule.
  if (end > 0 && lines[end - 1].trim() === NOTES_RULE && (end === 1 || lines[end - 2].trim() === "")) {
    end--;
  }

  return {
    content: trimBlankLines(lines.slice(0, end).join("\n")),
    notes: parseNotes(lines.slice(start + 1)),
  };
}

/**
 * Returns the index of the "## Notes" heading that opens the notes section, or
 * -1 when there is none. Only the heading of the body's last section qualifies;
 * any heading after it makes it content.
 */
function notesStart(lines: string[]): number {
  const [start, balanced] = scanNotesStart(lines, true);
  if (balanced) return start;
  // The body has an unclosed (or mismatched) fence, which would hide the notes
  // section behind it. Read the fences as ordinary lines instead.
  return scanNotesStart(lines, false)[0];
}

/** One pass of notesStart, also reporting whether the fences were balanced. */
function scanNotesStart(lines: string[], honourFences: boolean): [start: number, balanced: boolean] {
  let start = -1;
  let fenced = false;
  let inItem = false;

  for (let i = 0; i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (honourFences && FENCE_PATTERN.test(trimmed)) {
      fenced = !fenced;
      continue;
    }
    // An indented heading is ambiguous: a note's continuation lines are
    // indented under their bullet and may carry one, but CommonMark also
    // allows a real heading up to three spaces in. What separates them is the
    // list item — inside one the heading is the note's own text, and outside
    // one it is a heading like any other. A blank line does not end an item.
    const isIndented = INDENT_PATTERN.test(lines[i]);
    if (!isIndented && trimmed !== "") inItem = LIST_MARKER_PATTERN.test(trimmed);
    if (fenced || (isIndented && inItem) || !HEADING_PATTERN.test(trimmed)) continue;
    start = trimmed === NOTES_HEADING ? i : -1;
  }
  return [start, !fenced];
}

/**
 * Reads the lines under the heading as notes. Only an unindented bullet starts
 * a new one: wrapped lines, nested bullets and indented blocks stay attached to
 * the note above them, keeping their own indentation relative to it.
 */
export function parseNotes(lines: string[]): Note[] {
  const notes: Note[] = [];
  let blanks: string[] = [];

  for (const line of lines) {
    const text = line.replace(/\s+$/, "");
    if (text === "") {
      blanks.push("");
      continue;
    }

    const indented = /^\s/.test(text);
    if (notes.length === 0 || (!indented && BULLET_PATTERN.test(text))) {
      notes.push(parseNote(text.trim().replace(BULLET_PATTERN, "")));
      blanks = [];
      continue;
    }

    const last = notes[notes.length - 1];
    last.text = [last.text, ...blanks, text.replace(/^ {1,2}/, "")].join("\n");
    blanks = [];
  }
  return notes;
}

function parseNote(text: string): Note {
  const match = NOTE_PATTERN.exec(text);
  if (match && !Number.isNaN(new Date(match[1]).getTime())) {
    return { timestamp: match[1], text: match[2] };
  }
  // A hand-written bullet: keep it, just without a timestamp.
  return { timestamp: "", text };
}

/**
 * Puts a split body back together. An empty notes section is dropped rather
 * than written back as a heading with nothing under it — the panel shows it as
 * "no notes yet" either way.
 */
export function joinBody(body: SplitBody): string {
  const content = trimBlankLines(body.content);
  const notes = body.notes.filter((note) => note.text.trim() !== "");
  if (notes.length === 0) return content;

  const section = [NOTES_HEADING, "", ...notes.map(formatNote)].join("\n");
  // The blank line above the rule matters: content directly above "---" would
  // be a setext heading rather than a horizontal rule.
  return content === "" ? section : [content, "", NOTES_RULE, "", section].join("\n");
}

/**
 * Normalises a note's text into the lines of its bullet: line endings are
 * unified, trailing whitespace goes, runs of blank lines collapse to one, the
 * blank lines around the note are dropped, and the indent every line shares is
 * stripped — the bullet already owes each line two spaces, and a block pasted
 * with the margin it was copied with would read as an indented code block. What
 * is left is relative indentation, and it is kept. The first line is stripped
 * outright: it sits after the timestamp, where an indent cannot be read back.
 *
 * This mirrors noteLines in task.go, which `tq note` and POST
 * /api/tasks/{id}/notes write through, and the two have to agree. The board
 * re-renders every note when it saves a body, so a rule only one side applied
 * would make a task's canonical form depend on which surface touched it last,
 * and every save would land in a diff as whitespace churn (TQ-0054). The cases
 * both suites check are in internal/task/testdata/notes.json.
 */
export function noteLines(text: string): string[] {
  const lines: string[] = [];
  let blank = false;

  for (const raw of text.replace(/\r\n/g, "\n").split("\n")) {
    const line = raw.replace(/\r$/, "").replace(/[ \t]+$/, "");
    if (line === "") {
      // A blank line only counts once, and never before the first line.
      blank = lines.length > 0;
      continue;
    }
    if (blank) {
      lines.push("");
      blank = false;
    }
    lines.push(line);
  }
  if (lines.length === 0) return [];

  const indent = commonIndent(lines);
  const stripped = lines.map((line) => line.slice(indent.length));
  stripped[0] = stripped[0].replace(INDENT_RUN_PATTERN, "");
  return stripped;
}

/**
 * The run of spaces and tabs every non-blank line opens with. Blank lines are
 * skipped rather than counted as an empty prefix: a paragraph break carries no
 * indentation of its own, and letting one answer for the block around it would
 * flatten exactly the pasted text this is here to keep.
 */
function commonIndent(lines: string[]): string {
  let indent: string | null = null;

  for (const line of lines) {
    if (line === "") continue;
    const lead = INDENT_RUN_PATTERN.exec(line)?.[0] ?? "";
    if (indent === null) {
      indent = lead;
      continue;
    }
    let shared = 0;
    while (shared < indent.length && indent[shared] === lead[shared]) shared++;
    indent = indent.slice(0, shared);
    if (indent === "") break;
  }
  return indent ?? "";
}

export function formatNote(note: Note): string {
  const [first = "", ...rest] = noteLines(note.text);
  const head = note.timestamp === "" ? `- ${first}` : `- ${note.timestamp} — ${first}`;
  // Continuation lines are indented so they stay part of their bullet.
  return [head, ...rest.map((line) => (line === "" ? "" : CONTINUATION_INDENT + line))].join("\n");
}

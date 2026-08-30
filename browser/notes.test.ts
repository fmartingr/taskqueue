/**
 * The notes panel of the task dialog. `frontend/notes.test.ts` already covers
 * the split/join arithmetic and `frontend/edit.test.ts` what a write does about
 * the file; what only a browser can show is the editor TQ-0048 added — a note
 * turning into a textarea in place, and Enter, Shift+Enter, Escape and blur
 * each meaning something different in it.
 *
 * Since TQ-0069 an edit is a write: there is no Save left to carry it, so every
 * case below ends at the file rather than at the panel.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { cardIn, useBoard, type Board, type Task } from "./harness";

const openBoard = useBoard();

/** A task carrying the notes given, with its dialog already open. */
async function dialogWithNotes(...notes: string[]): Promise<Board & { id: string }> {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Task with notes");
    for (const note of notes) project.mustRun("note", id, "--", note);
  });

  await board.page.click(cardIn("todo", id));
  await board.page.waitForSelector("#task-dialog[open]");
  return { ...board, id };
}

/** The rendered text of every note in the panel. */
const noteTexts = (page: Page) =>
  page.$$eval("#task-notes .note .note-text", (items) => items.map((item) => item.textContent ?? ""));

/** The task as the server has it, which is what "it was written" means. */
async function saved(board: Board, id: string): Promise<Task | undefined> {
  return (await board.project.tasks(board.server)).find((candidate) => candidate.id === id);
}

test("the panel lists the notes the CLI appended, and the card counts them", async () => {
  const { page } = await dialogWithNotes("first note", "second note");

  expect(await noteTexts(page)).toEqual(["first note", "second note"]);
  // The timestamps are rendered as local times, not as the raw RFC 3339.
  expect(await page.textContent("#task-notes .note .note-time")).not.toContain("T");

  await page.click("#task-dialog [data-close='task-dialog'].close");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });
  expect(await page.textContent(".card .note-badge")).toContain("2");
});

test("a task with no notes says so", async () => {
  const { page } = await dialogWithNotes();
  expect(await page.textContent("#task-notes")).toContain("No notes yet");
});

test("editing a note in place writes it back to the file", async () => {
  const board = await dialogWithNotes("the original wording");
  const { page, id } = board;

  await page.click("#task-notes .note button.icon");
  await page.waitForSelector("#task-notes .note-editor");
  expect(await page.inputValue("#task-notes .note-editor")).toBe("the original wording");

  await page.fill("#task-notes .note-editor", "the edited wording");
  await page.press("#task-notes .note-editor", "Enter");

  // Enter ends the edit, and the edit is the write: the panel comes back from
  // the file rather than from what it was holding.
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["the edited wording"]);

  const task = await saved(board, id);
  expect(task?.body).toContain("the edited wording");
  expect(task?.body).not.toContain("the original wording");
  // The timestamp the CLI wrote survives the round trip.
  expect(task?.body).toMatch(/^- \d{4}-\d{2}-\d{2}T\S+ — the edited wording$/m);
});

test("Escape drops an edit and leaves the note as it was", async () => {
  const board = await dialogWithNotes("keep me");
  const { page, id } = board;

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "discard me");
  await page.press("#task-notes .note-editor", "Escape");

  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["keep me"]);
  // Escape inside an editor belongs to the editor: the dialog is still open.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();

  const task = await saved(board, id);
  expect(task?.body).toContain("keep me");
  expect(task?.body).not.toContain("discard me");
});

test("Shift+Enter writes a multi-line note, indented under its own bullet", async () => {
  const board = await dialogWithNotes("one line for now");
  const { page, id } = board;

  await page.click("#task-notes .note button.icon");
  const editor = "#task-notes .note-editor";
  await page.waitForSelector(editor);
  await page.fill(editor, "");
  await page.type(editor, "first line");
  await page.press(editor, "Shift+Enter");
  await page.type(editor, "second line");
  await page.press(editor, "Enter");

  await page.waitForSelector(editor, { state: "detached" });
  expect(await noteTexts(page)).toEqual(["first line\nsecond line"]);

  const task = await saved(board, id);
  expect(task?.body).toMatch(/- \S+ — first line\n {2}second line/);
});

test("clicking away from an editor keeps the edit, the way the composer does", async () => {
  const board = await dialogWithNotes("before the blur");
  const { page, id } = board;

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "after the blur");
  await page.click("#task-note"); // focus somewhere else in the dialog

  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["after the blur"]);
  expect((await saved(board, id))?.body).toContain("after the blur");
});

test("the composer appends a note, and clears itself once it has landed", async () => {
  const board = await dialogWithNotes();
  const { page, id } = board;

  await page.fill("#task-note", "appended from the panel");
  await page.click("#task-note-add");

  // The panel re-renders from the task the server returned.
  await page.waitForSelector("#task-notes .note .note-text");
  expect(await noteTexts(page)).toEqual(["appended from the panel"]);
  expect(await page.inputValue("#task-note")).toBe("");
  expect((await saved(board, id))?.body).toContain("appended from the panel");
});

// ── TQ-0054 ─────────────────────────────────────────────────────

test("a multi-line note can be written in the panel and survives the write", async () => {
  const board = await dialogWithNotes();
  const { page, id } = board;

  // A block pasted out of a terminal: every line carries the margin it was
  // copied with, and the margin is what must not reach the file — on top of
  // the two spaces the bullet already owes, it would read as a code block.
  await page.fill("#task-note", "    make test\n    make build");
  await page.click("#task-note-add");

  await page.waitForSelector("#task-notes .note .note-text");
  expect(await noteTexts(page)).toEqual(["make test\nmake build"]);
  expect(await page.inputValue("#task-note")).toBe("");
  expect((await saved(board, id))?.body).toContain(" — make test\n  make build");
});

test("Shift+Enter in the note field starts a second line instead of appending", async () => {
  const { page } = await dialogWithNotes();

  await page.fill("#task-note", "first line");
  await page.press("#task-note", "Shift+Enter");
  await page.press("#task-note", "s");

  expect(await page.inputValue("#task-note")).toBe("first line\ns");
  expect(await noteTexts(page)).toEqual([]);
  // TQ-0019's property, which outlived the form: the dialog is still open.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
});

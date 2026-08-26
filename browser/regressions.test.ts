/**
 * The three task-dialog bugs the Vue migration was asked to close.
 *
 * They are here rather than in notes.test.ts because none of them is about the
 * notes arithmetic: each is about what the page does with a real click, a real
 * Enter or a real file changing underneath it, which is what this layer sees
 * and no other layer does.
 *
 * Every one of these fails on the board as it was, and each fails for its own
 * reason — a framework that re-renders differently is not a fix, so none of
 * them is allowed to pass by accident.
 */

import { expect, test } from "bun:test";
import { cardIn, useBoard, type Board } from "./harness";

const openBoard = useBoard();

/** A task carrying the notes given, with its dialog already open. */
async function dialogWithNotes(...notes: string[]): Promise<Board & { id: string }> {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Task under edit");
    for (const note of notes) project.mustRun("note", id, "--", note);
  });

  await board.page.click(cardIn("todo", id));
  await board.page.waitForSelector("#task-dialog[open]");
  return { ...board, id };
}

/** The rendered text of every note in the panel. */
const noteTexts = (board: Board) =>
  board.page.$$eval("#task-notes .note .note-text", (items) => items.map((item) => item.textContent ?? ""));

// ── TQ-0010 ─────────────────────────────────────────────────────

test("saving the dialog keeps the notes written while it was open", async () => {
  const board = await dialogWithNotes("the note the dialog opened with");
  const { project, server, page, id } = board;

  // The poll stands down while the dialog is open, so the board cannot know
  // about these. Saving a body captured at open time is what erased them.
  project.mustRun("note", id, "--", "written by an agent");
  project.mustRun("note", id, "--", "and another one");

  await page.fill("#task-title", "Retitled while an agent worked");
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled while an agent worked");
  expect(task?.body).toContain("the note the dialog opened with");
  expect(task?.body).toContain("written by an agent");
  expect(task?.body).toContain("and another one");
});

test("an edit made in the dialog survives the merge with the file", async () => {
  const board = await dialogWithNotes("the original wording");
  const { project, server, page, id } = board;

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "the edited wording");
  await page.press("#task-notes .note-editor", "Enter");
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });

  // The file gains a note under the open dialog, and the dialog's own edit has
  // to survive being merged with it.
  project.mustRun("note", id, "--", "arrived while editing");

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("the edited wording");
  expect(task?.body).not.toContain("the original wording");
  expect(task?.body).toContain("arrived while editing");
});

// ── TQ-0019 ─────────────────────────────────────────────────────

test("Enter in the note field appends the note instead of saving the dialog", async () => {
  const board = await dialogWithNotes();
  const { project, server, page, id } = board;

  await page.fill("#task-note", "typed and entered");
  await page.press("#task-note", "Enter");

  // The note lands, the field clears, and — the actual bug — the dialog is
  // still open rather than having been submitted out from under the typing.
  await page.waitForSelector("#task-notes .note .note-text");
  expect(await noteTexts(board)).toEqual(["typed and entered"]);
  expect(await page.inputValue("#task-note")).toBe("");
  expect(await page.$("#task-dialog[open]")).not.toBeNull();

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("typed and entered");
});

// ── TQ-0027 ─────────────────────────────────────────────────────

test("clicking a second note's pencil while one is being edited opens it", async () => {
  const board = await dialogWithNotes("first note", "second note", "third note");
  const { page } = board;

  const pencil = (position: number) => `#task-notes .note:nth-of-type(${position}) button.icon`;
  const editor = (position: number) => `#task-notes .note:nth-of-type(${position}) .note-editor`;

  await page.click(pencil(1));
  await page.waitForSelector(editor(1));
  await page.fill(editor(1), "first note, reworded");

  // One click. The old panel rebuilt the list on blur, which detached this very
  // button before mouseup, so no click was ever dispatched and nothing happened.
  await page.click(pencil(2));

  await page.waitForSelector(editor(2));
  expect(await page.inputValue(editor(2))).toBe("second note");
  // The first note is out of edit, and kept what was typed in it.
  expect(await page.$(editor(1))).toBeNull();
  expect(await noteTexts(board)).toEqual(["first note, reworded", "third note"]);

  // And a third click keeps working, which is what "the click is swallowed"
  // would have broken from here on.
  await page.click(pencil(3));
  await page.waitForSelector(editor(3));
  expect(await page.inputValue(editor(3))).toBe("third note");
});

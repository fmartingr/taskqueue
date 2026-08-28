/**
 * The notes panel of the task dialog. `frontend/notes.test.ts` already covers
 * the split/join arithmetic; what only a browser can show is the editor TQ-0048
 * added — a note turning into a textarea in place, and Enter, Shift+Enter,
 * Escape and blur each meaning something different in it.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { cardIn, useBoard, type Board } from "./harness";

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

test("editing a note in place and saving writes it back to the file", async () => {
  const { project, server, page, id } = await dialogWithNotes("the original wording");

  await page.click("#task-notes .note button.icon");
  await page.waitForSelector("#task-notes .note-editor");
  expect(await page.inputValue("#task-notes .note-editor")).toBe("the original wording");

  await page.fill("#task-notes .note-editor", "the edited wording");
  await page.press("#task-notes .note-editor", "Enter");

  // Enter ends the edit and puts the note back as text, still unsaved.
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["the edited wording"]);

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("the edited wording");
  expect(task?.body).not.toContain("the original wording");
  // The timestamp the CLI wrote survives the round trip.
  expect(task?.body).toMatch(/^- \d{4}-\d{2}-\d{2}T\S+ — the edited wording$/m);
});

test("Escape drops an edit and leaves the note as it was", async () => {
  const { project, server, page, id } = await dialogWithNotes("keep me");

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "discard me");
  await page.press("#task-notes .note-editor", "Escape");

  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["keep me"]);

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("keep me");
  expect(task?.body).not.toContain("discard me");
});

test("Shift+Enter writes a multi-line note, indented under its own bullet", async () => {
  const { project, server, page, id } = await dialogWithNotes("one line for now");

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

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toMatch(/- \S+ — first line\n {2}second line/);
});

test("clicking away from an editor keeps the edit, the way the composer does", async () => {
  const { page } = await dialogWithNotes("before the blur");

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "after the blur");
  await page.click("#task-title"); // focus somewhere else in the dialog

  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  expect(await noteTexts(page)).toEqual(["after the blur"]);
});

test("adding a note through the panel saves pending edits with it", async () => {
  const { project, server, page, id } = await dialogWithNotes();

  await page.fill("#task-body", "Body typed but not saved yet.");
  await page.fill("#task-note", "appended from the panel");
  await page.click("#task-note-add");

  // The panel re-renders from the task the server returned.
  await page.waitForSelector("#task-notes .note .note-text");
  expect(await noteTexts(page)).toEqual(["appended from the panel"]);
  expect(await page.inputValue("#task-note")).toBe("");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Body typed but not saved yet.");
  expect(task?.body).toContain("appended from the panel");
});

// ── TQ-0054 ─────────────────────────────────────────────────────

test("a multi-line note can be written in the panel and survives the save", async () => {
  const { project, server, page, id } = await dialogWithNotes();

  // A block pasted out of a terminal: every line carries the margin it was
  // copied with, and the margin is what must not reach the file — on top of
  // the two spaces the bullet already owes, it would read as a code block.
  await page.fill("#task-note", "    make test\n    make build");
  await page.click("#task-note-add");

  await page.waitForSelector("#task-notes .note .note-text");
  expect(await noteTexts(page)).toEqual(["make test\nmake build"]);
  expect(await page.inputValue("#task-note")).toBe("");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain(" — make test\n  make build");
});

test("Shift+Enter in the note field starts a second line instead of appending", async () => {
  const { page } = await dialogWithNotes();

  await page.fill("#task-note", "first line");
  await page.press("#task-note", "Shift+Enter");
  await page.press("#task-note", "s");

  expect(await page.inputValue("#task-note")).toBe("first line\ns");
  expect(await noteTexts(page)).toEqual([]);
  // TQ-0019's property, which the textarea inherits: the dialog is still open.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
});

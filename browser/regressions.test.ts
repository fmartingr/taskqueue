/**
 * The task-dialog bugs about a file changing under an open dialog, and the
 * three the Vue migration was asked to close.
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

/** Opens a task's dialog on a board already showing its card. */
async function openDialog(board: Board, id: string): Promise<Board & { id: string }> {
  await board.page.click(cardIn("todo", id));
  await board.page.waitForSelector("#task-dialog[open]");
  return { ...board, id };
}

/** A task carrying the notes given, with its dialog already open. */
async function dialogWithNotes(...notes: string[]): Promise<Board & { id: string }> {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Task under edit");
    for (const note of notes) project.mustRun("note", id, "--", note);
  });

  return openDialog(board, id);
}

/** A task carrying the body given, with its dialog already open. */
async function dialogWithBody(body: string): Promise<Board & { id: string }> {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Task under edit", "--body", body);
  });

  return openDialog(board, id);
}

/**
 * The body changing outside the board, the way an agent revising a Finding
 * changes it. There is no `tq update --body`, so this goes through the API the
 * CLI shares — the same store, the same atomic write to the same file.
 */
async function reviseBody(board: Board, id: string, body: string): Promise<void> {
  const response = await fetch(`${board.server.url}/api/tasks/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ body }),
  });
  if (!response.ok) throw new Error(`PATCH /api/tasks/${id} = ${response.status}`);
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

// ── The same three bugs, one door further along ─────────────────
//
// Each of the tests above drives one path to its bug. Each of these drives the
// sibling path that the fix also has to cover, and that a plausible edit could
// take away without the tests above noticing.

// TQ-0010 again: "Add note" builds its patch the same way Save does, so it
// carries the same risk of writing back a body captured when the dialog opened.
test("appending a note keeps the notes written while the dialog was open", async () => {
  const board = await dialogWithNotes("the note the dialog opened with");
  const { project, server, page, id } = board;

  project.mustRun("note", id, "--", "written by an agent");

  await page.fill("#task-note", "appended from the panel");
  await page.click("#task-note-add");
  // Bounded on purpose: a note that never arrives is a failure, not a hang.
  await page.waitForSelector("#task-notes .note:nth-of-type(3)", { timeout: 10_000 });

  expect(await noteTexts(board)).toEqual([
    "the note the dialog opened with",
    "written by an agent",
    "appended from the panel",
  ]);

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("written by an agent");
});

// TQ-0027 again: the pencil answers to mousedown so the click that follows
// cannot be swallowed, but a focused button reached by keyboard fires no
// mousedown at all — so the click handler beside it is load-bearing, not
// redundant, and removing it would make the pencil mouse-only.
test("the pencil opens its editor from the keyboard as well as the mouse", async () => {
  const board = await dialogWithNotes("the note to edit");
  const { page } = board;

  await page.focus("#task-notes .note button.icon");
  await page.keyboard.press("Enter");

  await page.waitForSelector("#task-notes .note-editor", { timeout: 10_000 });
  expect(await page.inputValue("#task-notes .note-editor")).toBe("the note to edit");
});

// TQ-0019 in the other direction: the fix belongs to the note field alone. A
// form-level handler would pass the test above and quietly kill the implicit
// submit that Enter in any other field is supposed to do.
test("Enter in the title field still saves the dialog", async () => {
  const board = await dialogWithNotes("a note");
  const { project, server, page, id } = board;

  await page.fill("#task-title", "Retitled with Enter");
  await page.press("#task-title", "Enter");

  await page.waitForSelector("#task-dialog[open]", { state: "detached", timeout: 10_000 });
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled with Enter");
});

// ── TQ-0079 ─────────────────────────────────────────────────────
//
// TQ-0010 merged the notes half of the body and left the content half as the
// snapshot the dialog opened with, so a save that never touched the textarea
// still wrote that snapshot back over whatever had been written above the
// notes rule. The fix has two halves, and each has a test: an untouched
// textarea sends no body at all, and a textarea both sides edited is refused
// rather than merged.

test("a save that never touched the body leaves an edit made to it alone", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  // The poll stands down while the dialog is open, so the board cannot know.
  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");

  await page.selectOption("#task-priority", "urgent");
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.priority).toBe("urgent");
  expect(task?.body).toContain("Revised by an agent.");
  expect(task?.body).not.toContain("As filed.");
});

test("a body edited on both sides is refused, and the typing is kept", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  await page.fill("#task-body", "## Finding\n\nRewritten in the dialog.");
  await page.selectOption("#task-priority", "urgent");
  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");

  await page.click("#task-form button[type='submit']");

  const toast = await page.waitForSelector("#toasts .toast.error");
  const message = await toast.textContent();
  expect(message).toContain(id);
  expect(message).toContain("changed on disk");

  // The dialog is still open and still holds what the user typed: refusing is
  // only worth anything if the text it refused to write is still recoverable.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  expect(await page.inputValue("#task-body")).toBe("## Finding\n\nRewritten in the dialog.");

  // And nothing at all was written — not the body, and not the priority beside
  // it, which would have been a save that half happened.
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Revised by an agent.");
  expect(task?.body).not.toContain("Rewritten in the dialog.");
  expect(task?.priority).not.toBe("urgent");
});

// The refusal is about the content half alone. A merge that compared whole
// bodies would refuse this one too, and appending a note is the commonest
// thing to happen under an open dialog.
test("a note appended under the dialog does not block a body the user rewrote", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  await page.fill("#task-body", "## Finding\n\nRewritten in the dialog.");
  project.mustRun("note", id, "--", "arrived while editing");

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Rewritten in the dialog.");
  expect(task?.body).toContain("arrived while editing");
  expect(task?.body).not.toContain("As filed.");
});

// TQ-0010's property, on the path TQ-0079 changed: with no body to send at
// all, the notes appended under the dialog have to survive on the file's own
// terms rather than by being merged into one.
test("a note appended under the dialog survives a save that changed only the priority", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  project.mustRun("note", id, "--", "written by an agent");
  await page.selectOption("#task-priority", "urgent");

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.priority).toBe("urgent");
  expect(task?.body).toContain("As filed.");
  expect(task?.body).toContain("written by an agent");
});

// "Add note" is two writes, and the second can fail on its own. Until the
// dialog starts the body over from the first one, the write it just made
// itself reads back as somebody else's — and every later save refuses over it.
test("a note that fails to land does not strand the dialog on its own edit", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  const notesRoute = `**/api/tasks/${id}/notes`;
  await page.route(notesRoute, (route) =>
    route.fulfill({ status: 500, contentType: "application/json", body: `{"error":"nope"}` }),
  );

  await page.fill("#task-body", "## Finding\n\nRewritten in the dialog.");
  await page.fill("#task-note", "the note that never lands");
  await page.click("#task-note-add");

  const failed = await page.waitForSelector("#toasts .toast.error");
  expect(await failed.textContent()).toContain("Could not add a note");

  // The body write ahead of it did land, so saving now must not refuse it.
  await page.unroute(notesRoute);
  await page.fill("#task-title", "Saved after the note failed");
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Saved after the note failed");
  expect(task?.body).toContain("Rewritten in the dialog.");
});

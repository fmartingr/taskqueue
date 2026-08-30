/**
 * The task-dialog bugs, kept driving the dialog after TQ-0069 rebuilt it.
 *
 * Two of them — TQ-0010 and TQ-0079 — were about a save that wrote the whole
 * task back from a snapshot taken when the dialog opened. There is no such save
 * any more: a field is written when its editor closes, and the body is read off
 * the file first. That retires the mechanism, not the property, and the
 * property is what these check — a body written here must not take an agent's
 * notes with it, and must not put a paragraph back over one somebody else
 * rewrote.
 *
 * TQ-0019's own mechanism is gone too (there is no form and no submit button),
 * so what is left of it is the behaviour: Enter in the note box appends a note
 * and does not close anything.
 *
 * TQ-0027 is untouched by all of it and is checked exactly as it was.
 *
 * They are here rather than in notes.test.ts because none of them is about the
 * notes arithmetic: each is about what the page does with a real click, a real
 * Enter or a real file changing underneath it, which is what this layer sees
 * and no other layer does.
 */

import { expect, test } from "bun:test";
import { cardIn, choose, editBody, editField, openEditor, useBoard, type Board } from "./harness";

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

test("writing the description keeps the notes written while the dialog was open", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;
  project.mustRun("note", id, "--", "the note the dialog opened with");
  await page.waitForSelector("#task-notes .note .note-text");

  // The panel takes these in as they land (TQ-0084), but the write is what has
  // to keep them: a body captured at open time is what erased them, and
  // nothing about showing one makes it safe to write back.
  project.mustRun("note", id, "--", "written by an agent");
  project.mustRun("note", id, "--", "and another one");

  await editBody(page, "## Finding\n\nRewritten in the dialog.");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Rewritten in the dialog.");
  expect(task?.body).toContain("the note the dialog opened with");
  expect(task?.body).toContain("written by an agent");
  expect(task?.body).toContain("and another one");
});

test("a note edited here survives a note arriving beside it", async () => {
  const board = await dialogWithNotes("the original wording");
  const { project, server, page, id } = board;

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "the edited wording");
  // The file gains a note under the open editor, and the write that closes it
  // has to keep both — it is read off the file rather than replacing it.
  project.mustRun("note", id, "--", "arrived while editing");
  await page.waitForSelector("#task-notes .note:nth-of-type(2)");

  await page.press("#task-notes .note-editor", "Enter");
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("the edited wording");
  expect(task?.body).not.toContain("the original wording");
  expect(task?.body).toContain("arrived while editing");
});

// ── TQ-0019 ─────────────────────────────────────────────────────

test("Enter in the note field appends the note and closes nothing", async () => {
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

// TQ-0019 in the other direction: Enter in a single-line editor still ends the
// edit, which is what it is for. It just no longer submits anything.
test("Enter in the title editor writes the title and leaves the dialog open", async () => {
  const board = await dialogWithNotes("a note");
  const { project, server, page, id } = board;

  await editField(page, "task-title", "Retitled with Enter");

  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled with Enter");
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
  // The first note is out of edit, and its wording was written on the way past.
  expect(await page.$(editor(1))).toBeNull();
  expect(await noteTexts(board)).toEqual(["first note, reworded", "third note"]);

  // And a third click keeps working, which is what "the click is swallowed"
  // would have broken from here on.
  await page.click(pencil(3));
  await page.waitForSelector(editor(3));
  expect(await page.inputValue(editor(3))).toBe("third note");
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

// ── TQ-0079 ─────────────────────────────────────────────────────
//
// TQ-0010 merged the notes half of the body and left the content half as the
// snapshot the dialog opened with, so a save that never touched the textarea
// still wrote that snapshot back over whatever had been written above the
// notes rule. TQ-0069 answered the first half by construction — a field with
// no editor open on it is never written at all — and kept the second: a
// paragraph both sides rewrote is refused rather than merged.

test("editing another field leaves a description an agent rewrote alone", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");
  await page.waitForFunction(
    () => document.querySelector("#task-body .markdown p")?.textContent === "Revised by an agent.",
  );

  await choose(page, "task-priority", "urgent");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.priority).toBe("urgent");
  expect(task?.body).toContain("Revised by an agent.");
  expect(task?.body).not.toContain("As filed.");
});

test("a description edited on both sides is refused, and the typing is kept", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  await openEditor(page, "task-body", "## Finding\n\nRewritten in the dialog.");
  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");
  await page.waitForSelector("#task-changed:not([hidden])");

  await page.click(".inline-actions button.primary");

  const toast = await page.waitForSelector("#toasts .toast.error");
  const message = await toast.textContent();
  expect(message).toContain(id);
  expect(message).toContain("changed on disk");

  // The editor is still open and still holds what the user typed: refusing is
  // only worth anything if the text it refused to write is still recoverable.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  expect(await page.inputValue("#task-body-edit")).toBe("## Finding\n\nRewritten in the dialog.");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Revised by an agent.");
  expect(task?.body).not.toContain("Rewritten in the dialog.");
});

// The refusal is about the content half alone. A comparison of whole bodies
// would refuse this one too, and appending a note is the commonest thing to
// happen under an open dialog.
test("a note appended under the dialog does not block a description rewritten here", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  await openEditor(page, "task-body", "## Finding\n\nRewritten in the dialog.");
  project.mustRun("note", id, "--", "arrived while editing");
  await page.waitForSelector("#task-notes .note .note-text");

  await page.click(".inline-actions button.primary");
  await page.waitForSelector("#task-body-edit", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Rewritten in the dialog.");
  expect(task?.body).toContain("arrived while editing");
  expect(task?.body).not.toContain("As filed.");
});

// TQ-0010's property from the other side: with the description never opened,
// nothing writes a body at all, so a note appended under the dialog survives on
// the file's own terms rather than by being merged into one.
test("a note appended under the dialog survives an edit to another field", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  project.mustRun("note", id, "--", "written by an agent");
  await page.waitForSelector("#task-notes .note .note-text");

  await choose(page, "task-priority", "urgent");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.priority).toBe("urgent");
  expect(task?.body).toContain("As filed.");
  expect(task?.body).toContain("written by an agent");
});

// A write that fails is not a write that refused: the file did not move, so
// nothing is stranded and the very next attempt has to go through. This used
// to need a rebase of the dialog's baseline after every write; there is no
// baseline now, and this is what proves it is not needed rather than missing.
test("a note that fails to land leaves the next write to go through", async () => {
  const board = await dialogWithBody("## Finding\n\nAs filed.");
  const { project, server, page, id } = board;

  const notesRoute = `**/api/tasks/${id}/notes`;
  await page.route(notesRoute, (route) =>
    route.fulfill({ status: 500, contentType: "application/json", body: `{"error":"nope"}` }),
  );

  await page.fill("#task-note", "the note that never lands");
  await page.click("#task-note-add");

  const failed = await page.waitForSelector("#toasts .toast.error");
  expect(await failed.textContent()).toContain("Could not add a note");
  // The text is kept, because nothing was written with it.
  expect(await page.inputValue("#task-note")).toBe("the note that never lands");

  await page.unroute(notesRoute);
  await editBody(page, "## Finding\n\nRewritten after the note failed.");
  await editField(page, "task-title", "Retitled after the note failed");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled after the note failed");
  expect(task?.body).toContain("Rewritten after the note failed.");
});

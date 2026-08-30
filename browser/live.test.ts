/**
 * The open task dialog while its own task changes on disk.
 *
 * The board does not freeze for a dialog (TQ-0084), so the task under one moves
 * while it is being read: an agent claims it, appends a note, retitles it, or
 * deletes it outright. TQ-0069 made that the ordinary case rather than a case
 * to be reconciled — the dialog holds no draft of the task, so every field it
 * is not being edited in simply *is* the file, and there is nothing to adopt.
 *
 * What is left to prove is the other half, and it is the ticket's own rule:
 * an editor is never overwritten, and a write whose field moved underneath it
 * is refused outright — no merge, nothing written, the user's text still on
 * screen and the collision left for them to look at in the VCS.
 *
 * The arithmetic is in frontend/edit.test.ts. This is the layer that proves
 * the dialog wires it up, because "in the middle of" means focus and a caret
 * and only a browser has those.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { POLL_INTERVAL_MS, card, cardIn, editBody, openEditor, useBoard, type Board } from "./harness";

const openBoard = useBoard();

/** Comfortably inside one poll interval, so nothing here can be the poll. */
const BEFORE_A_POLL = { timeout: POLL_INTERVAL_MS - 1000 };

/** Long enough that a change the stream carries has certainly arrived. */
const AFTER_THE_STREAM_MS = 1500;

/** A task with its dialog already open, ready to be changed underneath. */
async function openDialog(title = "Under an agent's hands"): Promise<Board & { id: string }> {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add(title);
  });

  await board.page.click(cardIn("todo", id));
  await board.page.waitForSelector("#task-dialog[open]");
  return { ...board, id };
}

/** Waits for a field the dialog renders as text to say something. */
function reads(page: Page, selector: string, value: string): Promise<unknown> {
  return page.waitForFunction(
    ([target, wanted]) =>
      document.querySelector(target as string)?.textContent?.trim() === wanted,
    [selector, value],
    BEFORE_A_POLL,
  );
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

test("a note written on the CLI appears in the open dialog's panel", async () => {
  const { page, project, id } = await openDialog();

  // The whole point of TQ-0084: an agent running `tq note` against the task on
  // screen, while a human reads it.
  project.mustRun("note", id, "--", "written while you were looking at it");

  await page.waitForSelector("#task-notes .note .note-text", BEFORE_A_POLL);
  expect(await page.textContent("#task-notes .note .note-text")).toBe(
    "written while you were looking at it",
  );
});

// No adoption rule, no dirty flags: a field nobody has an editor open on is
// drawn from the task the board last read, so it follows the file by being it.
test("every field nobody is editing follows the file", async () => {
  const { page, project, id } = await openDialog();

  project.mustRun("update", id, "--title", "Retitled by an agent", "--assignee", "agent-api");
  project.mustRun("update", id, "--add-label", "backend");
  project.mustRun("move", id, "in-progress");

  await reads(page, "#task-title", "Retitled by an agent");
  await reads(page, "#task-assignee", "agent-api");
  await page.waitForFunction(
    () => (document.querySelector("#task-status") as HTMLSelectElement | null)?.value === "in-progress",
    undefined,
    BEFORE_A_POLL,
  );
  expect(await page.textContent("#task-labels")).toContain("backend");
  // Nothing is being edited, so there is nothing to warn about.
  expect(await page.$("#task-changed:not([hidden])")).toBeNull();
});

test("the body follows the file until an editor is opened on it", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands", "--body", "## Finding\n\nAs filed.");
  });
  const { page } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  // Rendered, not raw: the dialog shows the body as Markdown (TQ-0069).
  expect(await page.textContent("#task-body .markdown h2")).toBe("Finding");

  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");
  await reads(page, "#task-body .markdown p", "Revised by an agent.");
});

// The ticket's rule, in the smallest case there is: an editor is open, the file
// moves under that very field, and the dialog says so while the user is still
// typing rather than when they finish.
test("a field the file moved under an open editor is said, and the write refused", async () => {
  const { page, project, server, id } = await openDialog();

  await openEditor(page, "task-title", "Retitled in the dialog");
  project.mustRun("update", id, "--title", "Retitled by an agent");

  await page.waitForSelector("#task-changed:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-changed")).toContain("title");
  expect(await page.inputValue("#task-title-edit")).toBe("Retitled in the dialog");

  await page.press("#task-title-edit", "Enter");

  const toast = await page.waitForSelector("#toasts .toast.error");
  expect(await toast.textContent()).toContain("changed on disk");

  // Nothing was written, and the text that was refused is still on screen: a
  // refusal is only worth anything if what it refused can still be copied.
  expect(await page.inputValue("#task-title-edit")).toBe("Retitled in the dialog");
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled by an agent");
});

// The way out of a refusal that will not clear. Nothing merges the two, so the
// only thing the dialog can offer is the file's own value back.
test("Escape out of a refused edit puts the file's value back", async () => {
  const { page, project, id } = await openDialog();

  await openEditor(page, "task-title", "Retitled in the dialog");
  project.mustRun("update", id, "--title", "Retitled by an agent");
  await page.waitForSelector("#task-changed:not([hidden])", BEFORE_A_POLL);

  await page.press("#task-title-edit", "Escape");
  await page.waitForSelector("#task-title-edit", { state: "detached" });
  expect(await page.textContent("#task-title")).toBe("Retitled by an agent");
  expect(await page.$("#task-changed:not([hidden])")).toBeNull();
});

// A field with no editor to open is the same question asked at the other end:
// somebody chose from a select showing a value the file no longer holds.
test("a select used against a value the file has moved past is refused", async () => {
  const { page, project, server, id } = await openDialog();

  // The board is stopped from hearing about the change, so the select is still
  // offering what the dialog opened with when it is used.
  await page.route("**/api/tasks", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "[]" }),
  );
  project.mustRun("update", id, "--priority", "urgent");
  await page.waitForTimeout(AFTER_THE_STREAM_MS);

  await page.selectOption("#task-priority", "low");
  const toast = await page.waitForSelector("#toasts .toast.error");
  expect(await toast.textContent()).toContain("changed on disk");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.priority).toBe("urgent");
  // And the control is put back, because nothing local would re-render it.
  expect(await page.inputValue("#task-priority")).toBe("normal");
});

// The body is two fields in one string, and this is the half that must not
// refuse: appending a note is the commonest thing to happen under an open
// dialog, and it is not a change to the paragraph above the rule.
test("a note appended under an open description editor does not block it", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands", "--body", "## Finding\n\nAs filed.");
  });
  const { page, project, server } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  await openEditor(page, "task-body", "## Finding\n\nRewritten in the dialog.");
  project.mustRun("note", id, "--", "arrived while editing");
  await page.waitForTimeout(AFTER_THE_STREAM_MS);

  await page.click(".inline-actions button.primary");
  await page.waitForSelector("#task-body-edit", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Rewritten in the dialog.");
  expect(task?.body).toContain("arrived while editing");
  expect(task?.body).not.toContain("As filed.");
});

// And the half that must: both sides rewrote the paragraph, and there is no
// honest answer, so nothing is written at all.
test("a description rewritten on both sides is refused, and the typing is kept", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands", "--body", "## Finding\n\nAs filed.");
  });
  const { page, project, server } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  await openEditor(page, "task-body", "## Finding\n\nRewritten in the dialog.");
  await reviseBody(board, id, "## Finding\n\nRevised by an agent.");
  await page.waitForSelector("#task-changed:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-changed")).toContain("description");

  await page.click(".inline-actions button.primary");
  const toast = await page.waitForSelector("#toasts .toast.error");
  expect(await toast.textContent()).toContain("changed on disk");

  expect(await page.inputValue("#task-body-edit")).toBe("## Finding\n\nRewritten in the dialog.");
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Revised by an agent.");
  expect(task?.body).not.toContain("Rewritten in the dialog.");
});

// A description written here while notes land under it must not take those
// notes with it — TQ-0010's property, on the path TQ-0069 built.
test("writing the description keeps the notes the file gained under it", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands", "--body", "## Finding\n\nAs filed.");
    project.mustRun("note", id, "--", "the note the dialog opened with");
  });
  const { page, project, server } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  project.mustRun("note", id, "--", "written by an agent");
  await page.waitForSelector("#task-notes .note:nth-of-type(2)", BEFORE_A_POLL);

  await editBody(page, "## Finding\n\nRewritten in the dialog.");

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.body).toContain("Rewritten in the dialog.");
  expect(task?.body).toContain("the note the dialog opened with");
  expect(task?.body).toContain("written by an agent");
});

// The panel used to name the note under edit by its position in the list. The
// list follows the file, so a position is a name that can come to mean a
// different note between two keystrokes — and the edit would land on that one.
test("a note being edited keeps its identity when the list changes underneath", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands");
    for (const text of ["the first note", "the second note", "the third note"]) {
      project.mustRun("note", id, "--", text);
    }
  });
  const { page, project, server } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  await page.click("#task-notes .note:nth-of-type(2) button.icon");
  await page.fill("#task-notes .note-editor", "the second note, reworded");

  // The note *above* the one being edited comes off the file, so every position
  // below it shifts by one while the editor is open.
  const kept = (await project.tasks(server)).find((candidate) => candidate.id === id)?.body ?? "";
  await reviseBody(
    board,
    id,
    kept
      .split("\n")
      .filter((line) => !line.includes("the first note"))
      .join("\n"),
  );

  await page.waitForFunction(
    () => document.querySelectorAll("#task-notes .note").length === 2,
    undefined,
    BEFORE_A_POLL,
  );

  // Rebuilding the list takes the focused textarea with it, which writes the
  // edit the way losing focus always does. Which note it was written *to* is
  // the point: by position it would have landed on the third note.
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });
  const texts = await page.$$eval("#task-notes .note .note-text", (items) =>
    items.map((item) => item.textContent ?? ""),
  );
  expect(texts).toEqual(["the second note, reworded", "the third note"]);

  const written = (await project.tasks(server)).find((candidate) => candidate.id === id)?.body ?? "";
  expect(written).toContain("the second note, reworded");
  expect(written).not.toContain("the third note, reworded");
});

// The note under an editor going off the file is the one collision that would
// otherwise be silent: the reword has nowhere to land, and dropping it without
// a word is exactly what this ticket refuses.
test("a note that leaves the file under its own editor keeps the typing on screen", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands");
    project.mustRun("note", id, "--", "the note to lose");
  });
  const { page } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "reworded, and about to be orphaned");
  await reviseBody(board, id, "");

  await page.waitForSelector("#task-note-detached", BEFORE_A_POLL);
  expect(await page.inputValue("#task-notes .note-editor")).toBe(
    "reworded, and about to be orphaned",
  );
});

// A scan the store could not square with the directory comes back a task short
// on purpose (TQ-0012). That is not a deletion, and the dialog must not say it
// is — the task is back on the very next scan. Saying "deleted" over a file it
// could not parse, or an ID two files claim, would be the same red line.
test("a task missing from a listing is not called deleted while its file is there", async () => {
  const { page, project, id } = await openDialog();

  await page.route("**/api/tasks", (route) =>
    route.fulfill({ status: 200, contentType: "application/json", body: "[]" }),
  );
  project.add("A change, so the board asks");
  await page.waitForFunction(() => document.querySelectorAll(".card").length === 0, undefined, {
    timeout: 10_000,
  });

  // Long enough that the answer has certainly come back, so this is the dialog
  // deciding rather than the dialog not having got round to it.
  await page.waitForTimeout(AFTER_THE_STREAM_MS);
  expect(await page.$("#task-gone:not([hidden])")).toBeNull();
  expect(await page.$("#task-dialog[open]")).not.toBeNull();

  // And when the file really does go, the listing agrees for a reason.
  await page.unroute("**/api/tasks");
  project.remove(id);
  await page.waitForSelector("#task-gone:not([hidden])", { timeout: 10_000 });
  expect(await page.textContent("#task-gone")).toContain(id);
});

test("the open task leaving the queue is said in the dialog", async () => {
  const { page, project, id } = await openDialog();

  await openEditor(page, "task-title", "half-typed when it vanished");
  project.remove(id);

  await page.waitForSelector("#task-gone:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-gone")).toContain(id);

  // Said in the dialog, not by unmounting it: the text is still recoverable,
  // which is the whole reason a refresh is allowed under an open dialog at all.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  expect(await page.inputValue("#task-title-edit")).toBe("half-typed when it vanished");
});

test("typing in a composer survives a change arriving mid-draft, caret included", async () => {
  const { page, project } = await openBoard();

  await page.click(".column[data-status='todo'] .composer-open");
  const input = ".column[data-status='todo'] .composer-input";
  await page.waitForSelector(input);
  await page.fill(input, "half-typed draft");

  // Mid-word, where a remount would leave the caret at one end or the other.
  const area = page.locator(input);
  await area.evaluate((element) => (element as HTMLTextAreaElement).setSelectionRange(5, 5));

  const arrived = project.add("Arrived while composing");
  await page.waitForSelector(card(arrived), BEFORE_A_POLL);

  expect(await page.inputValue(input)).toBe("half-typed draft");
  expect(await area.evaluate((element) => (element as HTMLTextAreaElement).selectionStart)).toBe(5);

  // The proof that both survived: the next keystroke lands where it was aimed.
  await page.keyboard.type("X");
  expect(await page.inputValue(input)).toBe("half-Xtyped draft");
});

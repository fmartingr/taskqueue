/**
 * The open task dialog while its own task changes on disk (TQ-0084).
 *
 * The board no longer freezes for a dialog, so the task under one moves while
 * it is being edited: an agent claims it, appends a note, retitles it, or
 * deletes it outright. What the dialog does about that is a rule with two
 * halves — follow the file for anything the user is not in the middle of, never
 * overwrite anything they are — and only a real browser can show it, because
 * "in the middle of" means focus and a caret.
 *
 * The arithmetic underneath is in frontend/adopt.test.ts, which also proves the
 * adoption and the save-time merge agree on one snapshot. This is the layer
 * that proves the dialog wires them together.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { POLL_INTERVAL_MS, card, cardIn, useBoard, type Board } from "./harness";

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

/** Waits for a form control in the dialog to hold a given value. */
function holds(page: Page, selector: string, value: string): Promise<unknown> {
  return page.waitForFunction(
    ([target, wanted]) =>
      (document.querySelector(target as string) as HTMLInputElement | null)?.value === wanted,
    [selector, value],
    BEFORE_A_POLL,
  );
}

test("a note written on the CLI appears in the open dialog's panel", async () => {
  const { page, project, id } = await openDialog();

  // The whole point of the ticket: an agent running `tq note` against the task
  // on screen, while a human reads it.
  project.mustRun("note", id, "--", "written while you were looking at it");

  await page.waitForSelector("#task-notes .note .note-text", BEFORE_A_POLL);
  expect(await page.textContent("#task-notes .note .note-text")).toBe(
    "written while you were looking at it",
  );
});

test("a field nobody has touched adopts what the file holds", async () => {
  const { page, project, id } = await openDialog();

  project.mustRun("update", id, "--title", "Retitled by an agent", "--assignee", "agent-api");
  project.mustRun("move", id, "in-progress");

  await holds(page, "#task-title", "Retitled by an agent");
  await holds(page, "#task-assignee", "agent-api");
  await holds(page, "#task-status", "in-progress");
  // Nothing was overridden, so the dialog has nothing to report.
  expect(await page.$("#task-changed:not([hidden])")).toBeNull();
});

test("a field the user has edited is kept, and the dialog says the file moved", async () => {
  const { page, project, server, id } = await openDialog();

  await page.fill("#task-title", "Retitled in the dialog");
  project.mustRun("update", id, "--title", "Retitled by an agent");

  await page.waitForSelector("#task-changed:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-changed")).toContain("Title");
  expect(await page.inputValue("#task-title")).toBe("Retitled in the dialog");

  // Saying so is not the same as refusing: the save still writes what the user
  // typed. Only the body has no honest merge and is refused outright (TQ-0079).
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Retitled in the dialog");
});

// The refinement TQ-0084 flagged rather than smuggled in: a field under the
// caret has not been edited, but replacing a value someone is about to type
// over is the same surprise. It is a deferral, not a refusal, so nothing is
// dropped — the value lands the moment the caret leaves.
test("a field under the caret is left alone until the caret leaves it", async () => {
  const { page, project, id } = await openDialog();

  await page.focus("#task-assignee");
  project.mustRun("update", id, "--assignee", "agent-api");
  await page.waitForTimeout(AFTER_THE_STREAM_MS);

  expect(await page.inputValue("#task-assignee")).toBe("");
  // Deferring is not overriding: nothing was typed, so there is nothing to say.
  expect(await page.$("#task-changed:not([hidden])")).toBeNull();

  await page.focus("#task-title");
  await holds(page, "#task-assignee", "agent-api");
});

// The deferral is the one hold that says nothing, so the save has to settle it
// rather than trust the field. Enter in a text input submits the form without
// ever moving the focus, so focusout alone would let the stale value the field
// was still showing be written straight back over the agent's.
test("a deferred field is settled by the save rather than written back stale", async () => {
  const { page, project, server, id } = await openDialog();

  await page.focus("#task-assignee");
  project.mustRun("update", id, "--assignee", "agent-api");
  await page.waitForTimeout(AFTER_THE_STREAM_MS);
  expect(await page.inputValue("#task-assignee")).toBe("");

  // Enter from inside the deferred field itself: the form submits and the
  // focus never leaves.
  await page.press("#task-assignee", "Enter");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.assignee).toBe("agent-api");
});

// The panel used to name the note under edit by its position in the list. The
// list follows the file now, so a position is a name that can come to mean a
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
  const response = await fetch(`${server.url}/api/tasks/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      body: kept
        .split("\n")
        .filter((line) => !line.includes("the first note"))
        .join("\n"),
    }),
  });
  if (!response.ok) throw new Error(`PATCH /api/tasks/${id} = ${response.status}`);

  await page.waitForFunction(
    () => document.querySelectorAll("#task-notes .note").length === 2,
    undefined,
    BEFORE_A_POLL,
  );

  // Rebuilding the list takes the focused textarea with it, which keeps the
  // edit the way losing focus always does. Which note it was kept *on* is the
  // point: by position it landed on the third note.
  const texts = await page.$$eval("#task-notes .note .note-text", (items) =>
    items.map((item) => item.textContent ?? ""),
  );
  expect(texts).toEqual(["the second note, reworded", "the third note"]);
});

// A scan the store could not square with the directory comes back a task short
// on purpose (TQ-0012). That is not a deletion, and the dialog must not say it
// is — the task is back on the very next scan.
// A listing without the task is not proof of anything: the store returns a
// short one rather than pretend it read the whole directory (TQ-0012), and
// leaves out a file it could not parse or one whose ID two files claim. Saying
// "deleted" over any of those would be a red line about a task that is fine.
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

test("the body follows the file until the user types in it", async () => {
  let id = "";
  const board = await openBoard((project) => {
    id = project.add("Under an agent's hands", "--body", "## Finding\n\nAs filed.");
  });
  const { page, server } = board;
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  const revise = async (body: string) => {
    const response = await fetch(`${server.url}/api/tasks/${id}`, {
      method: "PATCH",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ body }),
    });
    if (!response.ok) throw new Error(`PATCH /api/tasks/${id} = ${response.status}`);
  };

  await revise("## Finding\n\nRevised by an agent.");
  await holds(page, "#task-body", "## Finding\n\nRevised by an agent.");

  // Now the user takes it over, and the next revision cannot have it back.
  await page.fill("#task-body", "## Finding\n\nRewritten in the dialog.");
  await page.focus("#task-title");
  await revise("## Finding\n\nRevised again.");

  await page.waitForSelector("#task-changed:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-changed")).toContain("Body");
  expect(await page.inputValue("#task-body")).toBe("## Finding\n\nRewritten in the dialog.");
});

// The adoption moves the dialog's baseline with it, or the save-time merge
// would read the note the dialog adopted as a note the dialog invented — and
// write it a second time (TQ-0010's arithmetic, on TQ-0084's new path).
test("a note adopted while the dialog was open is not written twice by the save", async () => {
  const { page, project, server, id } = await openDialog();

  project.mustRun("note", id, "--", "written by an agent");
  await page.waitForSelector("#task-notes .note .note-text", BEFORE_A_POLL);

  await page.fill("#task-title", "Saved after the note arrived");
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Saved after the note arrived");
  const written = task?.body ?? "";
  expect(written).toContain("written by an agent");
  expect(written.split("written by an agent").length - 1).toBe(1);
});

test("a note adopted and then reworded here is saved reworded, once", async () => {
  const { page, project, server, id } = await openDialog();

  project.mustRun("note", id, "--", "written by an agent");
  await page.waitForSelector("#task-notes .note .note-text", BEFORE_A_POLL);

  await page.click("#task-notes .note button.icon");
  await page.fill("#task-notes .note-editor", "reworded after it arrived");
  await page.press("#task-notes .note-editor", "Enter");
  await page.waitForSelector("#task-notes .note-editor", { state: "detached" });

  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  const written = task?.body ?? "";
  expect(written).toContain("reworded after it arrived");
  expect(written).not.toContain("written by an agent");
});

test("the open task leaving the queue is said in the dialog", async () => {
  const { page, project, id } = await openDialog();

  await page.fill("#task-title", "half-typed when it vanished");
  project.remove(id);

  await page.waitForSelector("#task-gone:not([hidden])", BEFORE_A_POLL);
  expect(await page.textContent("#task-gone")).toContain(id);

  // Said in the dialog, not by unmounting it: the text is still recoverable,
  // which is the whole reason a refresh is allowed under an open dialog at all.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  expect(await page.inputValue("#task-title")).toBe("half-typed when it vanished");
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

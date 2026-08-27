/**
 * The board in a real browser: drag and drop, the inline composer, the two
 * dialogs and the keyboard that opens one. Every assertion about what changed
 * is made against the API rather than the page, so a render that lies is a
 * failure and not a pass.
 */

import { expect, test } from "bun:test";
import { readFileSync, writeFileSync } from "node:fs";
import { join } from "node:path";
import { card, cardIn, idsIn, useBoard, type Project, type Server, type Task } from "./harness";

const openBoard = useBoard();

test("the board renders the tasks the CLI wrote", async () => {
  let todo = "";
  let doing = "";
  const { page } = await openBoard((project) => {
    todo = project.add("Something to do");
    doing = project.add("Already moving", "--status", "in-progress");
  });

  expect(await idsIn(page, "todo")).toEqual([todo]);
  expect(await idsIn(page, "in-progress")).toEqual([doing]);
  expect(await page.textContent(cardIn("todo", todo))).toContain("Something to do");
});

test("dragging a card to another column moves the task on the server", async () => {
  let id = "";
  const { project, server, page } = await openBoard((p) => {
    id = p.add("Drag me");
  });

  await page.dragAndDrop(card(id), ".column[data-status='in-progress']");

  // The board shows the move…
  await page.waitForSelector(cardIn("in-progress", id));
  expect(await idsIn(page, "todo")).toEqual([]);

  // …and, which is the point, so does the server.
  const tasks = await project.tasks(server);
  expect(tasks.find((task) => task.id === id)?.status).toBe("in-progress");
});

test("dropping a card back on its own column is not a move", async () => {
  let id = "";
  const { project, server, page } = await openBoard((p) => {
    id = p.add("Stay put");
  });

  const before = (await project.tasks(server)).find((task) => task.id === id)?.updated;
  await page.dragAndDrop(card(id), ".column[data-status='todo']");
  await page.waitForSelector(cardIn("todo", id));

  const after = (await project.tasks(server)).find((task) => task.id === id);
  expect(after?.status).toBe("todo");
  expect(after?.updated).toBe(before);
});

test("the inline composer files a card, stays open, and escapes out", async () => {
  const { project, server, page } = await openBoard();

  await page.click(".column[data-status='todo'] .composer-open");
  const input = ".column[data-status='todo'] .composer-input";
  await page.waitForSelector(input);

  await page.fill(input, "Composed here");
  await page.press(input, "Enter");

  // The card lands on the board and on the server…
  await page.waitForSelector(`.column[data-status='todo'] .card:has-text("Composed here")`);
  const tasks = await project.tasks(server);
  expect(tasks.map((task) => task.title)).toEqual(["Composed here"]);
  expect(tasks[0].status).toBe("todo");

  // …and the composer is still open and focused, ready for the next one.
  await page.waitForSelector(input);
  expect(await page.evaluate(() => document.activeElement?.className)).toBe("composer-input");

  // Escape closes it without filing anything.
  await page.press(input, "Escape");
  await page.waitForSelector(".column[data-status='todo'] .composer-open");
  expect(await page.$(input)).toBeNull();
  expect((await project.tasks(server)).length).toBe(1);
});

test("the composer files into the column it was opened in", async () => {
  const { project, server, page } = await openBoard();

  await page.click(".column[data-status='in-progress'] .composer-open");
  const input = ".column[data-status='in-progress'] .composer-input";
  await page.waitForSelector(input);
  await page.fill(input, "Started already");
  await page.press(input, "Enter");

  await page.waitForSelector(`.column[data-status='in-progress'] .card:has-text("Started already")`);
  expect((await project.tasks(server))[0].status).toBe("in-progress");
});

// `composing` is one shared value: only one composer is open at a time. Filing
// a card awaits the server, so by the time it closes itself the user may have
// opened a composer somewhere else — and closing theirs would file whatever
// they had typed so far, as a card titled with the first character or two.
test("filing a card leaves a composer opened in another column alone", async () => {
  const { project, server, page } = await openBoard();

  await page.click(".column[data-status='inbox'] .composer-open");
  const inbox = ".column[data-status='inbox'] .composer-input";
  await page.waitForSelector(inbox);
  await page.fill(inbox, "Filed from inbox");

  // Straight into another column. The click blurs the first composer, which
  // files it — and the close that follows lands after this one is already open.
  await page.click(".column[data-status='todo'] .composer-open");
  const todo = ".column[data-status='todo'] .composer-input";
  await page.waitForSelector(todo);
  await page.fill(todo, "Still typing this one");

  // Long enough for the first create and the refresh behind it to return.
  await page.waitForSelector(`.column[data-status='inbox'] .card:has-text("Filed from inbox")`);
  await page.waitForFunction(() => document.querySelectorAll(".composer-input").length > 0);

  expect(await page.isVisible(todo)).toBe(true);
  expect(await page.inputValue(todo)).toBe("Still typing this one");

  const titles = (await project.tasks(server)).map((task) => task.title);
  expect(titles).toEqual(["Filed from inbox"]);
});

test("the composer discards an empty draft rather than filing a blank card", async () => {
  const { project, server, page } = await openBoard();

  await page.click(".column[data-status='inbox'] .composer-open");
  const input = ".column[data-status='inbox'] .composer-input";
  await page.waitForSelector(input);
  await page.press(input, "Enter");

  await page.waitForSelector(".column[data-status='inbox'] .composer-open");
  expect(await project.tasks(server)).toEqual([]);
});

test("the task dialog opens on a card, saves every field, and closes", async () => {
  let id = "";
  const { project, server, page } = await openBoard((p) => {
    id = p.add("Before the edit");
  });

  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");
  expect(await page.textContent("#task-dialog-id")).toBe(id);
  expect(await page.inputValue("#task-title")).toBe("Before the edit");

  await page.fill("#task-title", "After the edit");
  await page.selectOption("#task-priority", "high");
  await page.fill("#task-assignee", "agent-api");
  await page.fill("#task-labels", "backend, auth");
  await page.fill("#task-body", "The body, written in the dialog.");
  await page.selectOption("#task-status", "in-progress");
  await page.click("#task-form button[type='submit']");

  // The dialog closes itself once the patch lands.
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task).toMatchObject({
    title: "After the edit",
    status: "in-progress",
    priority: "high",
    assignee: "agent-api",
    labels: ["backend", "auth"],
    body: "The body, written in the dialog.",
  });
  await page.waitForSelector(cardIn("in-progress", id));
});

test("cancelling the dialog leaves the task alone", async () => {
  let id = "";
  const { project, server, page } = await openBoard((p) => {
    id = p.add("Untouched");
  });

  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");
  await page.fill("#task-title", "Never saved");
  await page.click("#task-dialog [data-close='task-dialog'].close");

  await page.waitForSelector("#task-dialog[open]", { state: "detached" });
  const task = (await project.tasks(server)).find((candidate) => candidate.id === id);
  expect(task?.title).toBe("Untouched");
});

// Filing a task from the dialog is one gesture rather than four — the fields
// reach the API as the API wants them, the board catches up, and only then is
// anything said — so it is one test. `splitList` itself is a pure function with
// its own cases in frontend/format.test.ts; what a browser is here for is the
// wiring, that the comma-separated field reaches it at all (TQ-0080).
test("the create dialog files a task, waits for the board, and then says so", async () => {
  const { project, server, page } = await openBoard();

  await page.click("#new-task");
  await page.waitForSelector("#create-dialog[open]");

  // It opens on the title, so the dialog is typed into rather than clicked into.
  expect(await page.evaluate(() => document.activeElement?.id)).toBe("create-title");

  // The defaults a task filed without a status or a priority gets. That they
  // are read from the project's configuration rather than written into the
  // markup is what columns.test.ts and priorities.test.ts pin, on a board that
  // declares a vocabulary of its own.
  expect(await page.inputValue("#create-status")).toBe("inbox");
  expect(await page.inputValue("#create-priority")).toBe("normal");

  await page.fill("#create-title", "Filed from the dialog");
  await page.fill("#create-assignee", "agent-api");
  await page.fill("#create-labels", "backend, auth");
  await page.fill("#create-body", "Written in the dialog.");

  // Every listing is held from here on, so the board cannot catch up until this
  // test lets it. It is what turns "the dialog refreshes before it confirms"
  // into something with no timing in it: the confirmation the dialog would give
  // early has nothing to race, because the refresh it should be waiting on
  // physically cannot return.
  let release = (): void => {};
  const listings = new Promise<void>((resolve) => {
    release = resolve;
  });
  let holding = true;
  await page.route("**/api/tasks", async (route) => {
    if (holding && route.request().method() === "GET") await listings;
    await route.continue();
  });

  await page.click("#create-form button[type='submit']");

  // The task is on the server, so the POST is done and the dialog is inside its
  // refresh. Nothing may have been said yet, and no card may have appeared.
  const created = await waitForTask(project, server);
  // Read out of the page rather than through a locator: this has to answer
  // about the DOM as it stands, and every waiting form of the question would
  // sit here until the toast it is asserting the absence of turned up. Reading
  // the text rather than counting nodes is what makes the failure name what
  // was said too early.
  expect(await page.evaluate(() => document.querySelector("#toasts .toast.info")?.textContent ?? null)).toBeNull();
  expect(await idsIn(page, "inbox")).toEqual([]);

  holding = false;
  release();

  // The card is on the board by the time the confirmation is, and the two
  // render together, so reading the board straight after the toast reads the
  // same frame rather than a later one.
  await page.waitForSelector("#toasts .toast.info");
  const shown = await idsIn(page, "inbox");
  const message = await page.textContent("#toasts .toast.info");
  await page.waitForSelector("#create-dialog", { state: "detached" });

  expect(shown).toEqual([created.id]);
  expect(message).toContain(`Created ${created.id}`);

  // Two labels rather than one: the comma-separated field is split on its way
  // to the server.
  expect(created).toMatchObject({
    title: "Filed from the dialog",
    status: "inbox",
    priority: "normal",
    assignee: "agent-api",
    labels: ["backend", "auth"],
    body: "Written in the dialog.",
  });
});

/** Waits until the queue holds exactly one task and hands it back. Read from
 *  the API rather than the page, so it says the POST landed and nothing else. */
async function waitForTask(project: Project, server: Server): Promise<Task> {
  for (let attempt = 0; attempt < 100; attempt++) {
    const tasks = await project.tasks(server);
    if (tasks.length === 1) return tasks[0];
    await Bun.sleep(50);
  }
  throw new Error("the create never reached the server");
}

// A card is focusable and opens on a key as well as on a click. One board each,
// rather than one test pressing both: the second press would have to close the
// dialog the first opened and reopen it, and a `<dialog>` reopened in the
// millisecond it closed is swallowed (TQ-0081, rejected — the trigger stays).
test.each([
  ["Enter", "Enter"],
  ["Space", " "],
])("%s on a focused card opens its dialog", async (_name, key) => {
  let id = "";
  const { page } = await openBoard((project) => {
    id = project.add("Opened with a key");
  });

  // Asserted rather than assumed: focus() is a silent no-op on an element that
  // cannot take focus, and a card that lost its tabindex would fail below as a
  // dialog that never opened, saying nothing about why.
  await page.focus(cardIn("todo", id));
  expect(await page.evaluate(() => document.activeElement?.getAttribute("data-id"))).toBe(id);

  await page.keyboard.press(key);

  await page.waitForSelector("#task-dialog[open]");
  expect(await page.textContent("#task-dialog-id")).toBe(id);
});

test("a blocked card says what it is waiting for, in the board and the dialog", async () => {
  let blocker = "";
  let blocked = "";
  const { page } = await openBoard((project) => {
    blocker = project.add("Do this first");
    blocked = project.add("Then this", "--depends-on", blocker);
  });

  expect(await page.textContent(`${card(blocked)} .blocked-note`)).toContain(blocker);

  await page.click(cardIn("todo", blocked));
  await page.waitForSelector("#task-dialog[open]");
  expect(await page.textContent("#task-blocked")).toContain(blocker);
  expect(await page.inputValue("#task-depends-on")).toBe(blocker);
});

// Two files claiming one ID used to reach the board as two cards on a single
// key, and either of them 500d the moment it was dragged. Neither card is
// drawn now: the server withholds both copies and names the two files, and the
// board says a task is missing the same way it says a file was skipped — a
// toast, and a count in the footer for as long as it lasts (TQ-0040).
//
// A queue that would not hold still at all (TQ-0012's `incomplete`) travels
// the same path and is covered where it can be driven exactly, in the store's
// own tests: from a browser it would take a directory rewritten hard enough to
// lose three passes running, which is a race to wait on rather than a test.
test("an id two files claim is taken off the board and said so", async () => {
  let doubled = "";
  let healthy = "";
  const { page, project } = await openBoard((p) => {
    doubled = p.add("Claimed twice");
    healthy = p.add("Still here");
  });
  await page.waitForSelector(cardIn("todo", doubled));

  const original = readFileSync(join(project.dir, ".tasks", `${doubled}-claimed-twice.md`));
  writeFileSync(join(project.dir, ".tasks", `${doubled}-a-second-file.md`), original);

  const toast = await page.waitForSelector("#toasts .toast.error");
  expect(await toast.textContent()).toContain(`${doubled}-a-second-file.md`);

  // The doubled card is gone, the rest of the board stands, and the footer
  // keeps saying so after the toast has.
  await page.waitForFunction((id) => !document.querySelector(`[data-id="${id}"]`), doubled);
  expect(await idsIn(page, "todo")).toEqual([healthy]);
  await page.waitForFunction(() =>
    document.querySelector("#status-line")?.textContent?.includes("claimed by more than one file"),
  );
});

// One file nobody can parse used to empty the board: the listing failed, and
// with it the status line. It now costs only itself, and the board is what says
// so — the file is skipped on the server and named here (TQ-0011).
test("a task file that will not parse leaves the rest of the board standing", async () => {
  let healthy = "";
  const { page, project } = await openBoard((p) => {
    healthy = p.add("Still here");
  });
  await page.waitForSelector(cardIn("todo", healthy));

  // What a merge conflict in a committed .tasks/ looks like on disk.
  writeFileSync(
    join(project.dir, ".tasks", "TQ-0002-conflicted.md"),
    "<<<<<<< HEAD\n---\nid: TQ-0002\ntitle: mine\nstatus: todo\n=======\n---\nid: TQ-0002\ntitle: theirs\nstatus: done\n>>>>>>> other\n---\n",
  );

  const toast = await page.waitForSelector("#toasts .toast.error");
  expect(await toast.textContent()).toContain("TQ-0002-conflicted.md");

  // The healthy card is still drawn, and the footer keeps saying so after the
  // toast has gone.
  expect(await idsIn(page, "todo")).toEqual([healthy]);
  await page.waitForFunction(() =>
    document.querySelector("#status-line")?.textContent?.includes("could not be read"),
  );
});

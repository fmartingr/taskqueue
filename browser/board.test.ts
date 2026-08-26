/**
 * The board in a real browser: drag and drop, the inline composer and the task
 * dialog. Every assertion about what changed is made against the API rather
 * than the page, so a render that lies is a failure and not a pass.
 */

import { expect, test } from "bun:test";
import { card, cardIn, idsIn, useBoard } from "./harness";

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

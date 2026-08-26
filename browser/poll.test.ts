/**
 * The poll. `frontend/state.ts` refreshes the board every few seconds so that
 * work an agent does on the CLI shows up on its own, and stands down while the
 * user is dragging, composing or has a dialog open, so the board never moves
 * under their hands.
 *
 * These are the slow tests in the suite: proving that something does *not*
 * happen means outlasting the interval it would have happened in.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { POLL_INTERVAL_MS, card, cardIn, idsIn, useBoard, type OpenBoard } from "./harness";

const useBoardFor = useBoard();

/**
 * A board with the event stream refused, so these tests exercise the fallback
 * poll and nothing else.
 *
 * Without this they would all pass on the push from /api/events (TQ-0033) — and
 * keep passing if the poll were deleted, which is precisely the coverage the
 * fallback needs. The stream has its own tests in events.test.ts.
 */
const openBoard: OpenBoard = (seed) =>
  useBoardFor(seed, async (page) => {
    await page.route("**/api/events", (route) => route.abort());
  });

/** Long enough that a poll due since the last change has run. */
const AFTER_A_POLL = POLL_INTERVAL_MS + 750;
/** How long to wait for something the poll is expected to bring in. */
const WITHIN_A_POLL = { timeout: POLL_INTERVAL_MS * 3 };

/** The centre of an element, in page coordinates, for driving the mouse. */
async function centre(page: Page, selector: string): Promise<{ x: number; y: number }> {
  const box = await page.locator(selector).boundingBox();
  if (!box) throw new Error(`${selector} has no box to aim at`);
  return { x: box.x + box.width / 2, y: box.y + box.height / 2 };
}

test("a task the CLI creates appears on the board without a reload", async () => {
  const { project, page } = await openBoard();
  expect(await idsIn(page, "todo")).toEqual([]);

  const id = project.add("Written by an agent");

  await page.waitForSelector(cardIn("todo", id), WITHIN_A_POLL);
  expect(await page.textContent(cardIn("todo", id))).toContain("Written by an agent");
});

test("a move the CLI makes is picked up by the poll", async () => {
  let id = "";
  const { project, page } = await openBoard((p) => {
    id = p.add("Claimed by an agent");
  });

  project.mustRun("move", id, "in-progress");

  await page.waitForSelector(cardIn("in-progress", id), WITHIN_A_POLL);
  expect(await idsIn(page, "todo")).toEqual([]);
});

test("the poll stands down while the composer is open, and catches up after it", async () => {
  const { project, page } = await openBoard();

  await page.click(".column[data-status='todo'] .composer-open");
  const input = ".column[data-status='todo'] .composer-input";
  await page.waitForSelector(input);
  await page.fill(input, "half-typed draft");

  const id = project.add("Arrived while composing");
  await page.waitForTimeout(AFTER_A_POLL);

  // The board did not move: the card is not there and the draft is intact.
  expect(await page.$(card(id))).toBeNull();
  expect(await page.inputValue(input)).toBe("half-typed draft");

  // Closing the composer lets the next poll through.
  await page.press(input, "Escape");
  await page.waitForSelector(cardIn("todo", id), WITHIN_A_POLL);
});

test("the poll stands down while the task dialog is open, and catches up after it", async () => {
  let open = "";
  const { project, page } = await openBoard((p) => {
    open = p.add("Open me");
  });

  await page.click(card(open));
  await page.waitForSelector("#task-dialog[open]");
  await page.fill("#task-title", "an unsaved edit");

  const id = project.add("Arrived while the dialog was open");
  await page.waitForTimeout(AFTER_A_POLL);

  expect(await page.$(card(id))).toBeNull();
  expect(await page.inputValue("#task-title")).toBe("an unsaved edit");

  await page.click("#task-dialog [data-close='task-dialog'].close");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });
  await page.waitForSelector(cardIn("todo", id), WITHIN_A_POLL);
});

test("the poll stands down while the create dialog is open", async () => {
  const { project, page } = await openBoard();

  await page.click("#new-task");
  await page.waitForSelector("#create-dialog[open]");

  const id = project.add("Arrived while creating");
  await page.waitForTimeout(AFTER_A_POLL);
  expect(await page.$(card(id))).toBeNull();

  await page.click("#create-dialog [data-close='create-dialog'].close");
  await page.waitForSelector(cardIn("todo", id), WITHIN_A_POLL);
});

test("the poll stands down mid-drag, and the drop still lands", async () => {
  let dragged = "";
  const { project, server, page } = await openBoard((p) => {
    dragged = p.add("Held in the air");
  });

  // Pick the card up and hold it: dragstart has fired, so state.dragging is set.
  const from = await centre(page, card(dragged));
  const to = await centre(page, ".column[data-status='in-progress']");
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.mouse.move(to.x, to.y, { steps: 8 });

  const arrival = project.add("Arrived mid-drag");
  await page.waitForTimeout(AFTER_A_POLL);

  // A re-render here would have replaced the card under the pointer.
  expect(await page.$(card(arrival))).toBeNull();

  await page.mouse.up();

  // The drop still moved the task, and the poll resumes with both cards.
  await page.waitForSelector(cardIn("in-progress", dragged), WITHIN_A_POLL);
  await page.waitForSelector(cardIn("todo", arrival), WITHIN_A_POLL);
  const tasks = await project.tasks(server);
  expect(tasks.find((task) => task.id === dragged)?.status).toBe("in-progress");
});

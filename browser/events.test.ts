/**
 * The board's live connection.
 *
 * The point of the stream is latency: a task written in a terminal shows up
 * without the board asking. That is only observable in a browser against the
 * real binary, and only meaningfully if the assertion is faster than the poll —
 * a three-second fallback would make any of these pass eventually.
 */

import { expect, test } from "bun:test";
import { POLL_INTERVAL_MS, card, cardIn, centre, idsIn, useBoard } from "./harness";

const openBoard = useBoard();

/** Comfortably inside one poll interval, so nothing here can be the poll. */
const BEFORE_A_POLL = { timeout: POLL_INTERVAL_MS - 1000 };

test("a task written in a terminal appears without waiting for a poll", async () => {
  const { page, project } = await openBoard((project) => {
    project.add("Already here");
  });

  const id = project.add("Written by an agent");
  await page.waitForSelector(cardIn("todo", id), BEFORE_A_POLL);
});

test("a move made on the CLI reaches the board", async () => {
  let id = "";
  const { page, project } = await openBoard((project) => {
    id = project.add("Moves on its own");
  });
  await page.waitForSelector(cardIn("todo", id));

  project.mustRun("move", id, "in-progress");
  await page.waitForSelector(cardIn("in-progress", id), BEFORE_A_POLL);
});

// A dialog is a layer *above* the board rather than a hand on it: nothing moves
// under the user when a card appears behind a backdrop, and a modal can stay
// open for minutes while an agent works. So the board keeps up underneath one,
// and the dialog stays exactly where it was (TQ-0084).
test("a change that arrives while a dialog is open is applied while it is open", async () => {
  let open = "";
  const { page, project } = await openBoard((project) => {
    open = project.add("The task being looked at");
  });

  await page.click(cardIn("todo", open));
  await page.waitForSelector("#task-dialog[open]");

  const arrived = project.add("Filed while the dialog was open");
  await page.waitForSelector(card(arrived), BEFORE_A_POLL);

  // The board moved; the dialog did not, and is still open on its own task.
  expect(await page.$("#task-dialog[open]")).not.toBeNull();
  expect(await page.textContent("#task-dialog-id")).toBe(open);
  expect(await idsIn(page, "todo")).toEqual([open, arrived]);
});

// The one guard left. A native drag cannot survive a re-render: the element
// under the pointer would be replaced mid-gesture, and the drop would land on
// nothing. The stream has no next turn — it speaks only when something changed
// — so the signal is held rather than dropped, and lands on the drop.
test("a change that arrives mid-drag is held until the card lands", async () => {
  let dragged = "";
  const { page, project } = await openBoard((project) => {
    dragged = project.add("Held in the air");
  });

  const from = await centre(page, card(dragged));
  const to = await centre(page, ".column[data-status='in-progress']");
  await page.mouse.move(from.x, from.y);
  await page.mouse.down();
  await page.mouse.move(to.x, to.y, { steps: 8 });

  const arrived = project.add("Arrived mid-drag");
  await page.waitForTimeout(1000);
  expect(await page.$(card(arrived))).toBeNull();

  await page.mouse.up();
  await page.waitForSelector(cardIn("in-progress", dragged), BEFORE_A_POLL);
  await page.waitForSelector(cardIn("todo", arrived), BEFORE_A_POLL);
});

test("the footer says nothing about polling while the stream is up", async () => {
  const { page, project } = await openBoard((project) => {
    project.add("Anything");
  });

  // Waited for rather than sampled: `streaming` is null until the first
  // connection resolves, so reading the footer the moment the element exists
  // would race the EventSource opening and see whatever happened to be there.
  await page.waitForFunction(() => document.querySelector("#status-line")?.textContent?.includes("tasks"));
  const id = project.add("Proves the stream is up");
  await page.waitForSelector(cardIn("todo", id), BEFORE_A_POLL);

  expect(await page.textContent("#status-line")).not.toContain("polling");
});

// With the stream unavailable the board must still update, or a server or
// proxy that cannot do server-sent events would leave it silently stale. This
// is the one test that is allowed to be slow: it is waiting for the poll.
test("the board falls back to polling when the stream cannot connect", async () => {
  const { page, project } = await openBoard(undefined, async (page) => {
    // Refused before the page loads, so the board never has a stream at all.
    await page.route("**/api/events", (route) => route.abort());
  });

  await page.waitForFunction(
    () => document.querySelector("#status-line")?.textContent?.includes("polling") ?? false,
    undefined,
    { timeout: 10_000 },
  );

  const id = project.add("Filed with no stream");
  await page.waitForSelector(cardIn("todo", id), { timeout: POLL_INTERVAL_MS * 3 });
});

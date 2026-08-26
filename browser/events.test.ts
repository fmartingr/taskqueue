/**
 * The board's live connection.
 *
 * The point of the stream is latency: a task written in a terminal shows up
 * without the board asking. That is only observable in a browser against the
 * real binary, and only meaningfully if the assertion is faster than the poll —
 * a three-second fallback would make any of these pass eventually.
 */

import { expect, test } from "bun:test";
import { POLL_INTERVAL_MS, card, cardIn, idsIn, useBoard } from "./harness";

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

// The poll skips its turn when the user is busy, because another turn is three
// seconds away. The stream has no next turn — it speaks only when something
// changed — so a signal that arrives mid-drag has to be held, not dropped.
test("a change that arrives while a dialog is open is applied when it closes", async () => {
  let open = "";
  const { page, project } = await openBoard((project) => {
    open = project.add("The task being looked at");
  });

  await page.click(cardIn("todo", open));
  await page.waitForSelector("#task-dialog[open]");

  const arrived = project.add("Filed while the dialog was open");
  // The board must not move under the open dialog.
  await page.waitForTimeout(1000);
  expect(await idsIn(page, "todo")).toEqual([open]);

  await page.click("#task-dialog [data-close='task-dialog'].close");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  // And the moment it closes, the held change lands — again inside one poll,
  // so this is the queue draining rather than the fallback catching up.
  await page.waitForSelector(card(arrived), BEFORE_A_POLL);
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

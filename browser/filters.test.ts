/**
 * The filter bar's wiring, and the footer that counts what it left.
 *
 * The rules are not the subject: `visibleTasks` and `isReady` are unit-tested
 * in `frontend/board.test.ts`, against the same functions the board calls. What
 * only a browser can say is that the controls are bound to those rules at all —
 * a `v-model` dropped from a select is invisible to every other layer, and this
 * project has no component-mount layer to see it from (TQ-0080).
 *
 * The footer is read alongside, because it is what the board itself says the
 * filters left, and because it and the columns come off one render — so waiting
 * for it is also how the assertions after it know what they are looking at.
 * `statusLine` is a pure computed and its clauses would be cheaper still as
 * unit tests; that move is a follow-up on TQ-0080, not a reason to leave the
 * board's own readout unasserted here.
 *
 * The label and priority selects have their own coverage in labels.test.ts and
 * priorities.test.ts, because their options come from the project's vocabulary;
 * they appear here only in the reset, which has to clear all five.
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { idsIn, useBoard, type Board, type Project } from "./harness";

const openBoard = useBoard();

/**
 * How long a count is waited for. Comfortably shorter than the 30s bun gives a
 * test, which is the point: a wait that outlasts the test is torn down by the
 * teardown, and takes the file's Chromium — and every test after it — with it.
 */
const COUNT_TIMEOUT_MS = 10_000;

interface Seeded {
  /** Unblocked, in the one column `tq ready` offers work from. */
  ready: string;
  /** In progress, so neither ready nor in the todo column. */
  claimed: string;
  /** In the todo column, but waiting on the one above. */
  blocked: string;
}

/** Three tasks that every control on the bar tells apart, on a board of their
 *  own, already showing all three. One board per test: the reset test would
 *  otherwise be setting up the ones after it. */
async function openSeeded(): Promise<Board & Seeded> {
  let seeded: Seeded | undefined;
  const board = await openBoard((project: Project) => {
    const ready = project.add("Waiting to be picked up", "--assignee", "agent-api", "--label", "bug");
    const claimed = project.add("Already moving", "--status", "in-progress", "--assignee", "agent-web");
    const blocked = project.add("Not yet", "--assignee", "agent-web", "--depends-on", claimed);
    seeded = { ready, claimed, blocked };
  });
  if (seeded === undefined) throw new Error("the board opened without its tasks");

  // Nothing filtered: the footer counts the queue rather than a slice of it,
  // and waiting for it here is what makes every test below start from a board
  // that has finished rendering.
  await counts(board.page, "3 tasks");
  return { ...board, ...seeded };
}

/**
 * Waits for the footer to say what the board is showing, and says what it read
 * when it never did.
 *
 * It is both the assertion and the synchronisation: the count and the columns
 * come off the same render, so a footer that has caught up is a board that has.
 * The wait on its own would report only that it timed out, so a failure falls
 * through to an ordinary assertion, which names the line the board settled on.
 */
async function counts(page: Page, expected: string): Promise<void> {
  await page
    .waitForFunction(
      (want) => document.querySelector("#status-line")?.textContent?.startsWith(want) ?? false,
      expected,
      { timeout: COUNT_TIMEOUT_MS },
    )
    .catch(async () => {
      expect(await page.textContent("#status-line")).toContain(expected);
    });
}

test("the status filter shows one column's work, and the footer says how much", async () => {
  const { page, ready, claimed, blocked } = await openSeeded();

  await page.selectOption("#filter-status", "todo");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready, blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([]);

  // "any" is a filter too — it has to put the hidden column back.
  await page.selectOption("#filter-status", "");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);
});

test("the assignee box narrows the board to whoever is typed into it", async () => {
  const { page, ready, claimed, blocked } = await openSeeded();

  await page.fill("#filter-assignee", "agent-web");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  await page.fill("#filter-assignee", "");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready, blocked]);
});

test("ready only leaves the work that is actually offered", async () => {
  const { page, ready, blocked } = await openSeeded();

  // Both of the others go: one is in a column that offers no work, the other
  // is waiting on it.
  await page.check("#filter-ready");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready]);
  expect(await idsIn(page, "in-progress")).toEqual([]);

  await page.uncheck("#filter-ready");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready, blocked]);
});

test("reset clears every control on the bar at once", async () => {
  const { page, ready, claimed, blocked } = await openSeeded();

  await page.selectOption("#filter-status", "todo");
  await page.selectOption("#filter-priority", "normal");
  await page.fill("#filter-assignee", "agent-api");
  await page.selectOption("#filter-label", "bug");
  await page.check("#filter-ready");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready]);

  await page.click("#filter-reset");

  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([ready, blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  // The controls come back with it: the reset writes through the same bindings,
  // so a filter cleared in the state and left showing on the bar is a failure.
  expect(await page.inputValue("#filter-status")).toBe("");
  expect(await page.inputValue("#filter-priority")).toBe("");
  expect(await page.inputValue("#filter-assignee")).toBe("");
  expect(await page.inputValue("#filter-label")).toBe("");
  expect(await page.isChecked("#filter-ready")).toBe(false);
});

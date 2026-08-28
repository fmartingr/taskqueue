/**
 * The search bar, and the one thing only a browser can say about it: that the
 * query line and the filter bar are wired to the same state, in both
 * directions, and that the suggestion menu can be driven from the keyboard.
 *
 * The query language itself is not the subject — `parseQuery`, `formatQuery`
 * and `completeQuery` are unit-tested in `frontend/search.test.ts` against the
 * same functions the component calls. What is asserted here is what a `v-model`
 * dropped or a keydown swallowed would break and nothing else would see: the
 * caret, the `<input>`'s value, focus leaving the menu on Tab (TQ-0068).
 */

import { expect, test } from "bun:test";
import type { Page } from "playwright-core";
import { idsIn, useBoard, type Board, type Project } from "./harness";

const openBoard = useBoard();

const COUNT_TIMEOUT_MS = 10_000;

interface Seeded {
  urgent: string;
  bug: string;
  claimed: string;
}

/** Three tasks the query language can tell apart every way it knows how. */
async function openSeeded(): Promise<Board & Seeded> {
  let seeded: Seeded | undefined;
  const board = await openBoard((project: Project) => {
    const urgent = project.add("Push config changes", "--priority", "urgent", "--assignee", "agent-api");
    const bug = project.add("Fix the OIDC redirect", "--priority", "high", "--label", "bug");
    const claimed = project.add("Already moving", "--status", "in-progress", "--assignee", "agent web");
    seeded = { urgent, bug, claimed };
  });
  if (seeded === undefined) throw new Error("the board opened without its tasks");
  await counts(board.page, "3 tasks");
  return { ...board, ...seeded };
}

/** The footer is the assertion and the synchronisation at once, as in
 *  filters.test.ts: the count and the columns come off one render. */
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

/** What the menu is offering, in order. */
function options(page: Page): Promise<string[]> {
  return page.$$eval("#search-suggestions .search-option .search-option-label", (nodes) =>
    nodes.map((node) => node.textContent ?? ""),
  );
}

test("free text narrows the board to what carries it", async () => {
  const { page, bug } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "oidc");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([bug]);
});

test("a term typed into the query moves the control it stands for", async () => {
  const { page, urgent } = await openSeeded();

  await page.fill("#search-query", "priority=urgent");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  expect(await page.inputValue("#filter-priority")).toBe("urgent");

  // And back out again: the query is the filter state, so deleting the term
  // has to clear the select rather than leave the last one standing.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");
  expect(await page.inputValue("#filter-priority")).toBe("");
});

test("a control moved rewrites the query, so the two never disagree", async () => {
  const { page, claimed } = await openSeeded();

  await page.selectOption("#filter-status", "in-progress");
  await counts(page, "1 of 3 tasks");
  expect(await page.inputValue("#search-query")).toBe("status=in-progress");
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  // Reset owns the query too: it is one filter set with two editors.
  await page.click("#filter-reset");
  await counts(page, "3 tasks");
  expect(await page.inputValue("#search-query")).toBe("");
});

test("autocomplete offers the keys, then the project's own values", async () => {
  const { page, urgent } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "pri");
  expect(await options(page)).toEqual(["priority="]);

  // Enter takes the highlighted row, and a key stops at its equals so the
  // values for it come up on the same keystroke.
  await page.keyboard.press("Enter");
  expect(await page.inputValue("#search-query")).toBe("priority=");
  expect(await options(page)).toEqual(["urgent", "high", "normal", "low"]);

  await page.keyboard.press("Enter");
  expect(await page.inputValue("#search-query")).toBe("priority=urgent ");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
});

test("the arrows walk the menu and Escape sends it away without clearing", async () => {
  const { page, bug } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "priority=");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  expect(await page.inputValue("#search-query")).toBe("priority=high ");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([bug]);

  // A prefix narrows the labels, and the one that starts with it leads.
  await page.type("#search-query", "label=bu");
  expect((await options(page))[0]).toBe("bug");

  await page.keyboard.press("Escape");
  expect(await page.$("#search-suggestions")).toBeNull();
  // The query survives it: Escape dismisses the menu, not the search.
  expect(await page.inputValue("#search-query")).toBe("priority=high label=bu");
});

test("the menu does not trap focus", async () => {
  const { page } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "s");
  expect((await options(page)).length).toBeGreaterThan(0);

  await page.keyboard.press("Tab");
  expect(await page.evaluate(() => document.activeElement?.id)).not.toBe("search-query");
});

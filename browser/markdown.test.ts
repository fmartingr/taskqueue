/**
 * A task body, rendered, in a real browser.
 *
 * The markup itself is checked in frontend/markdown.test.ts, which is where a
 * case belongs unless a browser is the only thing that can show it. One is:
 * the dialog is a fixed width, and a GFM table is the one block a body can
 * make wider than that. It has to scroll inside its own box instead of
 * pushing the sheet sideways, and only a laid-out page knows the difference.
 */

import { expect, test } from "bun:test";
import { cardIn, useBoard } from "./harness";

const openBoard = useBoard();

/**
 * A table too wide for a 720px dialog, in the dialect GitHub would read. The
 * cells hold names rather than prose on purpose: a table of prose shrinks its
 * columns until it fits, and a table of identifiers is what actually overflows
 * — which is also what a task body's tables tend to hold.
 */
const WIDE_TABLE = [
  "| column one | column two | column three | column four |",
  "| --- | --- | ---: | --- |",
  "| TestAScanFailureIsReportedAgainToAFreshBoard" +
    " | TestTheSuiteIsIsolatedAgainstADeveloperShell" +
    " | 1234567890 | TestAListingIsCheckedAgainstItsOwnDirectory |",
].join("\n");

test("a table wider than the dialog scrolls inside it, and does not widen the sheet", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    id = project.add("A body with a table", "--body", WIDE_TABLE);
  });

  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  // The table reached the page as a table, alignment included.
  expect(await page.textContent("#task-body .markdown th:first-child")).toBe("column one");
  expect(await page.getAttribute("#task-body .markdown th:nth-child(3)", "style")).toBe(
    "text-align:right",
  );

  const fits = await page.evaluate(() => {
    const box = document.querySelector<HTMLElement>("#task-body .markdown .table-scroll");
    const sheet = document.querySelector<HTMLElement>("#task-dialog .task-sheet");
    if (!box || !sheet) throw new Error("no table box in the open dialog");
    return {
      boxScrolls: box.scrollWidth > box.clientWidth,
      sheetOverflow: sheet.scrollWidth - sheet.clientWidth,
    };
  });

  expect(fits.boxScrolls).toBe(true);
  expect(fits.sheetOverflow).toBeLessThanOrEqual(1);
});

test("a task list draws boxes that cannot be clicked", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    id = project.add("A body with a task list", "--body", "- [x] done\n- [ ] todo");
  });

  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");

  const boxes = page.locator("#task-body .markdown li.task-item input[type=checkbox]");
  expect(await boxes.count()).toBe(2);
  expect(await boxes.nth(0).isChecked()).toBe(true);
  expect(await boxes.nth(1).isChecked()).toBe(false);
  expect(await boxes.nth(1).isDisabled()).toBe(true);
});

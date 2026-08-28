/**
 * The project's configuration, live.
 *
 * `.taskqueue.yaml` decides the board's columns and both vocabularies, and the
 * board fetched it once at load: adding a label or renaming a column only
 * showed up on a reload. The event stream carries a `config` frame of its own
 * now (TQ-0034), and these are the assertions that it reaches the page.
 *
 * The half-saved file is the case that matters most. An editor leaves the
 * marker unparsable for a moment on every save, and a board that blanked its
 * labels for it would be worse than one that never updated at all.
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { POLL_INTERVAL_MS, card, useBoard, type Project } from "./harness";

const openBoard = useBoard();

/** Comfortably inside one poll interval, so nothing here can be the poll. */
const BEFORE_A_POLL = { timeout: POLL_INTERVAL_MS - 1000 };

/** Rewrites the project's marker, which is what an editor saving it does. */
function setConfig(project: Project, body: string): void {
  writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${body}`);
}

const RED = 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n';
const GREEN =
  'labels:\n  bug:\n    color: "#0e8a16"\n    display_name: Defect\n' +
  '  chore:\n    color: "#5319e7"\n    display_name: Chore\n';

/** The chips on a card, as the page renders them. */
const chips = (page: Awaited<ReturnType<typeof openBoard>>["page"], id: string) =>
  page.$$eval(`${card(id)} .label`, (nodes) =>
    nodes.map((node) => ({
      text: (node.textContent ?? "").trim(),
      background: getComputedStyle(node as HTMLElement).backgroundColor,
    })),
  );

/** The column headings, left to right. */
const headings = (page: Awaited<ReturnType<typeof openBoard>>["page"]) =>
  page.$$eval(".column h2", (nodes) => nodes.map((node) => (node.textContent ?? "").trim()));

/**
 * The labels the search bar offers, which are the vocabulary plus whatever
 * tasks carry that it does not declare.
 *
 * The box is emptied again before the assertion returns: `label=` narrows
 * nothing, but a query left in the line would be filtering the board every
 * test after this one looks at.
 */
async function offeredLabels(page: Awaited<ReturnType<typeof openBoard>>["page"]): Promise<string[]> {
  await page.click("#search-query");
  await page.fill("#search-query", "label=");
  const names = await page.$$eval("#search-suggestions .search-option-label", (nodes) =>
    nodes.map((node) => node.textContent ?? ""),
  );
  await page.fill("#search-query", "");
  await page.locator("#search-query").blur();
  return names;
}

test("a label recoloured in the marker repaints an open board", async () => {
  let id = "";
  const { page, project } = await openBoard((project) => {
    setConfig(project, RED);
    id = project.add("Carries a label", "--label", "bug");
  });

  expect(await chips(page, id)).toEqual([{ text: "Bug", background: "rgb(215, 58, 74)" }]);
  expect(await offeredLabels(page)).toEqual(["bug"]);

  setConfig(project, GREEN);

  await page.waitForFunction(
    (selector) => document.querySelector(selector)?.textContent?.trim() === "Defect",
    `${card(id)} .label`,
    BEFORE_A_POLL,
  );
  expect(await chips(page, id)).toEqual([{ text: "Defect", background: "rgb(14, 138, 22)" }]);

  // And a label the vocabulary gained is offered, though no task carries it.
  expect(await offeredLabels(page)).toEqual(["bug", "chore"]);
});

test("a column added to the marker appears without a reload", async () => {
  const { page, project } = await openBoard((project) => {
    setConfig(
      project,
      "columns:\n" +
        "  - {name: todo, display_name: To do, consider_ready: true, default: true}\n" +
        "  - {name: done, display_name: Done, consider_done: true}\n",
    );
    project.add("Something");
  });

  expect(await headings(page)).toEqual(["To do", "Done"]);

  setConfig(
    project,
    "columns:\n" +
      "  - {name: todo, display_name: To do, consider_ready: true, default: true}\n" +
      "  - {name: doing, display_name: Doing}\n" +
      "  - {name: done, display_name: Done, consider_done: true}\n",
  );

  await page.waitForFunction(() => document.querySelectorAll(".column").length === 3, undefined, BEFORE_A_POLL);
  expect(await headings(page)).toEqual(["To do", "Doing", "Done"]);
});

// The whole point of the ticket's warning: a file caught mid-save must not take
// the board down. The board says what is wrong and goes on drawing what it has.
test("a marker that will not parse leaves the board on its last good configuration", async () => {
  let id = "";
  const { page, project } = await openBoard((project) => {
    setConfig(project, RED);
    id = project.add("Carries a label", "--label", "bug");
  });
  expect(await chips(page, id)).toEqual([{ text: "Bug", background: "rgb(215, 58, 74)" }]);

  // What half of a save looks like on disk.
  writeFileSync(join(project.dir, ".taskqueue.yaml"), "version: 1\npath: .tasks\nlabels:\n  bug:\n    colo");

  const toast = await page.waitForSelector("#toasts .toast.error", BEFORE_A_POLL);
  expect(await toast.textContent()).toContain("configuration");

  // Nothing was blanked: the card, its chip and its colour are all still there.
  expect(await chips(page, id)).toEqual([{ text: "Bug", background: "rgb(215, 58, 74)" }]);
  expect(await headings(page)).toEqual(["Inbox", "To do", "In Progress", "Done", "Rejected"]);

  // And the editor finishes its save.
  setConfig(project, GREEN);
  await page.waitForFunction(
    (selector) => document.querySelector(selector)?.textContent?.trim() === "Defect",
    `${card(id)} .label`,
    BEFORE_A_POLL,
  );
});

// One scan on the server serves every connected board — that is what the event
// stream bought over each browser polling for itself. Two pages, one edit, and
// neither of them asked.
test("one edit to the marker reaches every connected board", async () => {
  let id = "";
  const board = await openBoard((project) => {
    setConfig(project, RED);
    id = project.add("Carries a label", "--label", "bug");
  });
  const other = await openBoard.another(board);

  const pages = [board.page, other.page];
  for (const page of pages) {
    expect(await chips(page, id)).toEqual([{ text: "Bug", background: "rgb(215, 58, 74)" }]);
  }

  setConfig(board.project, GREEN);

  // Waited on together, not one after the other: each page has the same one
  // poll interval from the edit to repaint in, so neither can be the fallback
  // catching up while the other is being asserted.
  await Promise.all(
    pages.map((page) =>
      page.waitForFunction(
        (selector) => document.querySelector(selector)?.textContent?.trim() === "Defect",
        `${card(id)} .label`,
        BEFORE_A_POLL,
      ),
    ),
  );

  for (const page of pages) {
    expect(await chips(page, id)).toEqual([{ text: "Defect", background: "rgb(14, 138, 22)" }]);
    expect(await offeredLabels(page)).toEqual(["bug", "chore"]);
  }
});

test("reverting the marker restores the board with no reload", async () => {
  let id = "";
  const { page, project } = await openBoard((project) => {
    setConfig(project, RED);
    id = project.add("Carries a label", "--label", "bug");
  });

  setConfig(project, GREEN);
  await page.waitForFunction(
    (selector) => document.querySelector(selector)?.textContent?.trim() === "Defect",
    `${card(id)} .label`,
    BEFORE_A_POLL,
  );

  setConfig(project, RED);
  await page.waitForFunction(
    (selector) => document.querySelector(selector)?.textContent?.trim() === "Bug",
    `${card(id)} .label`,
    BEFORE_A_POLL,
  );
  expect(await chips(page, id)).toEqual([{ text: "Bug", background: "rgb(215, 58, 74)" }]);
});

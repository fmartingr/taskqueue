/**
 * Custom column layout: grid width and headings follow GET /api/config.
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { cardIn, useBoard, type Project } from "./harness";

const openBoard = useBoard();

const CUSTOM = [
  "columns:",
  "  - {name: spotted, display_name: Spotted, consider_ready: true}",
  "  - {name: doing, display_name: Doing, default: true}",
  "  - {name: shipped, display_name: Shipped, consider_done: true}",
  "",
].join("\n");

function setBoard(project: Project, columns: string): void {
  writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${columns}`);
}

/** The column headings, left to right, as the page rendered them. */
const headings = (page: Awaited<ReturnType<typeof openBoard>>["page"]) =>
  page.$$eval(".column h2", (nodes) => nodes.map((node) => (node.textContent ?? "").trim()));

test("the board renders the columns the project declares, in order", async () => {
  const { page } = await openBoard((project) => {
    setBoard(project, CUSTOM);
    project.add("Something", "--status", "spotted");
  });

  expect(await headings(page)).toEqual(["Spotted", "Doing", "Shipped"]);

  // The grid follows the count rather than assuming four.
  const tracks = await page.$eval("#board", (board) =>
    getComputedStyle(board as HTMLElement).gridTemplateColumns.split(" ").length,
  );
  expect(tracks).toBe(3);
});

test("the built-in board is five columns, and inbox has replaced backlog", async () => {
  const { page } = await openBoard((project) => {
    project.add("Something");
  });

  expect(await headings(page)).toEqual(["Inbox", "To do", "In Progress", "Done", "Rejected"]);
});

// The default column, not the first one: CUSTOM puts `doing` second and marks it
// default, so a board ordered done-first cannot silently finish a stranded task
// and unblock whatever depended on it (TQ-0088).
test("a task whose column was removed is reconciled into the default one", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    // Filed on the built-in board, which has a done column…
    id = project.add("Filed before the board changed");
    project.mustRun("move", id, "done");
    // …and then the project declares a board that does not.
    setBoard(project, CUSTOM);
  });

  await page.waitForSelector(cardIn("doing", id));
});

test("the status selects offer the project's columns by their display names", async () => {
  const { page } = await openBoard((project) => {
    setBoard(project, CUSTOM);
    project.add("Something", "--status", "spotted");
  });

  await page.click("#new-task");
  const options = await page.$$eval("#create-status option", (nodes) =>
    nodes.map((node) => ({ value: (node as HTMLOptionElement).value, text: (node.textContent ?? "").trim() })),
  );
  expect(options).toEqual([
    { value: "spotted", text: "Spotted" },
    { value: "doing", text: "Doing" },
    { value: "shipped", text: "Shipped" },
  ]);
  // The dialog opens on the column the project marked default.
  expect(await page.inputValue("#create-status")).toBe("doing");
});

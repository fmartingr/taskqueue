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
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "playwright-core";
import { cardIn, idsIn, useBoard, type Board, type Project } from "./harness";

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

test("the menu scrolls the row the keyboard is on into view", async () => {
  const { page } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "label=");
  // The project's own label vocabulary is longer than the menu is tall, which
  // is the only reason this can be asserted at all.
  expect((await options(page)).length).toBeGreaterThan(10);

  // Up from the top wraps to the last row, which is the one furthest out of
  // sight: with the menu left where it was, it would be invisible.
  await page.keyboard.press("ArrowUp");

  const shown = await page.evaluate(() => {
    const list = document.querySelector("#search-suggestions");
    const row = list?.querySelector(".search-option.active");
    if (!(list instanceof HTMLElement) || !(row instanceof HTMLElement)) return null;
    const menu = list.getBoundingClientRect();
    const active = row.getBoundingClientRect();
    return {
      overflows: list.scrollHeight > list.clientHeight,
      scrolled: list.scrollTop,
      inside: active.top >= menu.top - 1 && active.bottom <= menu.bottom + 1,
    };
  });

  expect(shown?.overflows).toBe(true);
  expect(shown?.scrolled).toBeGreaterThan(0);
  expect(shown?.inside).toBe(true);
});

test("every word has to be found, and a quoted one has to be found whole", async () => {
  const { page, urgent } = await openSeeded();

  await page.fill("#search-query", "config changes");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  // Quoted, it is a phrase again, and that phrase is in no title.
  await page.fill("#search-query", '"changes config"');
  await counts(page, "0 of 3 tasks");

  // Words, not a line: the order they were typed in is not part of the search.
  await page.fill("#search-query", "changes config");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  // And one more word narrows again, rather than widening the net.
  await page.fill("#search-query", "changes config oidc");
  await counts(page, "0 of 3 tasks");
});

test("a minus takes tasks away, by word and by term", async () => {
  const { page, urgent, claimed } = await openSeeded();

  await page.fill("#search-query", "-oidc");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  // An exclusion no control can hold: the priority select stays on "any" while
  // the board goes on hiding what the query says to hide.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");
  await page.fill("#search-query", "-priority=high");
  await counts(page, "2 of 3 tasks");
  expect(await page.inputValue("#filter-priority")).toBe("");

  // And it survives a control being moved, which rewrites the line.
  await page.selectOption("#filter-status", "todo");
  await counts(page, "1 of 3 tasks");
  expect(await page.inputValue("#search-query")).toBe("status=todo -priority=high");
});

test("a mis-cased value finds its tasks and moves its control", async () => {
  const { page, urgent } = await openSeeded();

  await page.fill("#search-query", "PRIORITY=Urgent");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  // The select follows the project's own spelling…
  expect(await page.inputValue("#filter-priority")).toBe("urgent");
  // …and the line is left exactly as it was typed.
  expect(await page.inputValue("#search-query")).toBe("PRIORITY=Urgent");
});

test("the query is in the address bar, and a reload comes back to it", async () => {
  const { page, urgent } = await openSeeded();

  const before = await page.evaluate(() => history.length);

  await page.fill("#search-query", "priority=urgent");
  await counts(page, "1 of 3 tasks");
  expect(new URL(page.url()).searchParams.get("q")).toBe("priority=urgent");
  // replaceState, not pushState: typing must not fill the back button.
  expect(await page.evaluate(() => history.length)).toBe(before);

  await page.reload({ waitUntil: "domcontentloaded" });
  await page.waitForSelector(".column");
  expect(await page.inputValue("#search-query")).toBe("priority=urgent");
  expect(await page.inputValue("#filter-priority")).toBe("urgent");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  // Emptying the box takes the parameter away rather than leaving ?q= behind.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");
  expect(new URL(page.url()).searchParams.has("q")).toBe(false);
});

/**
 * A vocabulary none of the board's built-in fallbacks carry, so nothing can be
 * corrected until GET /api/config has actually landed. That is the case the
 * canonicalisation exists for: the query is parsed on the first render, and the
 * columns and labels it names arrive after it.
 */
const OWN_VOCABULARY =
  "columns:\n" +
  "  - {name: spotted, display_name: Spotted, consider_ready: true}\n" +
  "  - {name: doing, display_name: Doing, default: true}\n" +
  "  - {name: shipped, display_name: Shipped, consider_done: true}\n" +
  'labels:\n  glitch:\n    color: "#d73a4a"\n    display_name: Glitch\n';

test("a query the address carried moves controls the project spells its own way", async () => {
  const board = await openBoard((project: Project) => {
    writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${OWN_VOCABULARY}`);
    project.mustRun("add", "Caught in the act", "--status", "spotted", "--label", "glitch");
    project.mustRun("add", "Already moving", "--status", "doing");
  });
  const { page, server } = board;
  await counts(page, "2 tasks");

  await page.goto(`${server.url}/?q=status%3DSPOTTED+label%3DGlitch`, {
    waitUntil: "domcontentloaded",
  });
  await page.waitForSelector(".column");
  await counts(page, "1 of 2 tasks");

  // The controls carry the project's spelling, so the bar cannot read "any"
  // beside a board that is hiding a card…
  expect(await page.inputValue("#filter-status")).toBe("spotted");
  expect(await page.inputValue("#filter-label")).toBe("glitch");
  // …and no duplicate option was invented for the mis-cased value.
  const labelOptions = await page.$$eval("#filter-label option", (nodes) =>
    nodes.map((node) => (node as HTMLOptionElement).value),
  );
  expect(labelOptions).toEqual(["", "glitch"]);
  // The line itself is untouched: correcting it under the cursor is the one
  // thing this must not do.
  expect(await page.inputValue("#search-query")).toBe("status=SPOTTED label=Glitch");
});

test("the clear button empties the search and the controls it was setting", async () => {
  const { page } = await openSeeded();

  expect(await page.$("#search-clear")).toBeNull(); // nothing to clear yet

  await page.fill("#search-query", "priority=urgent");
  await counts(page, "1 of 3 tasks");

  await page.click("#search-clear");
  await counts(page, "3 tasks");
  expect(await page.inputValue("#search-query")).toBe("");
  expect(await page.inputValue("#filter-priority")).toBe("");
  expect(new URL(page.url()).searchParams.has("q")).toBe(false);
  // It is the search's own affordance, so it leaves the cursor where the work
  // is rather than sending focus back to the page.
  expect(await page.evaluate(() => document.activeElement?.id)).toBe("search-query");
});

test("slash puts the cursor in the box without typing itself into it", async () => {
  const { page } = await openSeeded();

  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.keyboard.press("/");

  expect(await page.evaluate(() => document.activeElement?.id)).toBe("search-query");
  expect(await page.inputValue("#search-query")).toBe("");
});

test("slash is a character in a composer, a dialog and the notes editor", async () => {
  let id = "";
  const board = await openBoard((project: Project) => {
    id = project.add("Fix the OIDC redirect");
    project.mustRun("note", id, "--", "the original wording");
  });
  const { page } = board;
  await counts(page, "1 task");

  // The composer: a `/` there is the start of a title, not a shortcut.
  await page.click(".column[data-status='todo'] .composer-open");
  await page.keyboard.press("/");
  expect(await page.evaluate(() => document.activeElement?.className)).toBe("composer-input");
  expect(await page.inputValue(".column[data-status='todo'] .composer-input")).toBe("/");
  await page.keyboard.press("Escape");
  await page.waitForSelector(".column[data-status='todo'] .composer-open");

  // A dialog, with nothing inside it focused: a modal is its own context, and
  // a shortcut that reached through one would put the cursor behind it.
  await page.click(cardIn("todo", id));
  await page.waitForSelector("#task-dialog[open]");
  await page.evaluate(() => (document.activeElement as HTMLElement | null)?.blur());
  await page.keyboard.press("/");
  expect(await page.evaluate(() => document.activeElement?.id)).not.toBe("search-query");
  expect(await page.$("#task-dialog[open]")).not.toBeNull();

  // The notes editor, which is a textarea inside that dialog.
  await page.click("#task-notes .note button.icon");
  await page.waitForSelector("#task-notes .note-editor");
  await page.click("#task-notes .note-editor");
  await page.keyboard.press("End");
  await page.keyboard.press("/");
  expect(await page.inputValue("#task-notes .note-editor")).toBe("the original wording/");
  expect(await page.evaluate(() => document.activeElement?.id)).not.toBe("search-query");
});

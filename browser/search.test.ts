/**
 * The search bar: the one control the board is narrowed from (TQ-0098).
 *
 * Two things are asserted here. That every field the removed filter bar held a
 * control for still narrows the board through a term — `status=`, `priority=`,
 * `label=`, `assignee=` and the bare `ready` — which is what a dropped binding
 * would break and no other layer would see. And the parts of the box only a
 * browser has: the caret, the suggestion menu, the keys it answers, focus
 * leaving it on Tab, and the address bar (TQ-0068).
 *
 * The query language itself is not the subject — `parseQuery`, `completeQuery`
 * and the filter rules are unit-tested in `frontend/search.test.ts` and
 * `frontend/board.test.ts`, against the same functions the component calls.
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "playwright-core";
import { cardIn, idsIn, useBoard, type Board, type Project } from "./harness";

const openBoard = useBoard();

/**
 * How long a count is waited for. Comfortably shorter than the 30s bun gives a
 * test, which is the point: a wait that outlasts the test is torn down by the
 * teardown, and takes the file's Chromium — and every test after it — with it.
 */
const COUNT_TIMEOUT_MS = 10_000;

interface Seeded {
  /** Urgent, assigned, labelled, and the only unblocked task in the queue. */
  urgent: string;
  /** In the todo column too, but waiting on the one below. */
  blocked: string;
  /** In progress, so neither ready nor in the todo column. */
  claimed: string;
}

/**
 * Three tasks every term can tell apart, on a board of their own, already
 * showing all three. One board per test: a test that narrows the board would
 * otherwise be setting up the ones after it.
 */
async function openSeeded(): Promise<Board & Seeded> {
  let seeded: Seeded | undefined;
  const board = await openBoard((project: Project) => {
    const claimed = project.add("Already moving", "--status", "in-progress", "--assignee", "agent-web");
    const urgent = project.add(
      "Push config changes",
      "--priority", "urgent",
      "--assignee", "agent-api",
      "--label", "bug",
    );
    const blocked = project.add(
      "Fix the OIDC redirect",
      "--priority", "high",
      "--assignee", "agent-web",
      "--depends-on", claimed,
    );
    seeded = { urgent, blocked, claimed };
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

/** What the menu is offering, in order. */
function options(page: Page): Promise<string[]> {
  return page.$$eval("#search-suggestions .search-option .search-option-label", (nodes) =>
    nodes.map((node) => node.textContent ?? ""),
  );
}

test("free text narrows the board to what carries it", async () => {
  const { page, blocked } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "oidc");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([blocked]);
});

test("a status term shows one column's work, and the footer says how much", async () => {
  const { page, urgent, blocked, claimed } = await openSeeded();

  await page.fill("#search-query", "status=todo");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent, blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([]);

  // Deleting the term is a filter too — it has to put the hidden column back.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);
});

test("a priority term narrows the board, and deleting it widens it again", async () => {
  const { page, urgent, blocked } = await openSeeded();

  await page.fill("#search-query", "priority=urgent");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  // The query *is* the filter state, so deleting the term has to clear it
  // rather than leave the last one standing.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent, blocked]);
});

test("an assignee term matches a substring of the name", async () => {
  const { page, urgent, blocked, claimed } = await openSeeded();

  await page.fill("#search-query", "assignee=agent-web");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  // A substring rather than a name: there is no vocabulary of assignees, so
  // half of one keeps everybody it is half of.
  await page.fill("#search-query", "assignee=agent");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent, blocked]);
});

test("a label term matches a whole label, not a piece of one", async () => {
  const { page, urgent } = await openSeeded();

  await page.fill("#search-query", "label=bug");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  // The board's own label is "bug", and a label is matched whole: "bu" is not
  // one, however many suggestions it would have offered.
  await page.fill("#search-query", "label=bu");
  await counts(page, "0 of 3 tasks");
});

test("ready leaves the work that is actually offered", async () => {
  const { page, urgent, blocked } = await openSeeded();

  // Both of the others go: one is in a column that offers no work, the other
  // is waiting on it.
  await page.fill("#search-query", "ready");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  expect(await idsIn(page, "in-progress")).toEqual([]);

  // Negated, it is the unchecked box: the board is not narrowed by readiness.
  await page.fill("#search-query", "-ready");
  await counts(page, "3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent, blocked]);
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
  const { page, blocked } = await openSeeded();

  await page.click("#search-query");
  await page.type("#search-query", "priority=");
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  expect(await page.inputValue("#search-query")).toBe("priority=high ");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([blocked]);

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

  // Back to the whole board first: the exclusion below hides the same card as
  // the word above, so without a state in between the count could not tell the
  // second query from the first not having landed yet.
  await page.fill("#search-query", "");
  await counts(page, "3 tasks");

  // An exclusion is the half of the language the filter bar never had: it takes
  // a value away rather than choosing one.
  await page.fill("#search-query", "-priority=high");
  await counts(page, "2 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);

  // And it narrows alongside a positive term rather than replacing it.
  await page.fill("#search-query", "status=todo -priority=high");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
});

test("a mis-cased value finds its tasks, and the line is left as typed", async () => {
  const { page, urgent } = await openSeeded();

  await page.fill("#search-query", "PRIORITY=Urgent");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);
  // Correcting the line under the cursor is the one thing this must not do.
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

test("a query the address carried narrows a board the project spells its own way", async () => {
  let spotted = "";
  const board = await openBoard((project: Project) => {
    writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${OWN_VOCABULARY}`);
    spotted = project.add("Caught in the act", "--status", "spotted", "--label", "glitch");
    project.mustRun("add", "Already moving", "--status", "doing");
  });
  const { page, server } = board;
  await counts(page, "2 tasks");

  await page.goto(`${server.url}/?q=status%3DSPOTTED+label%3DGlitch`, {
    waitUntil: "domcontentloaded",
  });
  await page.waitForSelector(".column");
  await counts(page, "1 of 2 tasks");
  expect(await idsIn(page, "spotted")).toEqual([spotted]);

  // The line itself is untouched: correcting it under the cursor is the one
  // thing this must not do.
  expect(await page.inputValue("#search-query")).toBe("status=SPOTTED label=Glitch");

  // The suggestions are the project's own spelling, and there is one of each:
  // a mis-cased value in the query invents no second option for itself.
  await page.click("#search-query");
  await page.fill("#search-query", "label=");
  expect(await options(page)).toEqual(["glitch"]);
});

test("the clear button empties every term at once", async () => {
  const { page, urgent, blocked, claimed } = await openSeeded();

  expect(await page.$("#search-clear")).toBeNull(); // nothing to clear yet

  // Every field the filter bar had a control for, set at once.
  await page.fill("#search-query", "status=todo priority=urgent assignee=agent-api label=bug ready");
  await counts(page, "1 of 3 tasks");
  expect(await idsIn(page, "todo")).toEqual([urgent]);

  await page.click("#search-clear");
  await counts(page, "3 tasks");
  expect(await page.inputValue("#search-query")).toBe("");
  expect(await idsIn(page, "todo")).toEqual([urgent, blocked]);
  expect(await idsIn(page, "in-progress")).toEqual([claimed]);
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

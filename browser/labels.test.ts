/**
 * Labels on the board: the chips a card draws, and the search terms that choose
 * between them.
 *
 * The colours and display names come from the project's `.taskqueue.yaml`, and
 * only a browser can say what the page actually painted — which is why the
 * assertions here read computed styles rather than the data behind them.
 *
 * The vocabulary reaches one more place: the search bar's suggestions for
 * `label=`, which are the only thing left that says what labels exist (TQ-0098).
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import type { Page } from "playwright-core";
import { card, idsIn, useBoard, type Project } from "./harness";

const openBoard = useBoard();

/** The chips on a card, as the page renders them — halves and all. */
async function chips(page: Awaited<ReturnType<typeof openBoard>>["page"], id: string) {
  return page.$$eval(`${card(id)} .label`, (nodes) =>
    nodes.map((node) => {
      const element = node as HTMLElement;
      const half = (selector: string) => {
        const part = element.querySelector<HTMLElement>(selector);
        if (part === null) return null;
        const style = getComputedStyle(part);
        return { text: part.textContent ?? "", background: style.backgroundColor, color: style.color };
      };
      const style = getComputedStyle(element);
      return {
        text: element.textContent ?? "",
        title: element.title,
        background: style.backgroundColor,
        color: style.color,
        // Every side carries the same colour; one of them is the assertion.
        borderColor: style.borderTopColor,
        tinted: element.classList.contains("tinted"),
        scoped: element.classList.contains("scoped"),
        scope: half(".label-scope"),
        value: half(".label-value"),
      };
    }),
  );
}

/** Replaces the project's vocabulary, before its server starts. */
function setLabels(project: Project, labels: string): void {
  writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${labels}`);
}

/**
 * What the search bar offers for a key, as the menu draws it.
 *
 * A row is one line of text, so a scoped label has to spell out both halves the
 * chip draws — otherwise the menu would read "Backend" beside a card reading
 * "Component | Backend".
 */
async function suggestions(page: Page, typed: string) {
  await page.click("#search-query");
  await page.fill("#search-query", typed);
  return page.$$eval("#search-suggestions .search-option", (nodes) =>
    nodes.map((node) => ({
      value: node.querySelector(".search-option-label")?.textContent ?? "",
      detail: node.querySelector(".search-option-detail")?.textContent ?? "",
    })),
  );
}

/** A project declaring one flat label and one scoped one. */
const TWO_LABELS =
  'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n' +
  '  component/frontend:\n    color: "#c5def5"\n    display_name: Frontend\n';

test("a label with no scope is drawn as one chip, in its colour", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    setLabels(project, TWO_LABELS);
    id = project.add("Coloured", "--label", "bug");
  });

  const [bug] = await chips(page, id);

  // The display name is what the board shows; the raw label, which is what the
  // CLI takes, is the tooltip.
  expect(bug).toMatchObject({
    text: "Bug",
    title: "bug",
    background: "rgb(215, 58, 74)",
    tinted: true,
    scoped: false,
    scope: null,
  });

  // The chip carries its own background, so its text is picked to contrast with
  // that rather than with the page: a dark chip gets light text and a light one
  // dark text, in either theme.
  expect(bug!.color).toBe("rgb(255, 255, 255)");
});

test("a scoped label is drawn as two halves, tied together by the border", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    setLabels(project, TWO_LABELS);
    id = project.add("Coloured", "--label", "component/frontend");
  });

  const [frontend] = await chips(page, id);

  // The scope comes from the key and the value from the display name, so the
  // half saying what kind of label this is survives onto the board.
  expect(frontend).toMatchObject({ title: "component/frontend", tinted: true, scoped: true });
  expect(frontend!.scope).toEqual({
    text: "Component",
    background: "rgb(197, 222, 245)",
    color: "rgb(17, 20, 24)",
  });
  expect(frontend!.value).toMatchObject({ text: "Frontend", color: "rgb(197, 222, 245)" });

  // One object rather than two: the pill's border is the label's colour, and
  // the value half sits on the page's own surface.
  expect(frontend!.borderColor).toBe("rgb(197, 222, 245)");
  expect(frontend!.value!.background).toBe("rgb(255, 255, 255)");

  // That surface is a theme token, not white: the same chip in the dark palette
  // draws its value half on the dark surface, with the same colour as its text.
  await page.emulateMedia({ colorScheme: "dark" });
  const [dark] = await chips(page, id);
  expect(dark!.value).toEqual({ text: "Frontend", background: "rgb(28, 32, 39)", color: "rgb(197, 222, 245)" });
  expect(dark!.scope).toEqual({ text: "Component", background: "rgb(197, 222, 245)", color: "rgb(17, 20, 24)" });
});

test("a scoped label the project does not declare still shows its scope", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    setLabels(project, TWO_LABELS);
    id = project.add("Improvised", "--label", "component/whatever");
  });

  const [chip] = await chips(page, id);

  // Both halves come from the key, and neither has a colour to draw with, so
  // the pill stays neutral.
  expect(chip).toMatchObject({ title: "component/whatever", tinted: false, scoped: true });
  expect(chip!.scope?.text).toBe("Component");
  expect(chip!.value?.text).toBe("Whatever");
  expect(chip!.borderColor).toBe("rgb(217, 220, 227)");
});

test("a label the project does not declare is accepted and drawn neutral", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    setLabels(project, 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n');
    id = project.add("Improvised", "--label", "not-in-the-set");
  });

  const [chip] = await chips(page, id);
  expect(chip).toMatchObject({ text: "not-in-the-set", title: "not-in-the-set", tinted: false });
  // Neutral: the pill's own border, no colour of its own.
  expect(chip!.background).toBe("rgba(0, 0, 0, 0)");
});

test("the search offers every label, spelled out, and filters on a whole one", async () => {
  let backend = "";
  let loose = "";
  const { page } = await openBoard((project) => {
    setLabels(
      project,
      'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n' +
        '  component/backend:\n    color: "#1d76db"\n    display_name: Backend\n' +
        '  component/cli:\n    color: "#0052cc"\n    display_name: CLI\n',
    );
    backend = project.add("Server side", "--label", "component/backend");
    loose = project.add("Everything else", "--label", "bug");
  });

  // The whole vocabulary, in one sorted list: the value is what the query takes
  // and the display name is what the chip would read.
  expect(await suggestions(page, "label=")).toEqual([
    { value: "bug", detail: "Bug" },
    { value: "component/backend", detail: "Component | Backend" },
    { value: "component/cli", detail: "Component | CLI" },
  ]);

  await page.fill("#search-query", "label=component/backend");
  await page.waitForSelector(card(loose), { state: "detached" });
  expect(await idsIn(page, "todo")).toEqual([backend]);

  // A label is matched whole, so the half that reads like one selects nothing.
  await page.fill("#search-query", "label=backend");
  await page.waitForSelector(card(backend), { state: "detached" });

  await page.fill("#search-query", "");
  await page.waitForSelector(card(loose));
});

test("a label only a task carries is still offered, and still filters", async () => {
  let improvised = "";
  const { page } = await openBoard((project) => {
    setLabels(project, 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n');
    improvised = project.add("Improvised", "--label", "surprise");
    project.add("Ordinary", "--label", "bug");
  });

  // A label the project does not declare has no display name to show beside it,
  // and is offered all the same: it still has to be filterable.
  expect(await suggestions(page, "label=")).toEqual([
    { value: "bug", detail: "Bug" },
    { value: "surprise", detail: "" },
  ]);

  await page.fill("#search-query", "label=surprise");
  await page.waitForSelector(card(improvised));
  expect(await idsIn(page, "todo")).toEqual([improvised]);
});

test("a label an agent has just filed turns up in the suggestions", async () => {
  const { project, page } = await openBoard((p) => {
    setLabels(p, 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n');
    p.add("Ordinary", "--label", "bug");
  });

  const offered = () =>
    page.$$eval("#search-suggestions .search-option-label", (nodes) =>
      nodes.map((node) => node.textContent ?? ""),
    );

  await page.click("#search-query");
  await page.fill("#search-query", "label=");
  expect(await offered()).toEqual(["bug"]);

  // The menu is a live list, not a snapshot taken when it opened: an agent
  // files a task carrying a label nothing has used yet, and the board hears
  // about it with the box still focused.
  project.add("From an agent", "--label", "surprise");
  await page.waitForFunction(
    () => document.querySelectorAll("#search-suggestions .search-option").length === 2,
    undefined,
    { timeout: 10_000 },
  );
  expect(await offered()).toEqual(["bug", "surprise"]);
});

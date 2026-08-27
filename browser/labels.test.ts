/**
 * Labels on the board: the chips a card draws, and the grouped filter that
 * chooses between them.
 *
 * The colours and display names come from the project's `.taskqueue.yaml`, and
 * only a browser can say what the page actually painted — which is why the
 * assertions here read computed styles rather than the data behind them.
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
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

test("the filter groups labels by prefix and filters on a whole label", async () => {
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

  // The grouped options: the flat labels first, then one optgroup per prefix.
  const groups = await page.$$eval("#filter-label optgroup", (nodes) =>
    nodes.map((node) => ({
      label: (node as HTMLOptGroupElement).label,
      options: [...node.querySelectorAll("option")].map((option) => option.value),
      // An option is one line of text, so it spells out what a chip draws as
      // two halves — otherwise the bar would read "Backend" beside a chip
      // reading "Component | Backend".
      texts: [...node.querySelectorAll("option")].map((option) => option.textContent),
    })),
  );
  expect(groups).toEqual([
    {
      label: "component",
      options: ["component/backend", "component/cli"],
      texts: ["Component | Backend", "Component | CLI"],
    },
  ]);

  const flat = await page.$$eval("#filter-label > option", (nodes) =>
    nodes.map((node) => (node as HTMLOptionElement).value),
  );
  expect(flat).toEqual(["", "bug"]);

  await page.selectOption("#filter-label", "component/backend");
  await page.waitForSelector(card(loose), { state: "detached" });
  expect(await idsIn(page, "todo")).toEqual([backend]);

  await page.selectOption("#filter-label", "");
  await page.waitForSelector(card(loose));
});

test("a label only a task carries is still filterable, marked as unconfigured", async () => {
  let improvised = "";
  const { page } = await openBoard((project) => {
    setLabels(project, 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n');
    improvised = project.add("Improvised", "--label", "surprise");
    project.add("Ordinary", "--label", "bug");
  });

  const options = await page.$$eval("#filter-label option", (nodes) =>
    nodes.map((node) => {
      const option = node as HTMLOptionElement;
      return { value: option.value, title: option.title };
    }),
  );
  expect(options.map((option) => option.value)).toEqual(["", "bug", "surprise"]);
  expect(options.find((option) => option.value === "surprise")?.title).toContain("not in the project's label set");

  await page.selectOption("#filter-label", "surprise");
  expect(await idsIn(page, "todo")).toEqual([improvised]);
});

test("the poll leaves the label filter alone while it has focus", async () => {
  const { project, page } = await openBoard((p) => {
    setLabels(p, 'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n');
    p.add("Ordinary", "--label", "bug");
  });

  const values = () =>
    page.$$eval("#filter-label option", (nodes) => nodes.map((node) => (node as HTMLOptionElement).value));
  expect(await values()).toEqual(["", "bug"]);

  // Focus stands in for an expanded dropdown, which is what a rebuild would
  // collapse — and which Playwright cannot hold open across an assertion.
  await page.focus("#filter-label");

  // An agent files a task with a label nothing has used yet: the poll picks the
  // task up, but the option list must not be replaced under the open control.
  project.add("From an agent", "--label", "surprise");
  await page.waitForSelector(".card", { state: "attached" });
  await page.waitForFunction(
    () => document.querySelectorAll(".card").length === 2,
    undefined,
    { timeout: 10_000 },
  );
  expect(await values()).toEqual(["", "bug"]);

  // Blurring picks the skipped rebuild back up.
  await page.locator("#filter-label").blur();
  await page.waitForFunction(() => document.querySelectorAll("#filter-label option").length === 3, undefined, {
    timeout: 10_000,
  });
  expect(await values()).toEqual(["", "bug", "surprise"]);
});

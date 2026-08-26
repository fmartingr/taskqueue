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

/** The chips on a card, as the page renders them. */
async function chips(page: Awaited<ReturnType<typeof openBoard>>["page"], id: string) {
  return page.$$eval(`${card(id)} .label`, (nodes) =>
    nodes.map((node) => {
      const element = node as HTMLElement;
      return {
        text: element.textContent ?? "",
        title: element.title,
        background: getComputedStyle(element).backgroundColor,
        color: getComputedStyle(element).color,
        tinted: element.classList.contains("tinted"),
      };
    }),
  );
}

/** Replaces the project's vocabulary, before its server starts. */
function setLabels(project: Project, labels: string): void {
  writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${labels}`);
}

test("a configured label is drawn with its colour and display name", async () => {
  let id = "";
  const { page } = await openBoard((project) => {
    setLabels(
      project,
      'labels:\n  bug:\n    color: "#d73a4a"\n    display_name: Bug\n' +
        '  component/frontend:\n    color: "#c5def5"\n    display_name: Frontend\n',
    );
    id = project.add("Coloured", "--label", "bug", "--label", "component/frontend");
  });

  const [bug, frontend] = await chips(page, id);

  // The display name is what the board shows; the raw label, which is what the
  // CLI takes, is the tooltip.
  expect(bug).toMatchObject({ text: "Bug", title: "bug", background: "rgb(215, 58, 74)", tinted: true });
  expect(frontend).toMatchObject({ text: "Frontend", title: "component/frontend", tinted: true });

  // Each chip carries its own background, so its text is picked to contrast
  // with that rather than with the page: a dark chip gets light text and a
  // light one dark text, in either theme.
  expect(bug!.color).toBe("rgb(255, 255, 255)");
  expect(frontend!.color).toBe("rgb(17, 20, 24)");
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
    })),
  );
  expect(groups).toEqual([{ label: "component", options: ["component/backend", "component/cli"] }]);

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

/**
 * Priorities on the board: the badge a card draws, and the three selects that
 * choose between them.
 *
 * These are the closed set labels are not, so the interesting cases are the
 * ones only a browser shows: options built from `.taskqueue.yaml` rather than
 * written into index.html, and a task whose priority the project has since
 * dropped — which must stay selectable, because the dialog writes every field
 * back and a missing option is a value the next save would erase.
 */

import { expect, test } from "bun:test";
import { writeFileSync } from "node:fs";
import { join } from "node:path";
import { card, idsIn, useBoard, type Project } from "./harness";

const openBoard = useBoard();

const CUSTOM = [
  "priorities:",
  '  - {name: p0, color: "#b60205", display_name: Critical}',
  '  - {name: p1, color: "#fbca04"}',
  '  - {name: p2, color: "#4b5563", display_name: Ordinary, default: true}',
  "",
].join("\n");

/** Replaces the project's vocabulary, before its server starts. */
function setPriorities(project: Project, priorities: string): void {
  writeFileSync(join(project.dir, ".taskqueue.yaml"), `version: 1\npath: .tasks\n${priorities}`);
}

/** The options a select offers, as the page built them. */
async function options(page: Awaited<ReturnType<typeof openBoard>>["page"], selector: string) {
  return page.$$eval(`${selector} option`, (nodes) =>
    nodes.map((node) => {
      const option = node as HTMLOptionElement;
      return { value: option.value, text: option.textContent ?? "", title: option.title };
    }),
  );
}

test("the selects are built from the project's vocabulary, not the markup", async () => {
  const { page } = await openBoard((project) => {
    setPriorities(project, CUSTOM);
    project.add("Anything");
  });

  // The filter bar keeps its "any", then offers the configured set in order.
  expect(await options(page, "#filter-priority")).toEqual([
    { value: "", text: "any", title: "" },
    { value: "p0", text: "Critical", title: "p0" },
    { value: "p1", text: "p1", title: "p1" },
    { value: "p2", text: "Ordinary", title: "p2" },
  ]);

  // The create dialog offers the same set, preselected at the configured
  // default rather than at whatever came first.
  await page.click("#new-task");
  expect((await options(page, "#create-priority")).map((option) => option.value)).toEqual(["p0", "p1", "p2"]);
  expect(await page.inputValue("#create-priority")).toBe("p2");
});

test("a card's badge carries the configured colour and display name", async () => {
  let critical = "";
  let light = "";
  const { page } = await openBoard((project) => {
    setPriorities(project, CUSTOM);
    critical = project.add("Drop everything", "--priority", "p0");
    light = project.add("Sunny", "--priority", "p1");
  });

  const badge = async (id: string) =>
    page.$eval(`${card(id)} .badge`, (node) => {
      const element = node as HTMLElement;
      return {
        text: element.textContent ?? "",
        title: element.title,
        background: getComputedStyle(element).backgroundColor,
        color: getComputedStyle(element).color,
        tinted: element.classList.contains("tinted"),
      };
    });

  // The display name is what the board shows; the stored value, which is what
  // the CLI and the filters take, is the tooltip.
  expect(await badge(critical)).toMatchObject({
    text: "Critical",
    title: "p0",
    background: "rgb(182, 2, 5)",
    tinted: true,
  });

  // The badge carries its own background, so its text contrasts with that
  // rather than with the page: dark on a light colour, light on a dark one.
  expect((await badge(critical)).color).toBe("rgb(255, 255, 255)");
  expect((await badge(light)).color).toBe("rgb(17, 20, 24)");
});

test("filtering by a configured priority hides the rest", async () => {
  let critical = "";
  let ordinary = "";
  const { page } = await openBoard((project) => {
    setPriorities(project, CUSTOM);
    critical = project.add("Drop everything", "--priority", "p0");
    ordinary = project.add("Later", "--priority", "p2");
  });

  await page.selectOption("#filter-priority", "p0");
  await page.waitForSelector(card(ordinary), { state: "detached" });
  expect(await idsIn(page, "todo")).toEqual([critical]);

  await page.selectOption("#filter-priority", "");
  await page.waitForSelector(card(ordinary));
});

// The vocabulary can be edited under tasks already filed. Opening one of those
// must offer the value it carries, or the save that follows would rewrite the
// file with whatever the select fell back to.
test("a priority the project has dropped stays selectable, and survives a save", async () => {
  let stale = "";
  const { page, project, server } = await openBoard((project) => {
    // Filed under the built-in set, then the project declares its own.
    stale = project.add("Filed earlier", "--priority", "high");
    setPriorities(project, CUSTOM);
  });

  // The card still shows the value it carries, drawn neutral.
  const badge = await page.$eval(`${card(stale)} .badge`, (node) => ({
    text: (node as HTMLElement).textContent ?? "",
    tinted: (node as HTMLElement).classList.contains("tinted"),
  }));
  expect(badge).toEqual({ text: "high", tinted: false });

  await page.click(card(stale));
  await page.waitForSelector("#task-dialog[open]");

  const offered = await options(page, "#task-priority");
  expect(offered.map((option) => option.value)).toEqual(["p0", "p1", "p2", "high"]);
  expect(offered[3]!.title).toContain("not in the project's priority set");
  expect(await page.inputValue("#task-priority")).toBe("high");

  // Saving without touching the priority writes it back unchanged.
  await page.fill("#task-title", "Retitled");
  await page.click("#task-form button[type='submit']");
  await page.waitForSelector("#task-dialog[open]", { state: "detached" });

  const [saved] = (await project.tasks(server)).filter((task) => task.id === stale);
  expect(saved).toMatchObject({ title: "Retitled", priority: "high" });
});

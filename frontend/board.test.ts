import { describe, expect, test } from "bun:test";
import {
  CHIP_DARK_TEXT,
  CHIP_LIGHT_TEXT,
  groupLabels,
  indexTasks,
  isConfigured,
  isReady,
  labelChip,
  labelDisplay,
  labelsInUse,
  pendingDependencies,
  visibleTasks,
  defaultPriority,
  findPriority,
  priorityChip,
  priorityDisplay,
  priorityOptions,
  FALLBACK_COLUMNS,
  type Filters,
  type LabelSet,
  type PrioritySet,
  type Status,
  type Task,
} from "./board";

const STAMP = "2026-08-25T09:42:00+02:00";

/** Mirrors the `task` helper in internal/task/task_test.go. */
function task(id: string, status: Status, ...deps: string[]): Task {
  return {
    id,
    title: `Task ${id}`,
    status,
    priority: "normal",
    depends_on: deps,
    created: STAMP,
    updated: STAMP,
    body: "",
  };
}

const NO_FILTERS: Filters = { status: "", priority: "", assignee: "", label: "", ready: false };

const filters = (overrides: Partial<Filters>): Filters => ({ ...NO_FILTERS, ...overrides });

describe("indexTasks", () => {
  test("keys tasks by ID", () => {
    const tasks = [task("TQ-0001", "todo"), task("TQ-0002", "done")];
    const index = indexTasks(tasks);
    expect(index.size).toBe(2);
    expect(index.get("TQ-0001")).toBe(tasks[0]!);
    expect(index.get("TQ-0002")).toBe(tasks[1]!);
    expect(index.get("TQ-9999")).toBeUndefined();
  });

  test("an empty list indexes to an empty map", () => {
    expect(indexTasks([]).size).toBe(0);
  });

  test("the last task with an ID wins, as IndexTasks does", () => {
    const first = task("TQ-0001", "todo");
    const second = task("TQ-0001", "done");
    expect(indexTasks([first, second]).get("TQ-0001")).toBe(second);
  });
});

describe("pendingDependencies", () => {
  const tasks = [
    task("TQ-0001", "done"),
    task("TQ-0002", "todo"),
    task("TQ-0003", "in-progress"),
    task("TQ-0004", "inbox"),
  ];
  const index = indexTasks(tasks);

  test("a task without dependencies has none pending", () => {
    expect(pendingDependencies(task("TQ-0100", "todo"), index, FALLBACK_COLUMNS)).toEqual([]);
  });

  test("depends_on may be absent altogether", () => {
    const { depends_on, ...without } = task("TQ-0100", "todo");
    expect(pendingDependencies(without, index, FALLBACK_COLUMNS)).toEqual([]);
  });

  test("a done dependency is not pending", () => {
    expect(pendingDependencies(task("TQ-0100", "todo", "TQ-0001"), index, FALLBACK_COLUMNS)).toEqual([]);
  });

  test("every unfinished status is pending", () => {
    const pending = pendingDependencies(task("TQ-0100", "todo", "TQ-0002", "TQ-0003", "TQ-0004"), index, FALLBACK_COLUMNS);
    expect(pending).toEqual(["TQ-0002", "TQ-0003", "TQ-0004"]);
  });

  test("a missing dependency is pending rather than ignored", () => {
    expect(pendingDependencies(task("TQ-0100", "todo", "TQ-9999"), index, FALLBACK_COLUMNS)).toEqual(["TQ-9999"]);
  });

  test("the declared order is kept and done dependencies drop out", () => {
    const pending = pendingDependencies(task("TQ-0100", "todo", "TQ-0002", "TQ-0001", "TQ-9999"), index, FALLBACK_COLUMNS);
    expect(pending).toEqual(["TQ-0002", "TQ-9999"]);
  });
});

// The same fixture and the same cases as TestReady in internal/task/task_test.go:
// isReady is a second implementation of that rule and the two have to agree.
describe("isReady", () => {
  const tasks = [
    task("TQ-0001", "done"),
    task("TQ-0002", "todo"),
    task("TQ-0003", "todo", "TQ-0001"),
    task("TQ-0004", "todo", "TQ-0002"),
    task("TQ-0005", "todo", "TQ-9999"),
    task("TQ-0006", "in-progress"),
    task("TQ-0007", "done"),
    task("TQ-0008", "inbox", "TQ-0001", "TQ-0007"),
  ];
  const index = indexTasks(tasks);

  const cases: [string, boolean, string][] = [
    ["TQ-0002", true, "no dependencies"],
    ["TQ-0003", true, "dependency is done"],
    ["TQ-0008", false, "dependencies all done, but intake is not offered until it is triaged"],
    ["TQ-0004", false, "dependency is not done"],
    ["TQ-0005", false, "dependency is missing"],
    ["TQ-0006", false, "already in progress"],
    ["TQ-0001", false, "already done"],
  ];

  for (const [id, want, why] of cases) {
    test(`${id} is ${want ? "ready" : "not ready"}: ${why}`, () => {
      expect(isReady(index.get(id)!, index, FALLBACK_COLUMNS)).toBe(want);
    });
  }

  test("an in-progress task is not ready even with every dependency done", () => {
    expect(isReady(task("TQ-0100", "in-progress", "TQ-0001"), index, FALLBACK_COLUMNS)).toBe(false);
  });

  test("a blocked task that is already done is still not ready", () => {
    expect(isReady(task("TQ-0100", "done", "TQ-9999"), index, FALLBACK_COLUMNS)).toBe(false);
  });
});

describe("visibleTasks", () => {
  const tasks = [
    { ...task("TQ-0001", "done"), labels: ["backend"], assignee: "agent-api" },
    { ...task("TQ-0002", "todo"), labels: ["backend", "auth"], assignee: "agent-ui", priority: "high" },
    { ...task("TQ-0003", "in-progress"), labels: [], assignee: "" },
    { ...task("TQ-0004", "todo", "TQ-0003"), labels: ["Frontend"], assignee: "Agent-UI" },
  ];

  test("no filters keeps every task, in order", () => {
    expect(visibleTasks(tasks, NO_FILTERS, FALLBACK_COLUMNS).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
      "TQ-0003",
      "TQ-0004",
    ]);
  });

  test("status matches exactly", () => {
    expect(visibleTasks(tasks, filters({ status: "todo" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0002", "TQ-0004"]);
  });

  test("priority matches exactly, and an unset priority reads as normal", () => {
    expect(visibleTasks(tasks, filters({ priority: "high" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0002"]);
    const unset = [{ ...task("TQ-0100", "todo"), priority: undefined }];
    expect(visibleTasks(unset, filters({ priority: "normal" }), FALLBACK_COLUMNS)).toEqual([]);
  });

  test("assignee matches a substring, case-insensitively", () => {
    expect(visibleTasks(tasks, filters({ assignee: "agent" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
      "TQ-0004",
    ]);
    expect(visibleTasks(tasks, filters({ assignee: "UI" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0002", "TQ-0004"]);
  });

  test("surrounding whitespace in a search box is ignored", () => {
    expect(visibleTasks(tasks, filters({ assignee: "  agent-api  " }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0001"]);
  });

  // The label filter is a list of the labels that exist, not a search box, so
  // it matches a whole label: "backend" must not also select "component/backend".
  test("label matches a whole label", () => {
    expect(visibleTasks(tasks, filters({ label: "backend" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
    ]);
    expect(visibleTasks(tasks, filters({ label: "end" }), FALLBACK_COLUMNS)).toEqual([]);
    expect(visibleTasks(tasks, filters({ label: "auth" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0002"]);
  });

  test("a task without an assignee or labels is dropped by those filters", () => {
    expect(visibleTasks(tasks, filters({ assignee: "a" }), FALLBACK_COLUMNS).map((t) => t.id)).not.toContain("TQ-0003");
    expect(visibleTasks(tasks, filters({ label: "backend" }), FALLBACK_COLUMNS).map((t) => t.id)).not.toContain("TQ-0003");
  });

  test("the ready filter keeps only unblocked, unclaimed tasks", () => {
    expect(visibleTasks(tasks, filters({ ready: true }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual(["TQ-0002"]);
  });

  test("filters combine", () => {
    expect(visibleTasks(tasks, filters({ status: "todo", label: "backend" }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual([
      "TQ-0002",
    ]);
  });

  test("readiness is judged against every task, not the visible ones", () => {
    // TQ-0004 depends on TQ-0003, which the status filter hides. Hiding a
    // dependency must not turn it into a missing one.
    const done = tasks.map((t) => (t.id === "TQ-0003" ? { ...t, status: "done" as Status } : t));
    expect(visibleTasks(done, filters({ status: "todo", ready: true }), FALLBACK_COLUMNS).map((t) => t.id)).toEqual([
      "TQ-0002",
      "TQ-0004",
    ]);
  });

  test("an empty list stays empty", () => {
    expect(visibleTasks([], filters({ ready: true }), FALLBACK_COLUMNS)).toEqual([]);
  });
});

// ── Labels ──────────────────────────────────────────────────────

const LABELS: LabelSet = {
  bug: { color: "#d73a4a", display_name: "Bug" },
  "component/backend": { color: "#1d76db", display_name: "Backend" },
  "component/frontend": { color: "#c5def5", display_name: "Frontend" },
  broken: { color: "not-a-colour", display_name: "Broken" },
};

describe("labelDisplay", () => {
  test("a configured label shows its display name", () => {
    expect(labelDisplay("component/backend", LABELS)).toBe("Backend");
  });

  test("an unconfigured label shows itself", () => {
    expect(labelDisplay("whatever", LABELS)).toBe("whatever");
  });

  test("a configured label with no display name shows itself", () => {
    expect(labelDisplay("x", { x: { color: "#ffffff", display_name: "" } })).toBe("x");
  });
});

describe("labelChip", () => {
  test("a light colour gets dark text, a dark one light text", () => {
    // The chip carries its own background, which is what makes one set of
    // colours readable against both themes.
    expect(labelChip("component/frontend", LABELS)?.text).toBe(CHIP_DARK_TEXT);
    expect(labelChip("component/backend", LABELS)?.text).toBe(CHIP_LIGHT_TEXT);
  });

  test("the background is the configured colour", () => {
    expect(labelChip("bug", LABELS)?.background).toBe("#d73a4a");
  });

  test("three-digit hex works", () => {
    expect(labelChip("x", { x: { color: "#FFF", display_name: "X" } })).toEqual({
      background: "#FFF",
      text: CHIP_DARK_TEXT,
    });
  });

  test("an unconfigured label has no chip, so it renders neutral", () => {
    expect(labelChip("whatever", LABELS)).toBeNull();
  });

  test("a colour the board cannot draw has no chip either", () => {
    expect(labelChip("broken", LABELS)).toBeNull();
    expect(labelChip("empty", { empty: { color: "", display_name: "Empty" } })).toBeNull();
  });
});

describe("groupLabels", () => {
  test("groups by the prefix before the first slash, ungrouped first", () => {
    const groups = groupLabels(LABELS, []);
    expect(groups.map((group) => group.prefix)).toEqual(["", "component"]);
    expect(groups[0]!.labels.map((label) => label.name)).toEqual(["broken", "bug"]);
    expect(groups[1]!.labels.map((label) => label.display)).toEqual(["Backend", "Frontend"]);
  });

  test("labels in use join their group even when unconfigured", () => {
    const groups = groupLabels(LABELS, ["component/docs", "loose"]);
    const component = groups.find((group) => group.prefix === "component")!;
    expect(component.labels.map((label) => label.name)).toEqual([
      "component/backend",
      "component/docs",
      "component/frontend",
    ]);
    expect(component.labels.find((label) => label.name === "component/docs")).toEqual({
      name: "component/docs",
      display: "component/docs",
      configured: false,
    });
    expect(groups[0]!.labels.map((label) => label.name)).toEqual(["broken", "bug", "loose"]);
  });

  test("a label in use twice appears once", () => {
    const groups = groupLabels({}, ["loose", "loose"]);
    expect(groups).toEqual([{ prefix: "", labels: [{ name: "loose", display: "loose", configured: false }] }]);
  });

  test("only the first slash groups: the rest is part of the label", () => {
    const groups = groupLabels({}, ["a/b/c"]);
    expect(groups).toEqual([
      { prefix: "a", labels: [{ name: "a/b/c", display: "a/b/c", configured: false }] },
    ]);
  });

  test("nothing configured and nothing in use is no groups at all", () => {
    expect(groupLabels({}, [])).toEqual([]);
  });

  test("groups come out in a stable order", () => {
    const groups = groupLabels({ "z/one": { color: "#fff", display_name: "" } }, ["a/two", "m"]);
    expect(groups.map((group) => group.prefix)).toEqual(["", "a", "z"]);
  });
});

describe("labelsInUse", () => {
  test("collects every label on every task, once", () => {
    const tasks = [
      { ...task("TQ-0001", "todo"), labels: ["bug", "component/cli"] },
      { ...task("TQ-0002", "todo"), labels: ["bug"] },
      task("TQ-0003", "todo"),
    ];
    expect(labelsInUse(tasks)).toEqual(["bug", "component/cli"]);
  });
});

// A task may carry any string as a label, including the names on
// Object.prototype. A plain `name in labels` answers yes for all of them.
describe("labels that collide with Object.prototype", () => {
  const inherited = ["constructor", "toString", "hasOwnProperty", "valueOf"];

  test("are not reported as configured", () => {
    for (const name of inherited) {
      expect(isConfigured(name, LABELS)).toBe(false);
      expect(groupLabels({}, [name])[0]!.labels[0]).toEqual({ name, display: name, configured: false });
    }
  });

  test("render as themselves, with no chip", () => {
    for (const name of inherited) {
      expect(labelDisplay(name, LABELS)).toBe(name);
      expect(labelChip(name, LABELS)).toBeNull();
    }
  });

  test("are still configurable like any other label", () => {
    const labels: LabelSet = { toString: { color: "#0e8a16", display_name: "Stringly" } };
    expect(isConfigured("toString", labels)).toBe(true);
    expect(labelDisplay("toString", labels)).toBe("Stringly");
    expect(labelChip("toString", labels)?.background).toBe("#0e8a16");
  });
});

// ── Priorities ──────────────────────────────────────────────────

const PRIORITIES: PrioritySet = [
  { name: "p0", color: "#b60205", display_name: "Critical" },
  { name: "p1", color: "#c2410c", display_name: "" },
  { name: "p2", color: "#4b5563", display_name: "Ordinary", default: true },
  { name: "p3", color: "not-a-colour", display_name: "Broken" },
];

describe("defaultPriority", () => {
  test("the entry marked default", () => {
    expect(defaultPriority(PRIORITIES)).toBe("p2");
  });

  test("the most severe when none is marked, so a card always has one to show", () => {
    expect(defaultPriority([{ name: "a", color: "#111111", display_name: "A" }])).toBe("a");
  });

  test("an empty vocabulary has no default rather than an invented one", () => {
    expect(defaultPriority([])).toBe("");
  });
});

describe("findPriority", () => {
  test("a configured value", () => {
    expect(findPriority("p0", PRIORITIES)?.color).toBe("#b60205");
  });

  test("one the project has dropped", () => {
    expect(findPriority("urgent", PRIORITIES)).toBeUndefined();
  });
});

describe("priorityDisplay", () => {
  test("a configured value shows its display name", () => {
    expect(priorityDisplay("p0", PRIORITIES)).toBe("Critical");
  });

  test("an empty display name falls back to the value, which is what the CLI takes", () => {
    expect(priorityDisplay("p1", PRIORITIES)).toBe("p1");
  });

  test("a value the project dropped shows itself", () => {
    expect(priorityDisplay("urgent", PRIORITIES)).toBe("urgent");
  });
});

describe("priorityChip", () => {
  test("a configured colour, with text picked to contrast with it", () => {
    expect(priorityChip("p0", PRIORITIES)).toEqual({ background: "#b60205", text: CHIP_LIGHT_TEXT });
  });

  test("a light colour takes dark text", () => {
    expect(priorityChip("p1", [{ name: "p1", color: "#fbca04", display_name: "P1" }])).toEqual({
      background: "#fbca04",
      text: CHIP_DARK_TEXT,
    });
  });

  test("a colour the board cannot parse draws nothing rather than guessing", () => {
    expect(priorityChip("p3", PRIORITIES)).toBeNull();
  });

  test("a value the project dropped draws nothing", () => {
    expect(priorityChip("urgent", PRIORITIES)).toBeNull();
  });
});

describe("priorityOptions", () => {
  test("the vocabulary in rank order, since the config is the ranking", () => {
    expect(priorityOptions(PRIORITIES, []).map((option) => option.name)).toEqual(["p0", "p1", "p2", "p3"]);
  });

  test("display names come along, falling back to the value", () => {
    const options = priorityOptions(PRIORITIES, []);
    expect(options[0]).toEqual({ name: "p0", display: "Critical", configured: true });
    expect(options[1]).toEqual({ name: "p1", display: "p1", configured: true });
  });

  // The dialog writes every field back on save, so an option the board dropped
  // is a priority the next save would erase.
  test("a value the project dropped stays selectable, marked unconfigured", () => {
    const options = priorityOptions(PRIORITIES, ["urgent"]);
    expect(options).toHaveLength(5);
    expect(options[4]).toEqual({ name: "urgent", display: "urgent", configured: false });
  });

  test("an extra that is configured is not repeated", () => {
    expect(priorityOptions(PRIORITIES, ["p0"])).toHaveLength(PRIORITIES.length);
  });

  test("extras are deduplicated, sorted, and never empty", () => {
    const extras = priorityOptions(PRIORITIES, ["zzz", "aaa", "zzz", ""]).slice(PRIORITIES.length);
    expect(extras.map((option) => option.name)).toEqual(["aaa", "zzz"]);
  });

  test("an empty vocabulary still offers what the task carries", () => {
    expect(priorityOptions([], ["urgent"]).map((option) => option.name)).toEqual(["urgent"]);
  });
});

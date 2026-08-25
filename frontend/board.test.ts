import { describe, expect, test } from "bun:test";
import {
  indexTasks,
  isReady,
  pendingDependencies,
  visibleTasks,
  type Filters,
  type Priority,
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
    task("TQ-0004", "backlog"),
  ];
  const index = indexTasks(tasks);

  test("a task without dependencies has none pending", () => {
    expect(pendingDependencies(task("TQ-0100", "todo"), index)).toEqual([]);
  });

  test("depends_on may be absent altogether", () => {
    const { depends_on, ...without } = task("TQ-0100", "todo");
    expect(pendingDependencies(without, index)).toEqual([]);
  });

  test("a done dependency is not pending", () => {
    expect(pendingDependencies(task("TQ-0100", "todo", "TQ-0001"), index)).toEqual([]);
  });

  test("every unfinished status is pending", () => {
    const pending = pendingDependencies(task("TQ-0100", "todo", "TQ-0002", "TQ-0003", "TQ-0004"), index);
    expect(pending).toEqual(["TQ-0002", "TQ-0003", "TQ-0004"]);
  });

  test("a missing dependency is pending rather than ignored", () => {
    expect(pendingDependencies(task("TQ-0100", "todo", "TQ-9999"), index)).toEqual(["TQ-9999"]);
  });

  test("the declared order is kept and done dependencies drop out", () => {
    const pending = pendingDependencies(task("TQ-0100", "todo", "TQ-0002", "TQ-0001", "TQ-9999"), index);
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
    task("TQ-0008", "backlog", "TQ-0001", "TQ-0007"),
  ];
  const index = indexTasks(tasks);

  const cases: [string, boolean, string][] = [
    ["TQ-0002", true, "no dependencies"],
    ["TQ-0003", true, "dependency is done"],
    ["TQ-0008", true, "all dependencies done, backlog still counts"],
    ["TQ-0004", false, "dependency is not done"],
    ["TQ-0005", false, "dependency is missing"],
    ["TQ-0006", false, "already in progress"],
    ["TQ-0001", false, "already done"],
  ];

  for (const [id, want, why] of cases) {
    test(`${id} is ${want ? "ready" : "not ready"}: ${why}`, () => {
      expect(isReady(index.get(id)!, index)).toBe(want);
    });
  }

  test("an in-progress task is not ready even with every dependency done", () => {
    expect(isReady(task("TQ-0100", "in-progress", "TQ-0001"), index)).toBe(false);
  });

  test("a blocked task that is already done is still not ready", () => {
    expect(isReady(task("TQ-0100", "done", "TQ-9999"), index)).toBe(false);
  });
});

describe("visibleTasks", () => {
  const tasks = [
    { ...task("TQ-0001", "done"), labels: ["backend"], assignee: "agent-api" },
    { ...task("TQ-0002", "todo"), labels: ["backend", "auth"], assignee: "agent-ui", priority: "high" as Priority },
    { ...task("TQ-0003", "in-progress"), labels: [], assignee: "" },
    { ...task("TQ-0004", "todo", "TQ-0003"), labels: ["Frontend"], assignee: "Agent-UI" },
  ];

  test("no filters keeps every task, in order", () => {
    expect(visibleTasks(tasks, NO_FILTERS).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
      "TQ-0003",
      "TQ-0004",
    ]);
  });

  test("status matches exactly", () => {
    expect(visibleTasks(tasks, filters({ status: "todo" })).map((t) => t.id)).toEqual(["TQ-0002", "TQ-0004"]);
  });

  test("priority matches exactly, and an unset priority reads as normal", () => {
    expect(visibleTasks(tasks, filters({ priority: "high" })).map((t) => t.id)).toEqual(["TQ-0002"]);
    const unset = [{ ...task("TQ-0100", "todo"), priority: undefined }];
    expect(visibleTasks(unset, filters({ priority: "normal" }))).toEqual([]);
  });

  test("assignee matches a substring, case-insensitively", () => {
    expect(visibleTasks(tasks, filters({ assignee: "agent" })).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
      "TQ-0004",
    ]);
    expect(visibleTasks(tasks, filters({ assignee: "UI" })).map((t) => t.id)).toEqual(["TQ-0002", "TQ-0004"]);
  });

  test("surrounding whitespace in a search box is ignored", () => {
    expect(visibleTasks(tasks, filters({ assignee: "  agent-api  " })).map((t) => t.id)).toEqual(["TQ-0001"]);
  });

  test("label matches a substring of any label, case-insensitively", () => {
    expect(visibleTasks(tasks, filters({ label: "end" })).map((t) => t.id)).toEqual([
      "TQ-0001",
      "TQ-0002",
      "TQ-0004",
    ]);
    expect(visibleTasks(tasks, filters({ label: "auth" })).map((t) => t.id)).toEqual(["TQ-0002"]);
  });

  test("a task without an assignee or labels is dropped by those filters", () => {
    expect(visibleTasks(tasks, filters({ assignee: "a" })).map((t) => t.id)).not.toContain("TQ-0003");
    expect(visibleTasks(tasks, filters({ label: "a" })).map((t) => t.id)).not.toContain("TQ-0003");
  });

  test("the ready filter keeps only unblocked, unclaimed tasks", () => {
    expect(visibleTasks(tasks, filters({ ready: true })).map((t) => t.id)).toEqual(["TQ-0002"]);
  });

  test("filters combine", () => {
    expect(visibleTasks(tasks, filters({ status: "todo", label: "backend" })).map((t) => t.id)).toEqual([
      "TQ-0002",
    ]);
  });

  test("readiness is judged against every task, not the visible ones", () => {
    // TQ-0004 depends on TQ-0003, which the status filter hides. Hiding a
    // dependency must not turn it into a missing one.
    const done = tasks.map((t) => (t.id === "TQ-0003" ? { ...t, status: "done" as Status } : t));
    expect(visibleTasks(done, filters({ status: "todo", ready: true })).map((t) => t.id)).toEqual([
      "TQ-0002",
      "TQ-0004",
    ]);
  });

  test("an empty list stays empty", () => {
    expect(visibleTasks([], filters({ ready: true }))).toEqual([]);
  });
});

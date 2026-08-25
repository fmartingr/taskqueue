/**
 * The board's decisions, taken away from the DOM.
 *
 * Everything here maps data to data: the task shape the API returns, the
 * dependency lookups the cards need, and the filter bar. app.ts keeps the
 * fetching, the rendering and the event wiring, so the rules below can be
 * unit-tested with `bun test` and no browser.
 *
 * isReady mirrors IsReady/IsBlocked in internal/task/task.go, which `tq ready`
 * and GET /api/tasks?ready=true go through. It is a second implementation of
 * the same rule, so the two have to agree — including on a missing dependency,
 * which blocks rather than being ignored.
 */

export const STATUSES = ["backlog", "todo", "in-progress", "done"] as const;
export const PRIORITIES = ["urgent", "high", "normal", "low"] as const;

export type Status = (typeof STATUSES)[number];
export type Priority = (typeof PRIORITIES)[number];

export interface Task {
  id: string;
  title: string;
  status: Status;
  priority?: Priority;
  assignee?: string;
  labels?: string[];
  depends_on?: string[];
  created: string;
  updated: string;
  body: string;
}

export interface Filters {
  status: string;
  priority: string;
  assignee: string;
  label: string;
  ready: boolean;
}

// ── Dependencies ────────────────────────────────────────────────

export function indexTasks(tasks: Task[]): Map<string, Task> {
  return new Map(tasks.map((task) => [task.id, task]));
}

/** Returns the dependencies that are missing or not done yet. */
export function pendingDependencies(task: Task, index: Map<string, Task>): string[] {
  return (task.depends_on ?? []).filter((id) => index.get(id)?.status !== "done");
}

export function isReady(task: Task, index: Map<string, Task>): boolean {
  if (task.status === "done" || task.status === "in-progress") return false;
  return pendingDependencies(task, index).length === 0;
}

// ── Filtering ───────────────────────────────────────────────────

/**
 * Applies the filter bar. It needs the whole task set rather than a slice of
 * it, because readiness depends on the state of the tasks a filter is hiding.
 */
export function visibleTasks(tasks: Task[], filters: Filters): Task[] {
  const { status, priority, assignee, label, ready } = filters;
  const index = indexTasks(tasks);

  // The assignee and label boxes are search fields, so they match substrings:
  // typing "agent" keeps agent-api and agent-ui.
  const matches = (haystack: string, needle: string) =>
    haystack.toLowerCase().includes(needle.trim().toLowerCase());

  return tasks.filter((task) => {
    if (status && task.status !== status) return false;
    if (priority && task.priority !== priority) return false;
    if (assignee && !matches(task.assignee ?? "", assignee)) return false;
    if (label && !(task.labels ?? []).some((l) => matches(l, label))) return false;
    if (ready && !isReady(task, index)) return false;
    return true;
  });
}

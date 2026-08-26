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

export type Status = (typeof STATUSES)[number];

export interface Task {
  id: string;
  title: string;
  status: Status;
  /** Whatever the file carries: the vocabulary is the project's, and a task
   *  may still hold a value it has since dropped. */
  priority?: string;
  assignee?: string;
  labels?: string[];
  depends_on?: string[];
  created: string;
  updated: string;
  body: string;
}

/** One entry of the project's label vocabulary, as GET /api/config returns it. */
export interface LabelDef {
  color: string;
  display_name: string;
}

/** The vocabulary, keyed by the label exactly as task frontmatter stores it. */
export type LabelSet = Record<string, LabelDef>;

/**
 * One level of the project's priority vocabulary, as GET /api/config returns
 * it. A list rather than a map, because priorities are ordered: the position is
 * the rank, most severe first, and that is the board's sort and the order of
 * its options.
 */
export interface PriorityDef {
  name: string;
  color: string;
  display_name: string;
  default?: boolean;
}

export type PrioritySet = PriorityDef[];

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

// ── Labels ──────────────────────────────────────────────────────

/**
 * Labels stay freeform: the configured set supplies colours, display names and
 * grouping, but a label outside it is legal everywhere and simply renders with
 * nothing to draw it. That is why every function here falls back to the label
 * itself rather than treating an unknown one as an error.
 *
 * The separator groups labels for display only, the way GitLab groups scoped
 * labels. Storage stays one flat string, which is why filtering matches the
 * whole label and never its prefix.
 */
export const LABEL_SEPARATOR = "/";

/**
 * Whether the project declares this label.
 *
 * Object.hasOwn rather than `in`: the set is a plain object parsed from JSON, so
 * `in` also answers yes for "constructor", "toString" and everything else on
 * Object.prototype — and a task may legitimately carry any of those as a label.
 */
export function isConfigured(name: string, labels: LabelSet): boolean {
  return Object.hasOwn(labels, name);
}

function definitionOf(name: string, labels: LabelSet): LabelDef | undefined {
  return isConfigured(name, labels) ? labels[name] : undefined;
}

/** What the board shows for a label: its display name, or the label itself. */
export function labelDisplay(name: string, labels: LabelSet): string {
  return definitionOf(name, labels)?.display_name || name;
}

/** Every label the tasks actually carry, deduplicated and sorted. */
export function labelsInUse(tasks: Task[]): string[] {
  const names = new Set<string>();
  for (const task of tasks) for (const label of task.labels ?? []) names.add(label);
  return [...names].sort();
}

export interface Chip {
  background: string;
  text: string;
}

/**
 * Text colours for a chip. They are fixed rather than themed on purpose: the
 * chip carries its own background, so what it needs to contrast with is the
 * configured colour, not the page behind it. That is what lets one set of
 * colours in .taskqueue.yaml stay readable in both themes.
 */
export const CHIP_DARK_TEXT = "#111418";
export const CHIP_LIGHT_TEXT = "#ffffff";

const HEX_COLOR = /^#(?:[0-9a-f]{3}|[0-9a-f]{6})$/i;

/**
 * How to draw a configured label, or null when there is nothing to draw it
 * with — an unconfigured label, or a colour this board cannot parse. A null
 * chip is not a failure: it is the neutral rendering the ticket asks for.
 */
export function labelChip(name: string, labels: LabelSet): Chip | null {
  const color = definitionOf(name, labels)?.color ?? "";
  if (!HEX_COLOR.test(color)) return null;
  return { background: color, text: readableText(color) };
}

/** Black or white, whichever contrasts more with the background. */
function readableText(color: string): string {
  const background = luminance(color);
  const onDark = contrast(background, luminance(CHIP_DARK_TEXT));
  const onLight = contrast(background, luminance(CHIP_LIGHT_TEXT));
  return onDark >= onLight ? CHIP_DARK_TEXT : CHIP_LIGHT_TEXT;
}

/** WCAG relative luminance, which is what a contrast ratio is computed from. */
function luminance(color: string): number {
  const digits = color.slice(1);
  const full = digits.length === 3 ? [...digits].map((digit) => digit + digit).join("") : digits;
  const [red, green, blue] = [0, 2, 4].map((at) => {
    const value = parseInt(full.slice(at, at + 2), 16) / 255;
    return value <= 0.03928 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  }) as [number, number, number];
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue;
}

const contrast = (a: number, b: number) => (Math.max(a, b) + 0.05) / (Math.min(a, b) + 0.05);

export interface GroupedLabel {
  name: string;
  display: string;
  /** False for a label tasks carry that the project does not declare. */
  configured: boolean;
}

export interface LabelGroup {
  /** The prefix before the first separator, or "" for a label without one. */
  prefix: string;
  labels: GroupedLabel[];
}

/**
 * The label list the filter bar offers: everything the project declares plus
 * everything the tasks actually carry, grouped by prefix. Both are needed —
 * a configured label nothing uses is still part of the vocabulary, and a label
 * in use that nothing declares still has to be filterable.
 */
export function groupLabels(labels: LabelSet, inUse: string[]): LabelGroup[] {
  const names = [...new Set([...Object.keys(labels), ...inUse])].filter((name) => name !== "").sort();

  const groups = new Map<string, GroupedLabel[]>();
  for (const name of names) {
    const at = name.indexOf(LABEL_SEPARATOR);
    const prefix = at > 0 ? name.slice(0, at) : "";
    const group = groups.get(prefix) ?? [];
    group.push({ name, display: labelDisplay(name, labels), configured: isConfigured(name, labels) });
    groups.set(prefix, group);
  }

  // Ungrouped labels first — the flat types — then the groups by name, so the
  // bar reads the same on every render.
  return [...groups.entries()]
    .sort(([a], [b]) => (a === "" ? -1 : b === "" ? 1 : a < b ? -1 : 1))
    .map(([prefix, group]) => ({ prefix, labels: group }));
}

// ── Priorities ──────────────────────────────────────────────────

/**
 * Priorities are the closed set labels are not: the store refuses a value
 * outside the project's vocabulary, so the selects offer exactly what it
 * declares and nothing else.
 *
 * Reading stays tolerant all the same. A task filed before the vocabulary
 * changed still carries the old value, and the board has to show it rather than
 * quietly present some other one — the task dialog writes back whatever its
 * select holds, so an option the board dropped is a priority the next save
 * would erase. That is what the extras argument to priorityOptions is for.
 */

export function findPriority(name: string, priorities: PrioritySet): PriorityDef | undefined {
  return priorities.find((priority) => priority.name === name);
}

/** What a task with no priority of its own is filed under. */
export function defaultPriority(priorities: PrioritySet): string {
  return (priorities.find((priority) => priority.default) ?? priorities[0])?.name ?? "";
}

/** What the board shows for a priority: its display name, or the value itself. */
export function priorityDisplay(name: string, priorities: PrioritySet): string {
  return findPriority(name, priorities)?.display_name || name;
}

/**
 * How to draw a priority badge, or null when there is nothing to draw it with —
 * a value the project no longer declares, or a colour this board cannot parse.
 * A null chip is not a failure: it is the neutral rendering.
 *
 * A filled chip rather than coloured text, for the reason labelChip is one: the
 * configured colour is what the text has to contrast with, and only a chip that
 * carries its own background can promise that in both themes.
 */
export function priorityChip(name: string, priorities: PrioritySet): Chip | null {
  const color = findPriority(name, priorities)?.color ?? "";
  if (!HEX_COLOR.test(color)) return null;
  return { background: color, text: readableText(color) };
}

export interface PriorityOption {
  name: string;
  display: string;
  /** False for a value a task carries that the project does not declare. */
  configured: boolean;
}

/**
 * The options a priority select offers: the vocabulary in rank order, then any
 * extra value that has to stay selectable — the priority of the task being
 * edited, or the one the filter bar is on — even though the project has dropped
 * it. Dropping such an option instead would leave the filter bar reading as one
 * priority while the board hid everything, and the task dialog reading as
 * another while its next save rewrote the file.
 */
export function priorityOptions(priorities: PrioritySet, extras: string[]): PriorityOption[] {
  const options = priorities.map((priority) => ({
    name: priority.name,
    display: priority.display_name || priority.name,
    configured: true,
  }));

  const unknown = [...new Set(extras)]
    .filter((name) => name !== "" && !findPriority(name, priorities))
    .sort();
  return [...options, ...unknown.map((name) => ({ name, display: name, configured: false }))];
}

// ── Filtering ───────────────────────────────────────────────────

/**
 * Applies the filter bar. It needs the whole task set rather than a slice of
 * it, because readiness depends on the state of the tasks a filter is hiding.
 */
export function visibleTasks(tasks: Task[], filters: Filters): Task[] {
  const { status, priority, assignee, label, ready } = filters;
  const index = indexTasks(tasks);

  // The assignee box is a search field, so it matches substrings: typing
  // "agent" keeps agent-api and agent-ui. The label filter is not — it is a
  // list of the labels that exist — so it matches a label whole, which also
  // keeps "component/backend" from being selected by "backend".
  const matches = (haystack: string, needle: string) =>
    haystack.toLowerCase().includes(needle.trim().toLowerCase());

  return tasks.filter((task) => {
    if (status && task.status !== status) return false;
    if (priority && task.priority !== priority) return false;
    if (assignee && !matches(task.assignee ?? "", assignee)) return false;
    if (label && !(task.labels ?? []).includes(label)) return false;
    if (ready && !isReady(task, index)) return false;
    return true;
  });
}

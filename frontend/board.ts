/**
 * The board's decisions, taken away from the DOM.
 *
 * Everything here maps data to data: the task shape the API returns, the
 * dependency lookups the cards need, and the filtering. The components keep
 * the fetching, the rendering and the event wiring, so the rules below can be
 * unit-tested with `bun test` and no browser — and no Vue.
 *
 * isReady mirrors IsReady/IsBlocked in internal/task/task.go, which `tq ready`
 * and GET /api/tasks?ready=true go through. It is a second implementation of
 * the same rule, so the two have to agree — including on a missing dependency,
 * which blocks rather than being ignored.
 */

/**
 * A status is whatever the board is configured with, so it is a plain string.
 * The columns come from GET /api/config; nothing here hard-codes them.
 */
export type Status = string;

/** One column of the project's board, as GET /api/config returns it. */
export interface ColumnDef {
  name: string;
  display_name: string;
  /** Absent means false: only a column that says so offers work to `tq ready`. */
  consider_ready?: boolean;
  /** Absent means false: a task here counts as finished. */
  consider_done?: boolean;
  default?: boolean;
}

export type ColumnSet = ColumnDef[];

/**
 * Fallback when GET /api/config is unavailable; must stay in sync with
 * internal/config/columns.go and internal/task/columns.go.
 */
export const FALLBACK_COLUMNS: ColumnSet = [
  { name: "inbox", display_name: "Inbox", default: true },
  { name: "todo", display_name: "To do", consider_ready: true },
  { name: "in-progress", display_name: "In Progress" },
  { name: "done", display_name: "Done", consider_done: true },
  { name: "rejected", display_name: "Rejected" },
];

/** Lookup only: the store resolves aliases and removed columns before the API. */
export function findColumn(status: string, columns: ColumnSet): ColumnDef | undefined {
  return columns.find((column) => column.name === status);
}

export function columnDisplay(status: string, columns: ColumnSet): string {
  return findColumn(status, columns)?.display_name || status;
}

export function columnOffersWork(status: string, columns: ColumnSet): boolean {
  return findColumn(status, columns)?.consider_ready === true;
}

export function columnSatisfies(status: string, columns: ColumnSet): boolean {
  return findColumn(status, columns)?.consider_done === true;
}

export function defaultColumn(columns: ColumnSet): string {
  return (columns.find((column) => column.default) ?? columns[0])?.name ?? "";
}

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

/**
 * One piece of free text from the search bar: a bare word, or a quoted phrase.
 *
 * Every term has to be carried, so `auth token` is two terms and both must be
 * found — a search box is expected to narrow as words are added. `negated` is
 * the `-auth` form: the term must not be found.
 */
export interface TextTerm {
  value: string;
  negated: boolean;
}

/**
 * A value a task must not carry — the `-priority=low` form.
 *
 * A list rather than a slot per key, unlike the positive terms below: there is
 * one slot per field because the newest thing typed is the one meant, but
 * excluding two labels at once is a sensible thing to ask for.
 */
export interface ExcludedTerm {
  key: "status" | "priority" | "assignee" | "label";
  value: string;
}

export interface Filters {
  status: string;
  priority: string;
  assignee: string;
  label: string;
  ready: boolean;
  /**
   * Free text from the search bar: words and phrases the id, the title or the
   * body has to carry. It is a filter like any other rather than a mode of its
   * own — one query line stands for this whole set, see search.ts.
   */
  text: TextTerm[];
  /** What the search bar's `-` terms exclude; nothing else writes to it. */
  excluded: ExcludedTerm[];
}

// ── Dependencies ────────────────────────────────────────────────

export function indexTasks(tasks: Task[]): Map<string, Task> {
  return new Map(tasks.map((task) => [task.id, task]));
}

/** Returns the dependencies that are missing or not done yet. */
export function pendingDependencies(task: Task, index: Map<string, Task>, columns: ColumnSet): string[] {
  return (task.depends_on ?? []).filter((id) => {
    const other = index.get(id);
    // A missing dependency blocks rather than being ignored, which is what
    // task.IsBlocked does too — the two are one rule, written twice.
    return other === undefined || !columnSatisfies(other.status, columns);
  });
}

export function isReady(task: Task, index: Map<string, Task>, columns: ColumnSet): boolean {
  if (!columnOffersWork(task.status, columns)) return false;
  return pendingDependencies(task, index, columns).length === 0;
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

/**
 * The two halves the board draws a label as, GitLab style. A label carrying the
 * separator splits at the first one: the scope says what kind of label it is,
 * the value says which one. A label without a separator has no scope and stays
 * the single-tone pill it has always been.
 */
export interface LabelHalves {
  /** Title-cased, or "" for a label the board draws as one piece. */
  scope: string;
  value: string;
}

/** The joiner where only one line of text fits, and there is no second half. */
export const LABEL_JOINER = " | ";

/**
 * Title-cases every segment. Nothing derives "API" from "api" — casing is the
 * whole remaining job of a display name — but a key with none still reads
 * better capitalised than raw.
 */
function titleCase(text: string): string {
  return text
    .split(LABEL_SEPARATOR)
    .map((segment) => segment.charAt(0).toUpperCase() + segment.slice(1))
    .join(LABEL_SEPARATOR);
}

/**
 * How the board splits a label into halves.
 *
 * A display name decorates the value half only: the scope always comes from the
 * key, so `component/api` reads "Component | API" rather than losing the half
 * that says it is a component at all. A display name equal to the key is the
 * absent case — internal/config fills an empty one in with the key, because
 * that is what `tq label list` prints — so the halves come from the key there
 * too.
 *
 * A key with an empty half (`/x`, `x/`) is not scoped: half a pill says less
 * than the whole key does.
 */
export function labelHalves(name: string, labels: LabelSet): LabelHalves {
  const configured = definitionOf(name, labels)?.display_name ?? "";
  const named = configured === name ? "" : configured;

  const at = name.indexOf(LABEL_SEPARATOR);
  const scope = at > 0 ? name.slice(0, at) : "";
  const value = at > 0 ? name.slice(at + LABEL_SEPARATOR.length) : "";
  if (scope === "" || value === "") return { scope: "", value: named || name };
  return { scope: titleCase(scope), value: named || titleCase(value) };
}

/**
 * What the board shows for a label where only one line of text fits — a row of
 * the search bar's suggestion menu, where the two halves a chip draws have to
 * be spelled out.
 */
export function labelDisplay(name: string, labels: LabelSet): string {
  const { scope, value } = labelHalves(name, labels);
  return scope === "" ? value : scope + LABEL_JOINER + value;
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
 *
 * It is the colour-filled part: the whole pill on a label with no scope, and
 * the scope half on one that has a scope. The other half is the page's own
 * surface, so it takes its colours from the theme rather than from here.
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

// ── Priorities ──────────────────────────────────────────────────

/**
 * Priorities are the closed set labels are not: the store refuses a value
 * outside the project's vocabulary, so the selects offer exactly what it
 * declares and nothing else.
 *
 * Reading stays tolerant: a task outside the vocabulary still carries its value,
 * and the board must show it — the dialog writes back whatever its select holds,
 * so a dropped option would be erased on save. That is what the extras argument
 * to priorityOptions is for.
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
 * The options a priority select offers: the vocabulary in rank order, then the
 * priority of the task being edited, even though the project has dropped it.
 * Dropping such an option instead would leave the task dialog reading as one
 * priority while its next save rewrote the file with another.
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
 * Applies the search bar's filter set. It needs the whole task set rather than
 * a slice of it, because readiness depends on the state of the tasks a filter
 * is hiding.
 */
export function visibleTasks(tasks: Task[], filters: Filters, columns: ColumnSet): Task[] {
  const { status, priority, assignee, label, ready, text, excluded } = filters;
  const index = indexTasks(tasks);

  return tasks.filter((task) => {
    if (status && !isValue(task.status, status)) return false;
    if (priority && !isValue(task.priority ?? "", priority)) return false;
    if (assignee && !contains(task.assignee ?? "", assignee)) return false;
    if (label && !carriesLabel(task, label)) return false;
    if (ready && !isReady(task, index, columns)) return false;

    for (const term of excluded ?? []) {
      if (carriesValue(task, term)) return false;
    }
    // Every word has to be found, and every negated one must not be: a search
    // box narrows as words are added rather than looking for the line typed.
    for (const term of text ?? []) {
      const wanted = term.value.trim().toLowerCase();
      if (wanted === "") continue;
      if (carriesText(task, wanted) === term.negated) return false;
    }
    return true;
  });
}

/**
 * Whether a task's value is the one asked for. Case is ignored throughout:
 * the query line is hand-typed as often as it is completed, and `priority=Urgent`
 * finding nothing is a bug rather than a strictness (TQ-0068).
 */
function isValue(held: string, wanted: string): boolean {
  return held.toLowerCase() === wanted.trim().toLowerCase();
}

/** `assignee=` matches substrings, because a name is not a vocabulary: typing
 *  `assignee=agent` keeps agent-api and agent-ui. */
function contains(haystack: string, needle: string): boolean {
  return haystack.toLowerCase().includes(needle.trim().toLowerCase());
}

/** `label=` names one of the labels that exist rather than searching for text,
 *  so it matches a label whole — which keeps `label=backend` from selecting
 *  "component/backend". */
function carriesLabel(task: Task, wanted: string): boolean {
  return (task.labels ?? []).some((label) => isValue(label, wanted));
}

/** Whether a task carries the value an exclusion names, by the same rule the
 *  positive filter for that key uses. */
function carriesValue(task: Task, term: ExcludedTerm): boolean {
  switch (term.key) {
    case "status":
      return isValue(task.status, term.value);
    case "priority":
      return isValue(task.priority ?? "", term.value);
    case "assignee":
      return contains(task.assignee ?? "", term.value);
    case "label":
      return carriesLabel(task, term.value);
  }
}

/**
 * Whether free text is anywhere in a task the board can see: its id, its title
 * or its body, matched case-insensitively.
 *
 * The body is in it because the listing already carries it — the search costs a
 * pass over strings the board has in hand, and grepping `.tasks` for a word is
 * how anyone finds a task on the CLI. Everything else a task holds is a
 * structured term of its own, so nothing here matches a status or a label by
 * accident.
 */
function carriesText(task: Task, wanted: string): boolean {
  return [task.id, task.title, task.body].some((field) => (field ?? "").toLowerCase().includes(wanted));
}

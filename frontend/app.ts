/**
 * tq Kanban board.
 *
 * No framework on purpose: the board fetches JSON from the same REST API the
 * CLI writes through, re-renders the columns from that data, and polls so that
 * tasks created or moved by an agent show up on their own.
 */

const STATUSES = ["backlog", "todo", "in-progress", "done"] as const;
const PRIORITIES = ["urgent", "high", "normal", "low"] as const;

type Status = (typeof STATUSES)[number];
type Priority = (typeof PRIORITIES)[number];

interface Task {
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

interface TaskInput {
  title: string;
  status?: string;
  priority?: string;
  assignee?: string;
  labels?: string[];
  depends_on?: string[];
  body?: string;
}

interface ServerStatus {
  ok: boolean;
  task_count: number;
  task_dir: string;
  version: string;
}

interface Filters {
  status: string;
  priority: string;
  assignee: string;
  label: string;
  ready: boolean;
}

const POLL_INTERVAL_MS = 3000;

const state = {
  tasks: [] as Task[],
  filters: { status: "", priority: "", assignee: "", label: "", ready: false } as Filters,
  /** Serialized last response, used to skip pointless re-renders while polling. */
  lastPayload: "",
  dragging: null as string | null,
  openTaskID: null as string | null,
  /** Body of the open task, split so notes can be edited on their own. */
  openBody: { content: "", notes: [] as Note[], trailing: "" } as SplitBody,
  /** Column whose inline "add a card" composer is open, with its draft text. */
  composing: null as Status | null,
  draft: "",
  taskDir: "",
  version: "",
};

// ── Elements ────────────────────────────────────────────────────

function byId<T extends HTMLElement>(id: string): T {
  const element = document.getElementById(id);
  if (!element) throw new Error(`missing element #${id}`);
  return element as T;
}

const board = byId<HTMLElement>("board");
const toasts = byId<HTMLElement>("toasts");
const statusLine = byId<HTMLElement>("status-line");
const taskDialog = byId<HTMLDialogElement>("task-dialog");
const createDialog = byId<HTMLDialogElement>("create-dialog");

// ── API ─────────────────────────────────────────────────────────

async function api<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const init: RequestInit = { method };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }

  const response = await fetch(path, init);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  return (await response.json()) as T;
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string };
    if (payload.error) return payload.error;
  } catch {
    // The body was not JSON; fall back to the status line.
  }
  return `${response.status} ${response.statusText}`;
}

const fetchTasks = () => api<Task[]>("/api/tasks");
const createTask = (input: TaskInput) => api<Task>("/api/tasks", "POST", input);
const patchTask = (id: string, patch: Partial<TaskInput>) => api<Task>(`/api/tasks/${id}`, "PATCH", patch);
const addNote = (id: string, text: string) => api<Task>(`/api/tasks/${id}/notes`, "POST", { text });

// ── Notes ───────────────────────────────────────────────────────
//
// Notes live in the Markdown body under a "## Notes" heading, exactly as the
// CLI writes them — the file stays the source of truth. The board splits them
// out for display and puts them back together on save.

const NOTES_HEADING = "## Notes";
const NOTE_PATTERN = /^-\s+(\S+)\s+—\s+([\s\S]*)$/;

interface Note {
  /** RFC 3339 timestamp, or "" for a bullet tq did not write. */
  timestamp: string;
  text: string;
}

interface SplitBody {
  content: string;
  notes: Note[];
  /** Anything after the notes section, kept so nothing is lost on save. */
  trailing: string;
}

function splitBody(body: string): SplitBody {
  const lines = body.split("\n");
  const start = lines.findIndex((line) => line.trim() === NOTES_HEADING);
  if (start === -1) {
    return { content: body.trim(), notes: [], trailing: "" };
  }

  let end = lines.length;
  for (let i = start + 1; i < lines.length; i++) {
    if (lines[i].trim().startsWith("## ")) {
      end = i;
      break;
    }
  }

  const notes: Note[] = [];
  for (const line of lines.slice(start + 1, end)) {
    const note = parseNote(line);
    if (note) notes.push(note);
  }
  return {
    content: lines.slice(0, start).join("\n").trim(),
    notes,
    trailing: lines.slice(end).join("\n").trim(),
  };
}

function parseNote(line: string): Note | null {
  const trimmed = line.trim();
  if (trimmed === "") return null;

  const match = NOTE_PATTERN.exec(trimmed);
  if (match && !Number.isNaN(new Date(match[1]).getTime())) {
    return { timestamp: match[1], text: match[2] };
  }
  // A hand-written bullet: keep it, just without a timestamp.
  return { timestamp: "", text: trimmed.replace(/^[-*]\s+/, "") };
}

function joinBody(body: SplitBody): string {
  const sections = [body.content.trim()];
  if (body.notes.length > 0) {
    sections.push([NOTES_HEADING, "", ...body.notes.map(formatNote)].join("\n"));
  }
  if (body.trailing.trim() !== "") {
    sections.push(body.trailing.trim());
  }
  return sections.filter((section) => section !== "").join("\n\n");
}

function formatNote(note: Note): string {
  const text = note.text.replace(/\s+/g, " ").trim();
  return note.timestamp === "" ? `- ${text}` : `- ${note.timestamp} — ${text}`;
}

// ── Dependencies ────────────────────────────────────────────────

function indexTasks(tasks: Task[]): Map<string, Task> {
  return new Map(tasks.map((task) => [task.id, task]));
}

/** Returns the dependencies that are missing or not done yet. */
function pendingDependencies(task: Task, index: Map<string, Task>): string[] {
  return (task.depends_on ?? []).filter((id) => index.get(id)?.status !== "done");
}

function isReady(task: Task, index: Map<string, Task>): boolean {
  if (task.status === "done" || task.status === "in-progress") return false;
  return pendingDependencies(task, index).length === 0;
}

// ── Filtering ───────────────────────────────────────────────────

function visibleTasks(): Task[] {
  const { status, priority, assignee, label, ready } = state.filters;
  const index = indexTasks(state.tasks);

  // The assignee and label boxes are search fields, so they match substrings:
  // typing "agent" keeps agent-api and agent-ui.
  const matches = (haystack: string, needle: string) =>
    haystack.toLowerCase().includes(needle.trim().toLowerCase());

  return state.tasks.filter((task) => {
    if (status && task.status !== status) return false;
    if (priority && task.priority !== priority) return false;
    if (assignee && !matches(task.assignee ?? "", assignee)) return false;
    if (label && !(task.labels ?? []).some((l) => matches(l, label))) return false;
    if (ready && !isReady(task, index)) return false;
    return true;
  });
}

// ── Rendering ───────────────────────────────────────────────────

function element<K extends keyof HTMLElementTagNameMap>(
  tag: K,
  className?: string,
  text?: string,
): HTMLElementTagNameMap[K] {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text !== undefined) node.textContent = text;
  return node;
}

function render(): void {
  const tasks = visibleTasks();
  const index = indexTasks(state.tasks);

  board.replaceChildren(
    ...STATUSES.map((status) => renderColumn(status, tasks.filter((task) => task.status === status), index)),
  );

  const total = state.tasks.length;
  const shown = tasks.length;
  const counts = shown === total ? `${total} tasks` : `${shown} of ${total} tasks`;
  statusLine.textContent = [counts, state.taskDir, state.version && `tq ${state.version}`]
    .filter(Boolean)
    .join(" · ");
}

function renderColumn(status: Status, tasks: Task[], index: Map<string, Task>): HTMLElement {
  const column = element("section", "column");
  column.dataset.status = status;

  const header = element("header", "column-header");
  header.append(element("h2", undefined, status), element("span", "count", String(tasks.length)));
  column.append(header);

  const list = element("div", "column-tasks");
  list.append(...tasks.map((task) => renderCard(task, index)));
  column.append(list);
  column.append(renderComposer(status));

  // Native drag and drop: the column is the drop target for its status.
  column.addEventListener("dragover", (event) => {
    event.preventDefault();
    column.classList.add("drop-target");
  });
  column.addEventListener("dragleave", (event) => {
    // dragleave also fires when the pointer moves onto a card inside the
    // column; only drop the highlight when it really left.
    const to = event.relatedTarget;
    if (to instanceof Node && column.contains(to)) return;
    column.classList.remove("drop-target");
  });
  column.addEventListener("drop", (event) => {
    event.preventDefault();
    column.classList.remove("drop-target");
    const id = event.dataTransfer?.getData("text/plain") || state.dragging;
    if (id) void moveTask(id, status);
  });

  return column;
}

/**
 * The composer is either a "+ Add a card" button or, once opened, a card-shaped
 * textarea. Enter or losing focus files the card; an empty one is discarded.
 */
function renderComposer(status: Status): HTMLElement {
  if (state.composing !== status) {
    const open = element("button", "composer-open", "+ Add a card");
    open.type = "button";
    open.addEventListener("click", () => {
      state.composing = status;
      state.draft = "";
      render();
    });
    return open;
  }

  const form = element("div", "composer");
  const input = element("textarea", "composer-input");
  input.rows = 2;
  input.placeholder = "Title";
  input.value = state.draft;
  form.append(input);

  // One-way latch: this node files at most one card, so the blur that follows
  // Enter (and the blur some browsers fire when the node is replaced) is a
  // no-op. Every re-render builds a fresh composer with a fresh latch.
  let settled = false;
  const close = () => {
    state.composing = null;
    state.draft = "";
    render();
  };
  const submit = (keepOpen: boolean) => {
    if (settled) return;
    settled = true;

    const title = input.value.trim();
    if (title === "") {
      close();
      return;
    }

    input.value = ""; // immediate feedback while the request is in flight
    state.draft = "";
    if (!keepOpen) {
      state.composing = null;
    }
    void quickAdd(title, status, keepOpen);
  };

  input.addEventListener("input", () => {
    state.draft = input.value;
  });
  input.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      submit(true);
    } else if (event.key === "Escape") {
      event.preventDefault();
      settled = true;
      close();
    }
  });
  input.addEventListener("blur", () => submit(false));

  // Focus after the browser has the node, restoring the caret while typing.
  queueMicrotask(() => {
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  });
  return form;
}

async function quickAdd(title: string, status: Status, keepOpen: boolean): Promise<void> {
  try {
    await createTask({ title, status });
    await refresh();
    if (keepOpen) {
      // refresh() re-rendered the board: put the cursor back in the composer.
      focusComposer();
    }
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
    // Hand the text back rather than losing what was typed.
    state.composing = status;
    state.draft = title;
    render();
    focusComposer();
  }
}

function focusComposer(): void {
  const input = document.querySelector<HTMLTextAreaElement>(".composer-input");
  input?.focus();
}

function renderCard(task: Task, index: Map<string, Task>): HTMLElement {
  const card = element("article", "card");
  card.draggable = true;
  card.dataset.id = task.id;
  card.tabIndex = 0;

  const top = element("div", "card-top");
  top.append(
    element("span", "task-id", task.id),
    element("span", `badge priority-${task.priority ?? "normal"}`, task.priority ?? "normal"),
  );
  card.append(top, element("p", "card-title", task.title));

  const meta = element("div", "card-meta");
  if (task.assignee) meta.append(element("span", "assignee", task.assignee));
  for (const label of task.labels ?? []) meta.append(element("span", "label", label));

  const noteCount = splitBody(task.body ?? "").notes.length;
  if (noteCount > 0) meta.append(noteBadge(noteCount));
  if (meta.childElementCount > 0) card.append(meta);

  const pending = pendingDependencies(task, index);
  if (pending.length > 0) {
    card.classList.add("blocked");
    card.append(element("p", "blocked-note", `Blocked by ${pending.join(", ")}`));
  }

  card.addEventListener("dragstart", (event) => {
    state.dragging = task.id;
    card.classList.add("dragging");
    event.dataTransfer?.setData("text/plain", task.id);
    if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
  });
  card.addEventListener("dragend", () => {
    state.dragging = null;
    card.classList.remove("dragging");
  });
  card.addEventListener("click", () => openTask(task.id));
  card.addEventListener("keydown", (event) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openTask(task.id);
    }
  });

  return card;
}

const SPEECH_BUBBLE =
  '<svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">' +
  '<path fill="currentColor" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v6a1.5 1.5 0 0 1-1.5 1.5H6.6L3.7 13.7A.5.5 0 0 1 3 13.3V11h-.5A1.5 1.5 0 0 1 1 9.5v-6z"/>' +
  "</svg>";

function noteBadge(count: number): HTMLElement {
  const badge = element("span", "note-badge");
  badge.title = count === 1 ? "1 note" : `${count} notes`;
  badge.innerHTML = SPEECH_BUBBLE; // a constant, no task data involved
  badge.append(String(count));
  return badge;
}

// ── Toasts ──────────────────────────────────────────────────────

function toast(message: string, kind: "error" | "info" = "error"): void {
  const node = element("div", `toast ${kind}`, message);
  toasts.append(node);
  setTimeout(() => node.remove(), 6000);
}

// ── Data ────────────────────────────────────────────────────────

async function refresh(): Promise<void> {
  const tasks = await fetchTasks();
  const payload = JSON.stringify(tasks);
  if (payload === state.lastPayload) return; // nothing changed since the last poll
  state.lastPayload = payload;
  state.tasks = tasks;
  render();
}

/** refreshQuietly keeps polling errors out of the way of the toast stack. */
async function refreshQuietly(): Promise<void> {
  try {
    await refresh();
  } catch (error) {
    console.error("refresh failed", error);
  }
}

async function moveTask(id: string, status: Status): Promise<void> {
  const task = state.tasks.find((candidate) => candidate.id === id);
  if (!task || task.status === status) return;

  try {
    await patchTask(id, { status });
    await refresh();
  } catch (error) {
    toast(`Could not move ${id}: ${describe(error)}`);
    await refreshQuietly(); // fall back to whatever the server actually has
  }
}

function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

function splitList(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

// ── Task dialog ─────────────────────────────────────────────────

function openTask(id: string): void {
  const task = state.tasks.find((candidate) => candidate.id === id);
  if (!task) return;

  state.openTaskID = id;
  byId<HTMLElement>("task-dialog-id").textContent = task.id;
  byId<HTMLInputElement>("task-title").value = task.title;
  byId<HTMLSelectElement>("task-status").value = task.status;
  byId<HTMLSelectElement>("task-priority").value = task.priority ?? "normal";
  byId<HTMLInputElement>("task-assignee").value = task.assignee ?? "";
  byId<HTMLInputElement>("task-labels").value = (task.labels ?? []).join(", ");
  byId<HTMLInputElement>("task-depends-on").value = (task.depends_on ?? []).join(", ");
  state.openBody = splitBody(task.body ?? "");
  byId<HTMLTextAreaElement>("task-body").value = state.openBody.content;
  byId<HTMLInputElement>("task-note").value = "";
  renderNotes();
  byId<HTMLElement>("task-timestamps").textContent =
    `created ${formatTime(task.created)} · updated ${formatTime(task.updated)}`;

  const pending = pendingDependencies(task, indexTasks(state.tasks));
  const blocked = byId<HTMLElement>("task-blocked");
  blocked.textContent = pending.length > 0 ? `Blocked by ${pending.join(", ")}` : "";
  blocked.hidden = pending.length === 0;

  taskDialog.showModal();
}

/** Renders the notes panel from state.openBody. */
function renderNotes(): void {
  const list = byId<HTMLUListElement>("task-notes");
  if (state.openBody.notes.length === 0) {
    const empty = element("li", "notes-empty", "No notes yet.");
    list.replaceChildren(empty);
    return;
  }
  list.replaceChildren(...state.openBody.notes.map(renderNote));
}

function renderNote(note: Note, position: number): HTMLElement {
  const item = element("li", "note");

  const head = element("div", "note-head");
  head.append(element("time", "note-time", note.timestamp === "" ? "note" : formatTime(note.timestamp)));

  const edit = element("button", "ghost icon", "✎");
  edit.type = "button";
  edit.title = "Edit this note";
  edit.setAttribute("aria-label", "Edit this note");
  head.append(edit);
  item.append(head);

  const text = element("p", "note-text", note.text);
  item.append(text);

  edit.addEventListener("click", () => startEditingNote(item, text, position));
  return item;
}

/**
 * Turns a note into a textarea in place. Enter or losing focus keeps the edit,
 * Escape drops it; either way the change is written with the dialog's Save,
 * like every other field.
 */
function startEditingNote(item: HTMLElement, text: HTMLElement, position: number): void {
  const editor = element("textarea", "note-editor");
  editor.value = state.openBody.notes[position]?.text ?? "";
  editor.rows = 2;
  item.replaceChild(editor, text);
  editor.focus();
  editor.setSelectionRange(editor.value.length, editor.value.length);

  let settled = false;
  const finish = (keep: boolean) => {
    if (settled) return;
    settled = true;

    const note = state.openBody.notes[position];
    if (keep && note) {
      const edited = editor.value.trim();
      if (edited !== "") note.text = edited;
    }
    renderNotes();
  };

  editor.addEventListener("keydown", (event) => {
    if (event.key === "Enter" && !event.shiftKey) {
      event.preventDefault();
      finish(true);
    } else if (event.key === "Escape") {
      event.preventDefault();
      finish(false);
    }
  });
  editor.addEventListener("blur", () => finish(true));
}

function formatTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

/** Sends every editable field of the dialog as one patch. */
function patchFromDialog(id: string): Promise<Task> {
  return patchTask(id, {
    title: byId<HTMLInputElement>("task-title").value,
    status: byId<HTMLSelectElement>("task-status").value,
    priority: byId<HTMLSelectElement>("task-priority").value,
    assignee: byId<HTMLInputElement>("task-assignee").value,
    labels: splitList(byId<HTMLInputElement>("task-labels").value),
    depends_on: splitList(byId<HTMLInputElement>("task-depends-on").value),
    body: joinBody({
      ...state.openBody,
      content: byId<HTMLTextAreaElement>("task-body").value,
    }),
  });
}

async function saveOpenTask(): Promise<void> {
  const id = state.openTaskID;
  if (!id) return;

  try {
    await patchFromDialog(id);
    taskDialog.close();
    state.openTaskID = null;
    await refresh();
  } catch (error) {
    toast(`Could not update ${id}: ${describe(error)}`);
  }
}

async function addNoteToOpenTask(): Promise<void> {
  const id = state.openTaskID;
  const input = byId<HTMLInputElement>("task-note");
  const text = input.value.trim();
  if (!id || text === "") return;

  try {
    // Pending edits are saved first: appending a note rewrites the body, and
    // silently dropping what the user typed would be worse than an extra write.
    await patchFromDialog(id);
    const task = await addNote(id, text);
    input.value = "";
    state.openBody = splitBody(task.body ?? "");
    byId<HTMLTextAreaElement>("task-body").value = state.openBody.content;
    renderNotes();
    await refresh();
    toast(`Note added to ${id}`, "info");
  } catch (error) {
    toast(`Could not add a note to ${id}: ${describe(error)}`);
  }
}

// ── Create dialog ───────────────────────────────────────────────

function openCreateDialog(): void {
  byId<HTMLFormElement>("create-form").reset();
  createDialog.showModal();
  byId<HTMLInputElement>("create-title").focus();
}

async function submitCreateDialog(): Promise<void> {
  const title = byId<HTMLInputElement>("create-title").value.trim();
  if (title === "") return;

  try {
    const task = await createTask({
      title,
      status: byId<HTMLSelectElement>("create-status").value,
      priority: byId<HTMLSelectElement>("create-priority").value,
      assignee: byId<HTMLInputElement>("create-assignee").value,
      labels: splitList(byId<HTMLInputElement>("create-labels").value),
      depends_on: splitList(byId<HTMLInputElement>("create-depends-on").value),
      body: byId<HTMLTextAreaElement>("create-body").value,
    });
    createDialog.close();
    await refresh();
    toast(`Created ${task.id}`, "info");
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
  }
}

// ── Wiring ──────────────────────────────────────────────────────

function readFilters(): void {
  state.filters = {
    status: byId<HTMLSelectElement>("filter-status").value,
    priority: byId<HTMLSelectElement>("filter-priority").value,
    assignee: byId<HTMLInputElement>("filter-assignee").value,
    label: byId<HTMLInputElement>("filter-label").value,
    ready: byId<HTMLInputElement>("filter-ready").checked,
  };
  render();
}

function wire(): void {
  for (const id of ["filter-status", "filter-priority", "filter-ready"]) {
    byId<HTMLElement>(id).addEventListener("change", readFilters);
  }
  for (const id of ["filter-assignee", "filter-label"]) {
    byId<HTMLElement>(id).addEventListener("input", readFilters);
  }
  byId<HTMLButtonElement>("filter-reset").addEventListener("click", () => {
    byId<HTMLSelectElement>("filter-status").value = "";
    byId<HTMLSelectElement>("filter-priority").value = "";
    byId<HTMLInputElement>("filter-assignee").value = "";
    byId<HTMLInputElement>("filter-label").value = "";
    byId<HTMLInputElement>("filter-ready").checked = false;
    readFilters();
  });

  byId<HTMLButtonElement>("new-task").addEventListener("click", openCreateDialog);
  byId<HTMLButtonElement>("task-note-add").addEventListener("click", () => void addNoteToOpenTask());

  for (const button of document.querySelectorAll<HTMLButtonElement>("[data-close]")) {
    button.addEventListener("click", () => {
      const dialog = document.getElementById(button.dataset.close ?? "");
      if (dialog instanceof HTMLDialogElement) dialog.close();
    });
  }

  // The forms use method="dialog": intercept the submit and talk to the API.
  byId<HTMLFormElement>("task-form").addEventListener("submit", (event) => {
    event.preventDefault();
    void saveOpenTask();
  });
  byId<HTMLFormElement>("create-form").addEventListener("submit", (event) => {
    event.preventDefault();
    void submitCreateDialog();
  });

  taskDialog.addEventListener("close", () => {
    state.openTaskID = null;
  });
}

async function loadServerStatus(): Promise<void> {
  try {
    const status = await api<ServerStatus>("/api/status");
    state.taskDir = status.task_dir;
    state.version = status.version;
  } catch (error) {
    console.error("status failed", error);
  }
}

async function start(): Promise<void> {
  wire();
  await loadServerStatus();

  try {
    await refresh();
  } catch (error) {
    toast(`Could not load tasks: ${describe(error)}`);
  }

  // Poll so CLI changes show up. Skip while dragging or editing so the board
  // never moves under the user's hands.
  setInterval(() => {
    if (state.dragging || state.composing || taskDialog.open || createDialog.open) return;
    void refreshQuietly();
  }, POLL_INTERVAL_MS);
}

void start();

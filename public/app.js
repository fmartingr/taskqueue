// frontend/notes.ts
var NOTES_HEADING = "## Notes";
var NOTES_RULE = "---";
var FENCE_PATTERN = /^(```|~~~)/;
var HEADING_PATTERN = /^#{1,6}\s/;
var BULLET_PATTERN = /^[-*]\s+/;
var NOTE_PATTERN = /^(\S+)\s+—\s+([\s\S]*)$/;
var CONTINUATION_INDENT = "  ";
function trimBlankLines(text) {
  return text.replace(/^\n+|\n+$/g, "");
}
function splitBody(body) {
  const lines = body.split(`
`);
  const start = notesStart(lines);
  if (start === -1) {
    return { content: trimBlankLines(body), notes: [] };
  }
  let end = start;
  while (end > 0 && lines[end - 1].trim() === "")
    end--;
  if (end > 0 && lines[end - 1].trim() === NOTES_RULE && (end === 1 || lines[end - 2].trim() === "")) {
    end--;
  }
  return {
    content: trimBlankLines(lines.slice(0, end).join(`
`)),
    notes: parseNotes(lines.slice(start + 1))
  };
}
function notesStart(lines) {
  const [start, balanced] = scanNotesStart(lines, true);
  if (balanced)
    return start;
  return scanNotesStart(lines, false)[0];
}
function scanNotesStart(lines, honourFences) {
  let start = -1;
  let fenced = false;
  for (let i = 0;i < lines.length; i++) {
    const trimmed = lines[i].trim();
    if (honourFences && FENCE_PATTERN.test(trimmed)) {
      fenced = !fenced;
      continue;
    }
    if (fenced || !HEADING_PATTERN.test(trimmed))
      continue;
    start = trimmed === NOTES_HEADING ? i : -1;
  }
  return [start, !fenced];
}
function parseNotes(lines) {
  const notes = [];
  let blanks = [];
  for (const line of lines) {
    const text = line.replace(/\s+$/, "");
    if (text === "") {
      blanks.push("");
      continue;
    }
    const indented = /^\s/.test(text);
    if (notes.length === 0 || !indented && BULLET_PATTERN.test(text)) {
      notes.push(parseNote(text.trim().replace(BULLET_PATTERN, "")));
      blanks = [];
      continue;
    }
    const last = notes[notes.length - 1];
    last.text = [last.text, ...blanks, text.replace(/^ {1,2}/, "")].join(`
`);
    blanks = [];
  }
  return notes;
}
function parseNote(text) {
  const match = NOTE_PATTERN.exec(text);
  if (match && !Number.isNaN(new Date(match[1]).getTime())) {
    return { timestamp: match[1], text: match[2] };
  }
  return { timestamp: "", text };
}
function joinBody(body) {
  const content = trimBlankLines(body.content);
  const notes = body.notes.filter((note) => note.text.trim() !== "");
  if (notes.length === 0)
    return content;
  const section = [NOTES_HEADING, "", ...notes.map(formatNote)].join(`
`);
  return content === "" ? section : [content, "", NOTES_RULE, "", section].join(`
`);
}
function formatNote(note) {
  const [first = "", ...rest] = trimBlankLines(note.text).split(`
`);
  const head = note.timestamp === "" ? `- ${first.trim()}` : `- ${note.timestamp} — ${first.trim()}`;
  return [head, ...rest.map((line) => line.trim() === "" ? "" : CONTINUATION_INDENT + line)].join(`
`);
}

// frontend/app.ts
var STATUSES = ["backlog", "todo", "in-progress", "done"];
var POLL_INTERVAL_MS = 3000;
var state = {
  tasks: [],
  filters: { status: "", priority: "", assignee: "", label: "", ready: false },
  lastPayload: "",
  dragging: null,
  openTaskID: null,
  openBody: { content: "", notes: [] },
  composing: null,
  draft: "",
  taskDir: "",
  version: ""
};
function byId(id) {
  const element = document.getElementById(id);
  if (!element)
    throw new Error(`missing element #${id}`);
  return element;
}
var board = byId("board");
var toasts = byId("toasts");
var statusLine = byId("status-line");
var taskDialog = byId("task-dialog");
var createDialog = byId("create-dialog");
async function api(path, method = "GET", body) {
  const init = { method };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }
  const response = await fetch(path, init);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  return await response.json();
}
async function errorMessage(response) {
  try {
    const payload = await response.json();
    if (payload.error)
      return payload.error;
  } catch {}
  return `${response.status} ${response.statusText}`;
}
var fetchTasks = () => api("/api/tasks");
var createTask = (input) => api("/api/tasks", "POST", input);
var patchTask = (id, patch) => api(`/api/tasks/${id}`, "PATCH", patch);
var addNote = (id, text) => api(`/api/tasks/${id}/notes`, "POST", { text });
function indexTasks(tasks) {
  return new Map(tasks.map((task) => [task.id, task]));
}
function pendingDependencies(task, index) {
  return (task.depends_on ?? []).filter((id) => index.get(id)?.status !== "done");
}
function isReady(task, index) {
  if (task.status === "done" || task.status === "in-progress")
    return false;
  return pendingDependencies(task, index).length === 0;
}
function visibleTasks() {
  const { status, priority, assignee, label, ready } = state.filters;
  const index = indexTasks(state.tasks);
  const matches = (haystack, needle) => haystack.toLowerCase().includes(needle.trim().toLowerCase());
  return state.tasks.filter((task) => {
    if (status && task.status !== status)
      return false;
    if (priority && task.priority !== priority)
      return false;
    if (assignee && !matches(task.assignee ?? "", assignee))
      return false;
    if (label && !(task.labels ?? []).some((l) => matches(l, label)))
      return false;
    if (ready && !isReady(task, index))
      return false;
    return true;
  });
}
function element(tag, className, text) {
  const node = document.createElement(tag);
  if (className)
    node.className = className;
  if (text !== undefined)
    node.textContent = text;
  return node;
}
function render() {
  const tasks = visibleTasks();
  const index = indexTasks(state.tasks);
  board.replaceChildren(...STATUSES.map((status) => renderColumn(status, tasks.filter((task) => task.status === status), index)));
  const total = state.tasks.length;
  const shown = tasks.length;
  const counts = shown === total ? `${total} tasks` : `${shown} of ${total} tasks`;
  statusLine.textContent = [counts, state.taskDir, state.version && `tq ${state.version}`].filter(Boolean).join(" · ");
}
function renderColumn(status, tasks, index) {
  const column = element("section", "column");
  column.dataset.status = status;
  const header = element("header", "column-header");
  header.append(element("h2", undefined, status), element("span", "count", String(tasks.length)));
  column.append(header);
  const list = element("div", "column-tasks");
  list.append(...tasks.map((task) => renderCard(task, index)));
  column.append(list);
  column.append(renderComposer(status));
  column.addEventListener("dragover", (event) => {
    event.preventDefault();
    column.classList.add("drop-target");
  });
  column.addEventListener("dragleave", (event) => {
    const to = event.relatedTarget;
    if (to instanceof Node && column.contains(to))
      return;
    column.classList.remove("drop-target");
  });
  column.addEventListener("drop", (event) => {
    event.preventDefault();
    column.classList.remove("drop-target");
    const id = event.dataTransfer?.getData("text/plain") || state.dragging;
    if (id)
      moveTask(id, status);
  });
  return column;
}
function renderComposer(status) {
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
  let settled = false;
  const close = () => {
    state.composing = null;
    state.draft = "";
    render();
  };
  const submit = (keepOpen) => {
    if (settled)
      return;
    settled = true;
    const title = input.value.trim();
    if (title === "") {
      close();
      return;
    }
    input.value = "";
    state.draft = "";
    if (!keepOpen) {
      state.composing = null;
    }
    quickAdd(title, status, keepOpen);
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
  queueMicrotask(() => {
    input.focus();
    input.setSelectionRange(input.value.length, input.value.length);
  });
  return form;
}
async function quickAdd(title, status, keepOpen) {
  try {
    await createTask({ title, status });
    await refresh();
    if (keepOpen) {
      focusComposer();
    }
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
    state.composing = status;
    state.draft = title;
    render();
    focusComposer();
  }
}
function focusComposer() {
  const input = document.querySelector(".composer-input");
  input?.focus();
}
function renderCard(task, index) {
  const card = element("article", "card");
  card.draggable = true;
  card.dataset.id = task.id;
  card.tabIndex = 0;
  const top = element("div", "card-top");
  top.append(element("span", "task-id", task.id), element("span", `badge priority-${task.priority ?? "normal"}`, task.priority ?? "normal"));
  card.append(top, element("p", "card-title", task.title));
  const meta = element("div", "card-meta");
  if (task.assignee)
    meta.append(element("span", "assignee", task.assignee));
  for (const label of task.labels ?? [])
    meta.append(element("span", "label", label));
  const noteCount = splitBody(task.body ?? "").notes.length;
  if (noteCount > 0)
    meta.append(noteBadge(noteCount));
  if (meta.childElementCount > 0)
    card.append(meta);
  const pending = pendingDependencies(task, index);
  if (pending.length > 0) {
    card.classList.add("blocked");
    card.append(element("p", "blocked-note", `Blocked by ${pending.join(", ")}`));
  }
  card.addEventListener("dragstart", (event) => {
    state.dragging = task.id;
    card.classList.add("dragging");
    event.dataTransfer?.setData("text/plain", task.id);
    if (event.dataTransfer)
      event.dataTransfer.effectAllowed = "move";
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
var SPEECH_BUBBLE = '<svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">' + '<path fill="currentColor" d="M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v6a1.5 1.5 0 0 1-1.5 1.5H6.6L3.7 13.7A.5.5 0 0 1 3 13.3V11h-.5A1.5 1.5 0 0 1 1 9.5v-6z"/>' + "</svg>";
function noteBadge(count) {
  const badge = element("span", "note-badge");
  badge.title = count === 1 ? "1 note" : `${count} notes`;
  badge.innerHTML = SPEECH_BUBBLE;
  badge.append(String(count));
  return badge;
}
function toast(message, kind = "error") {
  const node = element("div", `toast ${kind}`, message);
  toasts.append(node);
  setTimeout(() => node.remove(), 6000);
}
async function refresh() {
  const tasks = await fetchTasks();
  const payload = JSON.stringify(tasks);
  if (payload === state.lastPayload)
    return;
  state.lastPayload = payload;
  state.tasks = tasks;
  render();
}
async function refreshQuietly() {
  try {
    await refresh();
  } catch (error) {
    console.error("refresh failed", error);
  }
}
async function moveTask(id, status) {
  const task = state.tasks.find((candidate) => candidate.id === id);
  if (!task || task.status === status)
    return;
  try {
    await patchTask(id, { status });
    await refresh();
  } catch (error) {
    toast(`Could not move ${id}: ${describe(error)}`);
    await refreshQuietly();
  }
}
function describe(error) {
  return error instanceof Error ? error.message : String(error);
}
function splitList(value) {
  return value.split(",").map((item) => item.trim()).filter((item) => item.length > 0);
}
function openTask(id) {
  const task = state.tasks.find((candidate) => candidate.id === id);
  if (!task)
    return;
  state.openTaskID = id;
  byId("task-dialog-id").textContent = task.id;
  byId("task-title").value = task.title;
  byId("task-status").value = task.status;
  byId("task-priority").value = task.priority ?? "normal";
  byId("task-assignee").value = task.assignee ?? "";
  byId("task-labels").value = (task.labels ?? []).join(", ");
  byId("task-depends-on").value = (task.depends_on ?? []).join(", ");
  state.openBody = splitBody(task.body ?? "");
  byId("task-body").value = state.openBody.content;
  byId("task-note").value = "";
  renderNotes();
  byId("task-timestamps").textContent = `created ${formatTime(task.created)} · updated ${formatTime(task.updated)}`;
  const pending = pendingDependencies(task, indexTasks(state.tasks));
  const blocked = byId("task-blocked");
  blocked.textContent = pending.length > 0 ? `Blocked by ${pending.join(", ")}` : "";
  blocked.hidden = pending.length === 0;
  taskDialog.showModal();
}
function renderNotes() {
  const list = byId("task-notes");
  if (state.openBody.notes.length === 0) {
    const empty = element("li", "notes-empty", "No notes yet.");
    list.replaceChildren(empty);
    return;
  }
  list.replaceChildren(...state.openBody.notes.map(renderNote));
}
function renderNote(note, position) {
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
function startEditingNote(item, text, position) {
  const editor = element("textarea", "note-editor");
  editor.value = state.openBody.notes[position]?.text ?? "";
  editor.rows = 2;
  item.replaceChild(editor, text);
  editor.focus();
  editor.setSelectionRange(editor.value.length, editor.value.length);
  let settled = false;
  const finish = (keep) => {
    if (settled)
      return;
    settled = true;
    const note = state.openBody.notes[position];
    if (keep && note) {
      const edited = editor.value.trim();
      if (edited !== "")
        note.text = edited;
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
function formatTime(value) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}
function patchFromDialog(id) {
  return patchTask(id, {
    title: byId("task-title").value,
    status: byId("task-status").value,
    priority: byId("task-priority").value,
    assignee: byId("task-assignee").value,
    labels: splitList(byId("task-labels").value),
    depends_on: splitList(byId("task-depends-on").value),
    body: joinBody({
      ...state.openBody,
      content: byId("task-body").value
    })
  });
}
async function saveOpenTask() {
  const id = state.openTaskID;
  if (!id)
    return;
  try {
    await patchFromDialog(id);
    taskDialog.close();
    state.openTaskID = null;
    await refresh();
  } catch (error) {
    toast(`Could not update ${id}: ${describe(error)}`);
  }
}
async function addNoteToOpenTask() {
  const id = state.openTaskID;
  const input = byId("task-note");
  const text = input.value.trim();
  if (!id || text === "")
    return;
  try {
    await patchFromDialog(id);
    const task = await addNote(id, text);
    input.value = "";
    state.openBody = splitBody(task.body ?? "");
    byId("task-body").value = state.openBody.content;
    renderNotes();
    await refresh();
    toast(`Note added to ${id}`, "info");
  } catch (error) {
    toast(`Could not add a note to ${id}: ${describe(error)}`);
  }
}
function openCreateDialog() {
  byId("create-form").reset();
  createDialog.showModal();
  byId("create-title").focus();
}
async function submitCreateDialog() {
  const title = byId("create-title").value.trim();
  if (title === "")
    return;
  try {
    const task = await createTask({
      title,
      status: byId("create-status").value,
      priority: byId("create-priority").value,
      assignee: byId("create-assignee").value,
      labels: splitList(byId("create-labels").value),
      depends_on: splitList(byId("create-depends-on").value),
      body: byId("create-body").value
    });
    createDialog.close();
    await refresh();
    toast(`Created ${task.id}`, "info");
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
  }
}
function readFilters() {
  state.filters = {
    status: byId("filter-status").value,
    priority: byId("filter-priority").value,
    assignee: byId("filter-assignee").value,
    label: byId("filter-label").value,
    ready: byId("filter-ready").checked
  };
  render();
}
function wire() {
  for (const id of ["filter-status", "filter-priority", "filter-ready"]) {
    byId(id).addEventListener("change", readFilters);
  }
  for (const id of ["filter-assignee", "filter-label"]) {
    byId(id).addEventListener("input", readFilters);
  }
  byId("filter-reset").addEventListener("click", () => {
    byId("filter-status").value = "";
    byId("filter-priority").value = "";
    byId("filter-assignee").value = "";
    byId("filter-label").value = "";
    byId("filter-ready").checked = false;
    readFilters();
  });
  byId("new-task").addEventListener("click", openCreateDialog);
  byId("task-note-add").addEventListener("click", () => void addNoteToOpenTask());
  for (const button of document.querySelectorAll("[data-close]")) {
    button.addEventListener("click", () => {
      const dialog = document.getElementById(button.dataset.close ?? "");
      if (dialog instanceof HTMLDialogElement)
        dialog.close();
    });
  }
  byId("task-form").addEventListener("submit", (event) => {
    event.preventDefault();
    saveOpenTask();
  });
  byId("create-form").addEventListener("submit", (event) => {
    event.preventDefault();
    submitCreateDialog();
  });
  taskDialog.addEventListener("close", () => {
    state.openTaskID = null;
  });
}
async function loadServerStatus() {
  try {
    const status = await api("/api/status");
    state.taskDir = status.task_dir;
    state.version = status.version;
  } catch (error) {
    console.error("status failed", error);
  }
}
async function start() {
  wire();
  await loadServerStatus();
  try {
    await refresh();
  } catch (error) {
    toast(`Could not load tasks: ${describe(error)}`);
  }
  setInterval(() => {
    if (state.dragging || state.composing || taskDialog.open || createDialog.open)
      return;
    refreshQuietly();
  }, POLL_INTERVAL_MS);
}
start();

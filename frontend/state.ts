/**
 * The board's shared state.
 *
 * Everything a component cannot own alone lives here: the tasks, the filter
 * bar, the project's vocabularies, and the four flags that say the user is in
 * the middle of something. Anything a single component *can* own — a composer's
 * draft, a note being edited, the fields of an open dialog — deliberately does
 * not, because that is the state the old imperative board had to keep in a
 * global just to stop a re-render from taking it away.
 *
 * The rules themselves are still in board.ts and notes.ts, which know nothing
 * about Vue and keep their own unit tests.
 */

import { computed, reactive, ref } from "vue";

import { createTask, describe, fetchConfig, fetchStatus, fetchTasks, patchTask } from "./api";
import {
  indexTasks,
  visibleTasks,
  type Filters,
  type LabelSet,
  type PrioritySet,
  type Status,
  type Task,
} from "./board";

export const POLL_INTERVAL_MS = 3000;

/**
 * The built-in vocabulary, mirroring internal/config/priorities.go. It stands
 * in only when GET /api/config could not be read: priorities are a closed set,
 * so with none the selects would be empty and the board could not file a task
 * at all. A project that has no config of its own gets this set from the server
 * anyway, so standing in with it is no worse than being one request behind.
 */
export const FALLBACK_PRIORITIES: PrioritySet = [
  { name: "urgent", color: "#b42318", display_name: "Urgent" },
  { name: "high", color: "#c2410c", display_name: "High" },
  { name: "normal", color: "#4b5563", display_name: "Normal", default: true },
  { name: "low", color: "#6b7280", display_name: "Low" },
];

// ── The data ────────────────────────────────────────────────────

export const tasks = ref<Task[]>([]);
export const filters = reactive<Filters>({
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
});

/** The project's label vocabulary, from GET /api/config. */
export const labels = ref<LabelSet>({});
/** The project's priority vocabulary, in rank order, from GET /api/config. */
export const priorities = ref<PrioritySet>(FALLBACK_PRIORITIES);
export const taskDir = ref("");
export const version = ref("");

export const index = computed(() => indexTasks(tasks.value));
export const visible = computed(() => visibleTasks(tasks.value, filters));

export const statusLine = computed(() => {
  const total = tasks.value.length;
  const shown = visible.value.length;
  const counts = shown === total ? `${total} tasks` : `${shown} of ${total} tasks`;
  return [counts, taskDir.value, version.value && `tq ${version.value}`].filter(Boolean).join(" · ");
});

// ── What the user is in the middle of ───────────────────────────

/** The task being dragged, while a native drag is in flight. */
export const dragging = ref<string | null>(null);
/** The column whose inline composer is open — at most one at a time. */
export const composing = ref<Status | null>(null);
/** The task whose dialog is open, by ID. */
export const openTaskID = ref<string | null>(null);
export const creating = ref(false);

/** The poll stands down for any of it, so the board never moves under a hand. */
export const busy = computed(
  () =>
    dragging.value !== null ||
    composing.value !== null ||
    openTaskID.value !== null ||
    creating.value,
);

// ── Toasts ──────────────────────────────────────────────────────

export interface Toast {
  id: number;
  kind: "error" | "info";
  message: string;
}

const TOAST_MS = 6000;
let nextToastID = 0;

export const toasts = ref<Toast[]>([]);

export function toast(message: string, kind: "error" | "info" = "error"): void {
  const id = nextToastID++;
  toasts.value = [...toasts.value, { id, kind, message }];
  setTimeout(() => {
    toasts.value = toasts.value.filter((candidate) => candidate.id !== id);
  }, TOAST_MS);
}

// ── Loading ─────────────────────────────────────────────────────

/** Serialized last response, so a poll that changed nothing changes nothing. */
let lastPayload = "";

export async function refresh(): Promise<void> {
  const fetched = await fetchTasks();
  const payload = JSON.stringify(fetched);
  if (payload === lastPayload) return;
  lastPayload = payload;
  tasks.value = fetched;
}

/** refreshQuietly keeps polling errors out of the way of the toast stack. */
export async function refreshQuietly(): Promise<void> {
  try {
    await refresh();
  } catch (error) {
    console.error("refresh failed", error);
  }
}

export async function moveTask(id: string, status: Status): Promise<void> {
  const task = tasks.value.find((candidate) => candidate.id === id);
  if (!task || task.status === status) return;

  try {
    await patchTask(id, { status });
    await refresh();
  } catch (error) {
    toast(`Could not move ${id}: ${describe(error)}`);
    await refreshQuietly(); // fall back to whatever the server actually has
  }
}

/** Files a card from a column's composer. */
export async function quickAdd(title: string, status: Status): Promise<void> {
  await createTask({ title, status });
  await refresh();
}

async function loadServerStatus(): Promise<void> {
  try {
    const status = await fetchStatus();
    taskDir.value = status.task_dir;
    version.value = status.version;
  } catch (error) {
    console.error("status failed", error);
  }
}

/**
 * Reads the project's vocabularies once at start-up. They are read from the
 * config rather than hard-coded here so a project's own colours, names and
 * priority levels are what the board draws and offers.
 *
 * Failing is survivable: labels then render the way an unconfigured one does,
 * and the priority selects keep FALLBACK_PRIORITIES.
 */
async function loadProjectConfig(): Promise<void> {
  try {
    const config = await fetchConfig();
    labels.value = config.labels ?? {};
    if (config.priorities?.length) priorities.value = config.priorities;
  } catch (error) {
    console.error("config failed", error);
  }
}

/**
 * Loads what the board needs and starts polling, so work an agent does on the
 * CLI shows up without a reload.
 */
export async function start(): Promise<void> {
  await Promise.all([loadServerStatus(), loadProjectConfig()]);

  try {
    await refresh();
  } catch (error) {
    toast(`Could not load tasks: ${describe(error)}`);
  }

  setInterval(() => {
    if (busy.value) return;
    void refreshQuietly();
  }, POLL_INTERVAL_MS);
}

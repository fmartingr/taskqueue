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

import { computed, reactive, ref, watch } from "vue";

import { createTask, describe, fetchConfig, fetchStatus, fetchTasks, patchTask } from "./api";
import { connectEvents } from "./events";
import {
  indexTasks,
  visibleTasks,
  FALLBACK_COLUMNS,
  type ColumnSet,
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
/** The project's board, left to right, from GET /api/config. */
export const columns = ref<ColumnSet>(FALLBACK_COLUMNS);
export const taskDir = ref("");
export const version = ref("");

/** False until the first listing lands, so the footer can say so. */
export const loaded = ref(false);

/**
 * Whether the event stream is up: null until the first attempt has resolved
 * either way.
 *
 * The three states matter. `false` makes the footer say the board is polling,
 * and a plain boolean starting at false says it on every single load, for the
 * moment between the first listing and the stream opening.
 */
export const streaming = ref<boolean | null>(null);

/** Set when a refresh failed, and cleared when one succeeds. */
export const stale = ref(false);

export const index = computed(() => indexTasks(tasks.value));
export const visible = computed(() => visibleTasks(tasks.value, filters, columns.value));

/** The footer. It says "Loading…" until the first listing lands, so the board
 *  does not open by announcing "0 tasks" for a queue it has not read yet. */
export const statusLine = computed(() => {
  if (!loaded.value) return "Loading…";
  const total = tasks.value.length;
  const shown = visible.value.length;
  const counts = shown === total ? `${total} tasks` : `${shown} of ${total} tasks`;
  // Nothing is said while the stream is up, which is the ordinary case, and
  // nothing while it is still connecting. The word appears only once a
  // connection has actually failed, when updates really have got slower.
  const link = streaming.value === false ? "polling" : "";
  return [counts, taskDir.value, version.value && `tq ${version.value}`, link]
    .filter(Boolean)
    .join(" · ");
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

/**
 * Rises with every refresh started, so a response that resolves after a newer
 * one can be recognised and dropped.
 *
 * The poll made this vanishingly unlikely — three seconds is a long time for
 * two requests to overtake each other. The stream can ask twice in a second,
 * and an older listing landing last would be held by the payload comparison
 * below until something else changed.
 */
let issued = 0;

export async function refresh(): Promise<void> {
  const ticket = ++issued;
  const fetched = await fetchTasks();
  if (ticket !== issued) return; // a newer refresh is already in flight

  const payload = JSON.stringify(fetched);
  loaded.value = true;
  stale.value = false;
  if (payload === lastPayload) return;
  lastPayload = payload;
  tasks.value = fetched;
}

/** refreshQuietly keeps polling errors out of the way of the toast stack. */
export async function refreshQuietly(): Promise<void> {
  try {
    await refresh();
  } catch (error) {
    // Remembered, not just logged: the stream only speaks when the queue
    // changes, so nothing would ask again until it did, and the board would sit
    // on a stale listing indefinitely. This is what lets the poll pick it up.
    stale.value = true;
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
    if (config.columns?.length) columns.value = config.columns;
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

  listen();

  // The fallback. It does nothing while the stream is up, which is why it can
  // stay at three seconds: it is what covers a server too old to have
  // /api/events, and the gap while a dropped stream is being reconnected.
  setInterval(() => {
    if (busy.value) return;
    // The stream covers the ordinary case, so the poll stands aside for it —
    // except after a refresh that failed, which nothing else would ever retry.
    if (streaming.value === true && !stale.value) return;
    void refreshQuietly();
  }, POLL_INTERVAL_MS);
}

/**
 * A refresh the stream asked for while the user was in the middle of something.
 *
 * The poll drops these — it simply skips its turn — because another one is
 * three seconds away. The stream has no next turn: it speaks when something
 * changed, so a signal dropped here is a change the board never hears about.
 * Holding it until the hand comes off is the difference.
 */
let queued = false;

function listen(): void {
  connectEvents({
    onTasks() {
      if (busy.value) {
        queued = true;
        return;
      }
      void refreshQuietly();
    },
    onScanFailed(message) {
      toast(`The server cannot read the queue: ${message}`);
    },
    onConnected(connected) {
      streaming.value = connected;
    },
  });

  watch(busy, (isBusy) => {
    if (isBusy || !queued) return;
    queued = false;
    void refreshQuietly();
  });
}

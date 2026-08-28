/**
 * The board's shared state.
 *
 * Everything a component cannot own alone lives here: the tasks, the filter
 * bar, the project's vocabularies, and the flags that say the user is in the
 * middle of something. Anything a single component *can* own — a composer's
 * draft, a note being edited, the fields of an open dialog — deliberately does
 * not, because that is the state the old imperative board had to keep in a
 * global just to stop a re-render from taking it away.
 *
 * The rules themselves are still in board.ts and notes.ts, which know nothing
 * about Vue and keep their own unit tests.
 */

import { computed, reactive, ref, watch } from "vue";

import {
  ApiError,
  createTask,
  describe,
  fetchConfig,
  fetchStatus,
  fetchTask,
  fetchTasks,
  patchTask,
  type DuplicatedID,
  type UnreadableFile,
} from "./api";
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

/** The one status code the board reads rather than just repeats: it is how a
 *  task that has left the queue is told from a server that went quiet. */
const NOT_FOUND = 404;

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
/**
 * The filter bar and the search bar, which are one thing: the query line parses
 * into this and formats back out of it, so typing `priority=urgent` moves the
 * select and moving the select rewrites the query (TQ-0068).
 */
export const filters = reactive<Filters>({
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
  text: "",
});

/** The project's label vocabulary, from GET /api/config. */
export const labels = ref<LabelSet>({});
/** The project's priority vocabulary, in rank order, from GET /api/config. */
export const priorities = ref<PrioritySet>(FALLBACK_PRIORITIES);
/** The project's board, left to right, from GET /api/config. */
export const columns = ref<ColumnSet>(FALLBACK_COLUMNS);
export const taskDir = ref("");
export const version = ref("");

/**
 * The task files the server could not read, from GET /api/status.
 *
 * The listing itself cannot carry this: /api/tasks is an array of tasks, and a
 * skipped file is not one. It is a real problem — a merge conflict, a
 * hand-edited key — so the board says so twice over: a toast when it appears,
 * and a count in the footer for as long as it lasts (TQ-0011).
 */
export const unreadable = ref<UnreadableFile[]>([]);

/**
 * The IDs the server found more than one task file for, from GET /api/status.
 *
 * Neither copy is in the listing, so this is a card the board cannot draw, and
 * it is said the same way a skipped file is: a toast when it appears, and a
 * count in the footer for as long as it lasts (TQ-0040).
 */
export const duplicated = ref<DuplicatedID[]>([]);

/**
 * Whether the server's last scan could be squared with the task directory,
 * from GET /api/status.
 *
 * A queue being written to while it is read can come back a task short, and
 * the store says so rather than passing the result off as the whole queue. The
 * board says it the same way it says a file was skipped: a toast when it
 * appears, and a word in the footer while it lasts (TQ-0012).
 */
export const incomplete = ref(false);

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
  // A toast is gone in six seconds and a broken file is not: whoever opens the
  // board an hour later still has to be told the count is short.
  const broken = unreadable.value.length;
  const skipped = broken ? `${broken} file${broken === 1 ? "" : "s"} could not be read` : "";
  // And for an ID the server could not tell apart: the card is not on the
  // board, and nothing else on the page would say why.
  const doubled = duplicated.value.length;
  const claimed = doubled ? `${doubled} id${doubled === 1 ? "" : "s"} claimed by more than one file` : "";
  // Same reasoning for a listing the server could not square with the
  // directory: the count above may not be the queue, and the toast that said
  // so is long gone.
  const unsquared = incomplete.value ? "the queue was changing as it was read" : "";
  return [counts, skipped, claimed, unsquared, taskDir.value, version.value && `tq ${version.value}`, link]
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

/**
 * The open task as the board last saw it, and whether the listing still has it.
 *
 * Held rather than derived straight from the listing, which is what the dialog
 * used to be found in. A refresh that no longer carries the task — it was
 * deleted, or the scan came back a task short (TQ-0012) — would unmount the
 * dialog and take an unsaved edit with it, and that is the one thing a refresh
 * under an open dialog must never do (TQ-0084). So the dialog stays on the last
 * version the board saw, and `openTaskMissing` is what lets it say the task is
 * gone rather than letting Save fail with a 404 — but only once the task's own
 * endpoint has confirmed it, since a listing can leave a task out for reasons
 * that have nothing to do with it (see confirmMissing).
 */
export const openTask = ref<Task | null>(null);
export const openTaskMissing = ref(false);

/** Rises with every listing that changes the question, so a confirmation that
 *  resolves after the board has moved on can be recognised and dropped. */
let asked = 0;

watch(
  [tasks, openTaskID],
  () => {
    const id = openTaskID.value;
    asked++;
    if (id === null) {
      openTask.value = null;
      openTaskMissing.value = false;
      return;
    }
    const found = tasks.value.find((task) => task.id === id);
    if (found) {
      openTask.value = found;
      openTaskMissing.value = false;
      return;
    }
    void confirmMissing(id, asked);
  },
  // Synchronously, so the dialog mounts on the click that opened it rather than
  // a tick later — the render that reads `openTask` is the very next thing to
  // happen after `openTaskID` is set.
  { immediate: true, flush: "sync" },
);

/**
 * Asks the task's own endpoint whether it has really left the queue.
 *
 * A listing without it is not proof of anything. The store returns a short one
 * rather than pretend it read the whole directory (TQ-0012), leaves out a file
 * it could not parse (TQ-0011) and both copies of an ID two files claim
 * (TQ-0040) — and an agent writing to the open task passes through all three.
 * A lookup by ID sees past every one of them, so the listing raises the
 * question and this answers it, and the dialog says nothing in between.
 *
 * Only a plain "no such task" counts. A server that could not be reached at all
 * says nothing about whether the task is still there, and claiming it was
 * deleted over a dropped connection is the alarming version of being stale.
 */
async function confirmMissing(id: string, ticket: number): Promise<void> {
  try {
    await fetchTask(id);
    if (ticket === asked) openTaskMissing.value = false;
  } catch (error) {
    if (ticket === asked) {
      openTaskMissing.value = error instanceof ApiError && error.status === NOT_FOUND;
    }
  }
}

/**
 * Whether a refresh has to wait.
 *
 * A drag and nothing else. Everything else the user can be in the middle of
 * happens in a layer *above* the board — a dialog over a backdrop, a composer
 * whose textarea a re-render leaves mounted — and nothing moves under their
 * hand when a card appears behind one. A native drag genuinely cannot survive a
 * re-render: the element under the pointer would be replaced mid-gesture and
 * the drop would land nowhere.
 *
 * It used to cover the composer and both dialogs too, which froze the board for
 * as long as a modal was open — minutes, while an agent worked — and kept the
 * open task from ever hearing that it had changed (TQ-0084).
 */
export const busy = computed(() => dragging.value !== null);

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

/**
 * Rises with every status request started, so an older response that resolves
 * last can be dropped — the same guard `refresh` keeps, and for the same
 * reason: two change signals half a second apart put two of these in flight,
 * and the stale one would leave the footer complaining about a file that has
 * since been fixed.
 */
let statusIssued = 0;

async function loadServerStatus(): Promise<void> {
  const ticket = ++statusIssued;
  try {
    const status = await fetchStatus();
    if (ticket !== statusIssued) return; // a newer request is already in flight
    taskDir.value = status.task_dir;
    version.value = status.version;
    reportMissing(status.unreadable ?? [], status.duplicated ?? []);
    reportIncomplete(status.incomplete ?? false);
  } catch (error) {
    console.error("status failed", error);
  }
}

/** What has already been complained about, so one problem is one toast rather
 *  than one per refresh — and one that is fixed and comes back is a new toast. */
let complainedAbout = new Set<string>();

/** How many problems are named one by one before the rest are summarised.
 *  A conflicted merge breaks several at once, and a toast per file would cover
 *  the board it is complaining about. */
const NAMED_IN_TOASTS = 3;

/**
 * Records what the server could not put in the listing — the files it had to
 * skip (TQ-0011) and the IDs more than one file claims (TQ-0040) — and toasts
 * about whatever is new.
 *
 * Both are a task missing from the board, so the count that follows them in
 * the footer would otherwise just look wrong, and both are counted together
 * here so that a directory in a bad way is one burst of toasts rather than
 * two.
 */
function reportMissing(files: UnreadableFile[], doubled: DuplicatedID[]): void {
  unreadable.value = files;
  duplicated.value = doubled;
  const seen = new Set([
    ...files.map((file) => `${file.file}: ${file.reason}`),
    ...doubled.map((id) => id.reason),
  ]);
  const fresh = [...seen].filter((complaint) => !complainedAbout.has(complaint));
  complainedAbout = seen;

  for (const complaint of fresh.slice(0, NAMED_IN_TOASTS)) {
    toast(`Not on the board — ${complaint}`);
  }
  const rest = fresh.length - NAMED_IN_TOASTS;
  if (rest > 0) toast(`…and ${rest} more problem${rest === 1 ? "" : "s"} the board could not show`);
}

/** Whether the last scan was already known to be unsquared, so a queue being
 *  written to is one toast rather than one per refresh. */
let complainedAboutIncomplete = false;

/**
 * Records that the server could not square its scan with the directory, and
 * toasts when that is news. The footer keeps saying it for as long as it
 * lasts; a listing that settles clears both, because a retitle finishing is
 * not something to keep complaining about.
 */
function reportIncomplete(unsquared: boolean): void {
  incomplete.value = unsquared;
  if (unsquared && !complainedAboutIncomplete) {
    toast("The queue was changing as it was read — this board may be a task short");
  }
  complainedAboutIncomplete = unsquared;
}

/** Serialized last configuration, so a refetch that changed nothing re-renders
 *  nothing — the stream sends one on every connection, not only on a change. */
let lastConfig = "";

/** The last complaint about the config, so one broken file is one toast rather
 *  than one per keystroke of whoever is editing it. */
let lastConfigError = "";

/**
 * Reads the project's vocabularies. They are read from the config rather than
 * hard-coded here so a project's own colours, names and priority levels are
 * what the board draws and offers.
 *
 * Called again whenever `.taskqueue.yaml` changes (TQ-0034), which is why
 * failing has to be survivable in both directions. An editor leaves the file
 * unparsable for a moment while it saves, and the board must not blank its
 * labels for it: the last configuration that parsed stays in place, a toast
 * says what is wrong, and the next event puts it right. At start-up there is no
 * last good one, and the fallbacks stand in — an unconfigured label renders
 * neutral, and the selects keep FALLBACK_PRIORITIES.
 */
async function loadProjectConfig(): Promise<boolean> {
  let config;
  try {
    config = await fetchConfig();
  } catch (error) {
    const message = describe(error);
    if (message !== lastConfigError) {
      lastConfigError = message;
      toast(`Could not read the project configuration: ${message}`);
    }
    return false;
  }

  lastConfigError = "";
  const payload = JSON.stringify(config);
  if (payload === lastConfig) return false;
  lastConfig = payload;

  labels.value = config.labels ?? {};
  // Only when the server has something to offer: an empty set would leave the
  // board with no columns to draw and no priorities to file a task with.
  if (config.priorities?.length) priorities.value = config.priorities;
  if (config.columns?.length) columns.value = config.columns;
  return true;
}

/** What a refresh was asked for: either signal, or both at once. */
interface Signals {
  tasks: boolean;
  config: boolean;
}

/**
 * Answers whatever asked, in the order that makes sense: the configuration
 * first, because the listing depends on it. The server resolves a task's status
 * against the project's columns and sorts by its priorities before serving it,
 * so a board whose columns changed has its tasks in different places — and a
 * card whose column was removed is shown in the first one. That is why a config
 * that really changed refetches the listing even when nothing asked it to.
 *
 * "Really changed" is the other half. The stream opens with a config frame
 * whether or not the file moved, so refetching the listing on every one would
 * double what a reconnect costs.
 */
async function applySignals(signals: Signals): Promise<void> {
  const changed = signals.config ? await loadProjectConfig() : false;
  if (!signals.tasks && !changed) return;
  await refreshQuietly();
  // The listing is an array of tasks and has nowhere to say that a file was
  // skipped, so the files the server could not read come from /api/status —
  // asked for again here because the change that just arrived is exactly how a
  // file becomes unreadable, or stops being (TQ-0011). It is one extra request
  // per change, not per tick: nothing asks while the queue is sitting still.
  await loadServerStatus();
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
    // The configuration is in the poll because nothing else would notice a
    // marker that changed while the stream is down, and what the fallback
    // promises is that the board is never silently stale — about its columns as
    // much as its cards. It costs a second request only while there is no
    // stream for it to stand aside for.
    void applySignals({ tasks: true, config: true });
  }, POLL_INTERVAL_MS);
}

/**
 * What the stream asked for while a card was in the air.
 *
 * The poll drops these — it simply skips its turn — because another one is
 * three seconds away. The stream has no next turn: it speaks when something
 * changed, so a signal dropped here is a change the board never hears about.
 * Holding it until the hand comes off is the difference.
 *
 * A flag each rather than a queue, because a signal is not a payload: ten
 * events between two renders are one refetch. Two flags rather than one because
 * the two signals are answered differently, and a config change that arrived
 * behind a task change must not be answered as a task change.
 */
const queued: Signals = { tasks: false, config: false };

function listen(): void {
  connectEvents({
    onTasks() {
      if (busy.value) {
        queued.tasks = true;
        return;
      }
      void applySignals({ tasks: true, config: false });
    },
    onConfig() {
      if (busy.value) {
        queued.config = true;
        return;
      }
      void applySignals({ tasks: false, config: true });
    },
    onScanFailed(message) {
      toast(`The server cannot read the queue: ${message}`);
    },
    onConnected(connected) {
      streaming.value = connected;
    },
  });

  watch(busy, (isBusy) => {
    if (isBusy) return;
    const held = { ...queued };
    queued.tasks = false;
    queued.config = false;
    if (held.tasks || held.config) void applySignals(held);
  });
}

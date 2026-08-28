<script setup lang="ts">
/**
 * The task dialog: every editable field of one task, plus its notes panel.
 *
 * The board stays live while it is open, so the task under it can move: an
 * agent claims it, appends a note, rewrites the body. The dialog follows what
 * the file holds for every field the user is not in the middle of, and never
 * overwrites one they are — see `adopt` and ../adopt.ts (TQ-0084).
 *
 * Saving is the other half. A body snapshot taken at open time and PATCHed
 * wholesale erased every note the CLI wrote in between (TQ-0010), so a save
 * re-reads the task first and merges (see buildPatch and mergeBody): the file
 * decides which notes exist, the dialog decides only the wording of the ones it
 * holds, and the content half above them belongs to whichever side actually
 * edited it — the snapshot is never written back over an edit the dialog did
 * not make (TQ-0079).
 *
 * The two meet at `baseline`, the body the merge is defined against. Every
 * adoption moves it, or the next save would read the dialog's own adoption as
 * somebody else's edit.
 */
import { computed, onBeforeUnmount, onMounted, ref, watch, type Ref } from "vue";

import { adoptBody, adoptField } from "../adopt";
import { addNote, describe, fetchTask, patchTask, type TaskInput } from "../api";
import { defaultPriority, pendingDependencies, priorityOptions, type Task } from "../board";
import { formatTime, splitList } from "../format";
import { mergeBody, splitBody, type Note, type SplitBody } from "../notes";
import { columns, index, openTaskMissing, priorities, refresh, toast } from "../state";
import NotesPanel from "./NotesPanel.vue";

const props = defineProps<{ task: Task }>();
const emit = defineEmits<{ close: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);

const opened = splitBody(props.task.body ?? "");
/** The body exactly as the dialog last took it from the file, for the merge on
 *  save — moved by every adoption, so the two work from one snapshot. */
const baseline = ref<SplitBody>({
  content: opened.content,
  notes: opened.notes.map((note) => ({ ...note })),
});

const title = ref(props.task.title);
const status = ref<string>(props.task.status);
const priority = ref(props.task.priority || defaultPriority(priorities.value));
const assignee = ref(props.task.assignee ?? "");
const labelList = ref((props.task.labels ?? []).join(", "));
const dependsOn = ref((props.task.depends_on ?? []).join(", "));
const content = ref(opened.content);
const notes = ref<Note[]>(opened.notes.map((note) => ({ ...note })));
const noteDraft = ref("");

/**
 * The priorities the select has to keep offering: the one the task was opened
 * with, and any a live adoption has since put in the field. The dialog writes
 * every field back on save, so a value with no option of its own is a priority
 * the next save would silently erase. They accumulate rather than tracking the
 * current value, so switching away from one does not take it off the list.
 */
const kept = ref<string[]>([priority.value]);
watch(priority, (value) => {
  if (!kept.value.includes(value)) kept.value = [...kept.value, value];
});
const priorityChoices = computed(() => priorityOptions(priorities.value, kept.value));

const pending = computed(() => pendingDependencies(props.task, index.value, columns.value));
const timestamps = computed(
  () => `created ${formatTime(props.task.created)} · updated ${formatTime(props.task.updated)}`,
);

onMounted(() => dialog.value?.showModal());

// ── Following the file ──────────────────────────────────────────

/**
 * One plain field: what the notice calls it, the control the user would be
 * typing in, the ref behind it, and how to read it off a task.
 *
 * `taken` is the value the field was last given from the file — what it opened
 * with, or the last thing it adopted. `local` differing from it is exactly "the
 * user typed here", which is why no input needs a dirty flag kept in step.
 */
interface Field {
  label: string;
  id: string;
  local: Ref<string>;
  from: (task: Task) => string;
  taken: string;
}

function field(label: string, id: string, local: Ref<string>, from: (task: Task) => string): Field {
  return { label, id, local, from, taken: from(props.task) };
}

const fields: Field[] = [
  field("Title", "task-title", title, (task) => task.title),
  field("Status", "task-status", status, (task) => task.status),
  field("Priority", "task-priority", priority, (task) => task.priority || defaultPriority(priorities.value)),
  field("Assignee", "task-assignee", assignee, (task) => task.assignee ?? ""),
  field("Labels", "task-labels", labelList, (task) => (task.labels ?? []).join(", ")),
  field("Depends on", "task-depends-on", dependsOn, (task) => (task.depends_on ?? []).join(", ")),
];

/** The fields the file changed under an edit in progress. Named rather than
 *  counted: "Title, Labels" is what tells the user where to look. */
const changed = ref<string[]>([]);

/**
 * Records whether the file is still holding something back from this field.
 *
 * It has to go both ways. A user who puts a field back the way they found it
 * frees the dialog to adopt it after all, and a notice still naming that field
 * would be pointing at text nobody is holding any more.
 */
function report(label: string, withheld: boolean): void {
  if (changed.value.includes(label) === withheld) return;
  changed.value = withheld
    ? [...changed.value, label]
    : changed.value.filter((named) => named !== label);
}

/** Whether the caret is in this control right now. */
function focused(id: string): boolean {
  return document.activeElement?.id === id;
}

/**
 * Takes what the file now holds into every field the user is not in the middle
 * of, and says which ones it had to leave alone.
 *
 * `caret` is what makes a deferral: with it, a field the user is merely inside
 * keeps what it is showing until they leave. A write passes false, because by
 * then there is nothing left to protect and everything left to lose — see
 * buildPatch.
 */
function adopt(task: Task, caret = true): void {
  for (const entry of fields) {
    const incoming = entry.from(task);
    switch (adoptField(entry.taken, entry.local.value, incoming, caret && focused(entry.id))) {
      case "take":
        entry.local.value = incoming;
        entry.taken = incoming;
        report(entry.label, false);
        break;
      case "keep":
        report(entry.label, true);
        break;
      case "unchanged":
        // The file agrees with what this field last took, so whatever was being
        // withheld from it no longer is: a notice still naming it would point
        // at text nobody is holding back.
        report(entry.label, false);
        break;
      // "defer" leaves the field exactly as it stands, and says nothing: the
      // pass that runs when the caret leaves is what settles it.
    }
  }

  const adoption = adoptBody(
    baseline.value,
    { content: content.value, notes: notes.value },
    splitBody(task.body ?? ""),
    caret && focused("task-body"),
  );
  content.value = adoption.edited.content;
  notes.value = adoption.edited.notes;
  baseline.value = adoption.baseline;
  // "held" is a body nothing is being withheld from either — the caret is in it,
  // or the file has not moved since the edit began.
  report("Body", adoption.content === "overridden");
}

watch(() => props.task, (task) => adopt(task));

/**
 * The deferral half of the rule: a field is left alone while the caret is in
 * it, so the pass has to run again once the caret has gone, or a change that
 * arrived at the wrong moment would never be shown at all.
 *
 * A turn later, because focusout fires while the focus is still on its way out
 * and document.activeElement would still name the control being left.
 */
let refocus: ReturnType<typeof setTimeout> | undefined;

function onFocusOut(): void {
  clearTimeout(refocus);
  refocus = setTimeout(() => adopt(props.task), 0);
}

onBeforeUnmount(() => clearTimeout(refocus));

/** Every path out of the dialog goes through the element's own close event. */
function dismiss(): void {
  dialog.value?.close();
}

// ── Writing ─────────────────────────────────────────────────────

/**
 * The whole dialog as one patch, against what the file holds *now* rather than
 * what the dialog last read.
 *
 * `body` is left out of the patch whenever the body needs no writing, so a save
 * that changed only Priority cannot touch content someone else wrote. Null is
 * the one case with no honest patch at all — both sides edited the content half
 * — and every caller answers it by writing nothing and saying so.
 *
 * The patch is a save of every field, so a deferral has to be settled before it
 * is built. A field the caret is in keeps showing what it had while the user is
 * reading it, and Enter in a text input submits the form without ever moving
 * the focus — so without this a change the dialog quietly deferred would be
 * written straight back out of the stale field, saying nothing to anyone.
 *
 * And what it is built from is read *before* the round trip, not after. An
 * adoption landing while the fetch is in flight moves the baseline and the
 * fields to a file newer than the `current` below, and a merge across those two
 * would write the pre-adoption content half back over the newer one — TQ-0079's
 * bug again, on the path this ticket opened.
 */
async function buildPatch(): Promise<Partial<TaskInput> | null> {
  adopt(props.task, false);

  // By reference, and safely so: an adoption replaces these refs rather than
  // writing into what they held (see adoptBody), so what is captured here is
  // still the pair mergeNotes needs, lined up index for index. A merge that
  // ever started mutating in place would break this silently.
  const opened = baseline.value;
  const edited = { content: content.value, notes: notes.value };
  const patch: Partial<TaskInput> = {
    title: title.value,
    status: status.value,
    priority: priority.value,
    assignee: assignee.value,
    labels: splitList(labelList.value),
    depends_on: splitList(dependsOn.value),
  };

  const current = splitBody((await fetchTask(props.task.id)).body ?? "");
  const merged = mergeBody(opened, edited, current);
  if (merged.outcome === "conflict") return null;
  if (merged.outcome === "write") patch.body = merged.body;
  return patch;
}

/**
 * Says why nothing was written, and leaves every field exactly as the user left
 * it, typing included: the point of refusing is that their text is the half
 * that would have been lost, so it is the last thing to throw away.
 *
 * Deliberately without a refresh. A task withheld from a scan is exactly what a
 * file being written twice looks like (TQ-0012, TQ-0040), which is the
 * situation a refusal is already in, and there is nothing here a refetch would
 * put right: the dialog already follows the file for every field it can.
 *
 * "since this dialog read it" rather than "while it was open": the body it
 * holds may have been adopted a moment ago rather than read at open time.
 */
function refuse(): void {
  toast(
    `Not saved: the body of ${props.task.id} changed on disk since this dialog read it. ` +
      `Your text is still here — copy it, then close and reopen the task.`,
  );
}

/**
 * Starts the dialog over from what the server just wrote back, baseline and all.
 *
 * Anything that writes has to do this before it can fail again: a baseline left
 * behind a write the dialog itself made reads as somebody else's edit, and
 * every later save would refuse itself over it. The plain fields are marked
 * taken from what they hold rather than from the response, because the patch
 * that returned it is what put those values on disk — so nothing is left
 * looking edited, and the notice about the file changing underneath is spent.
 */
function rebase(task: Task): void {
  const written = splitBody(task.body ?? "");
  content.value = written.content;
  notes.value = written.notes.map((note) => ({ ...note }));
  baseline.value = { content: written.content, notes: written.notes.map((note) => ({ ...note })) };
  for (const entry of fields) entry.taken = entry.local.value;
  changed.value = [];
}

async function save(): Promise<void> {
  try {
    const patch = await buildPatch();
    if (patch === null) {
      refuse();
      return;
    }
    await patchTask(props.task.id, patch);
    dismiss();
    await refresh();
  } catch (error) {
    toast(`Could not update ${props.task.id}: ${describe(error)}`);
  }
}

async function append(): Promise<void> {
  // Trimmed to answer "is there a note here at all", and not otherwise: the
  // indent a pasted block shares is what makes it one block, and the store
  // normalises the text anyway (TQ-0054).
  const text = noteDraft.value;
  if (text.trim() === "") return;

  try {
    // Pending edits are saved first: appending a note rewrites the body, and
    // silently dropping what the user typed would be worse than an extra write.
    const patch = await buildPatch();
    if (patch === null) {
      refuse();
      return;
    }
    // Rebased twice, because the note is a second write and can fail on its
    // own: leaving the dialog on the pre-patch baseline until both had landed
    // is what would strand it, refusing every later save over its own edit.
    rebase(await patchTask(props.task.id, patch));
    rebase(await addNote(props.task.id, text));
    noteDraft.value = "";

    await refresh();
    toast(`Note added to ${props.task.id}`, "info");
  } catch (error) {
    toast(`Could not add a note to ${props.task.id}: ${describe(error)}`);
  }
}

function editNote(note: Note, text: string): void {
  note.text = text;
}
</script>

<template>
  <dialog id="task-dialog" ref="dialog" class="dialog" @close="emit('close')">
    <form id="task-form" method="dialog" @submit.prevent="save" @focusout="onFocusOut">
      <header class="dialog-header">
        <span id="task-dialog-id" class="task-id">{{ task.id }}</span>
        <button
          type="button"
          class="ghost close"
          data-close="task-dialog"
          aria-label="Close"
          @click="dismiss"
        >
          ✕
        </button>
      </header>

      <p id="task-gone" class="dialog-note gone" :hidden="!openTaskMissing">
        {{ task.id }} is no longer in the queue — it may have been deleted. Copy anything you
        still need here: a save has nothing left to write to.
      </p>

      <p id="task-changed" class="dialog-note" :hidden="changed.length === 0">
        Changed on disk while you were editing: {{ changed.join(", ") }}. Your text was kept.
      </p>

      <label>
        Title
        <input id="task-title" v-model="title" name="title" type="text" required />
      </label>

      <div class="grid">
        <label>
          Status
          <select id="task-status" v-model="status" name="status">
            <option v-for="column in columns" :key="column.name" :value="column.name">{{ column.display_name }}</option>
          </select>
        </label>
        <label>
          Priority
          <select id="task-priority" v-model="priority" name="priority">
            <option
              v-for="option in priorityChoices"
              :key="option.name"
              :value="option.name"
              :title="
                option.configured ? option.name : `${option.name} — not in the project's priority set`
              "
            >{{ option.display }}</option>
          </select>
        </label>
        <label>
          Assignee
          <input id="task-assignee" v-model="assignee" name="assignee" type="text" autocomplete="off" />
        </label>
        <label>
          Labels
          <input
            id="task-labels"
            v-model="labelList"
            name="labels"
            type="text"
            placeholder="backend, auth"
            autocomplete="off"
          />
        </label>
        <label>
          Depends on
          <input
            id="task-depends-on"
            v-model="dependsOn"
            name="depends_on"
            type="text"
            placeholder="TQ-0002, TQ-0003"
            autocomplete="off"
          />
        </label>
      </div>

      <p id="task-blocked" class="blocked-note" :hidden="pending.length === 0">{{
        pending.length === 0 ? "" : `Blocked by ${pending.join(", ")}`
      }}</p>

      <label>
        Body (Markdown)
        <textarea id="task-body" v-model="content" name="body" rows="10" spellcheck="false"></textarea>
      </label>

      <NotesPanel
        v-model:draft="noteDraft"
        :notes="notes"
        @edit="editNote"
        @append="append"
      />

      <footer class="dialog-footer">
        <span id="task-timestamps" class="timestamps">{{ timestamps }}</span>
        <span class="spacer"></span>
        <button type="button" class="ghost" data-close="task-dialog" @click="dismiss">Cancel</button>
        <button type="submit" class="primary">Save</button>
      </footer>
    </form>
  </dialog>
</template>

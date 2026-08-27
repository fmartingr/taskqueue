<script setup lang="ts">
/**
 * The task dialog: every editable field of one task, plus its notes panel.
 *
 * Its fields are read from the task once, when it opens, and are never written
 * back from the props again — an unsaved edit belongs to the user until they
 * save or cancel it, and the poll stands down for as long as the dialog is
 * open for the same reason.
 *
 * That is also what made TQ-0010 possible: a body snapshot taken at open time
 * and PATCHed wholesale erased every note the CLI wrote in between. Saving
 * therefore re-reads the task first and merges (see buildPatch and mergeBody):
 * the file decides which notes exist, the dialog decides only the wording of
 * the ones it was opened with, and the content half above them belongs to
 * whichever side actually edited it — the snapshot is never written back over
 * an edit the dialog did not make (TQ-0079).
 */
import { computed, onMounted, ref } from "vue";

import { addNote, describe, fetchTask, patchTask, type TaskInput } from "../api";
import { defaultPriority, pendingDependencies, priorityOptions, type Task } from "../board";
import { formatTime, splitList } from "../format";
import { mergeBody, splitBody, type Note, type SplitBody } from "../notes";
import { columns, index, priorities, refresh, toast } from "../state";
import NotesPanel from "./NotesPanel.vue";

const props = defineProps<{ task: Task }>();
const emit = defineEmits<{ close: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);

const opened = splitBody(props.task.body ?? "");
/** The body exactly as the dialog opened it, for the merge on save. */
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

// The task's own priority is offered even when the project has dropped it: the
// dialog writes every field back on save, so an option missing here is a
// priority the next save would silently erase. The extra is fixed at open time
// so switching away from it does not take it off the list.
const stale = [priority.value];
const priorityChoices = computed(() => priorityOptions(priorities.value, stale));

const pending = pendingDependencies(props.task, index.value, columns.value);
const timestamps = `created ${formatTime(props.task.created)} · updated ${formatTime(props.task.updated)}`;

onMounted(() => dialog.value?.showModal());

/** Every path out of the dialog goes through the element's own close event. */
function dismiss(): void {
  dialog.value?.close();
}

/**
 * The whole dialog as one patch, against what the file holds *now* rather than
 * what it held when the dialog opened.
 *
 * `body` is left out of the patch whenever the body needs no writing, so a save
 * that changed only Priority cannot touch content someone else wrote. Null is
 * the one case with no honest patch at all — both sides edited the content half
 * — and every caller answers it by writing nothing and saying so.
 */
async function buildPatch(): Promise<Partial<TaskInput> | null> {
  const current = splitBody((await fetchTask(props.task.id)).body ?? "");
  const merged = mergeBody(baseline.value, { content: content.value, notes: notes.value }, current);
  if (merged.outcome === "conflict") return null;

  const patch: Partial<TaskInput> = {
    title: title.value,
    status: status.value,
    priority: priority.value,
    assignee: assignee.value,
    labels: splitList(labelList.value),
    depends_on: splitList(dependsOn.value),
  };
  if (merged.outcome === "write") patch.body = merged.body;
  return patch;
}

/**
 * Says why nothing was written, and leaves every field exactly as the user left
 * it, typing included: the point of refusing is that their text is the half
 * that would have been lost, so it is the last thing to throw away.
 *
 * Deliberately without a refresh. The listing is what App.vue finds this dialog
 * in, so replacing it can unmount the component — and a task withheld from a
 * scan is exactly what a file being written twice looks like (TQ-0012,
 * TQ-0040), which is the situation a refusal is already in. Closing the dialog
 * releases the change the stream held while it was open, so reopening starts
 * from the file either way.
 *
 * "since this dialog read it" rather than "while it was open": the body it was
 * opened with comes from the last listing, so the change may have landed just
 * before the click rather than after it.
 */
function refuse(): void {
  toast(
    `Not saved: the body of ${props.task.id} changed on disk since this dialog read it. ` +
      `Your text is still here — copy it, then close and reopen the task.`,
  );
}

/**
 * Starts the body over from what the server just wrote back, baseline and all.
 *
 * Anything that writes has to do this before it can fail again: a baseline left
 * behind a write the dialog itself made reads as somebody else's edit, and
 * every later save would refuse itself over it.
 */
function rebase(task: Task): void {
  const written = splitBody(task.body ?? "");
  content.value = written.content;
  notes.value = written.notes.map((note) => ({ ...note }));
  baseline.value = { content: written.content, notes: written.notes.map((note) => ({ ...note })) };
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
  const text = noteDraft.value.trim();
  if (text === "") return;

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

function editNote(position: number, text: string): void {
  const note = notes.value[position];
  if (note) note.text = text;
}
</script>

<template>
  <dialog id="task-dialog" ref="dialog" class="dialog" @close="emit('close')">
    <form id="task-form" method="dialog" @submit.prevent="save">
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

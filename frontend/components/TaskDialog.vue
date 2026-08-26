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
 * therefore re-reads the task first and merges (see buildPatch and mergeNotes):
 * the file decides which notes exist, the dialog decides only the wording of
 * the ones it was opened with.
 */
import { computed, onMounted, ref } from "vue";

import { addNote, describe, fetchTask, patchTask, type TaskInput } from "../api";
import { defaultPriority, pendingDependencies, priorityOptions, type Task } from "../board";
import { formatTime, splitList } from "../format";
import { joinBody, mergeNotes, splitBody, type Note } from "../notes";
import { columns, index, priorities, refresh, toast } from "../state";
import NotesPanel from "./NotesPanel.vue";

const props = defineProps<{ task: Task }>();
const emit = defineEmits<{ close: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);

const opened = splitBody(props.task.body ?? "");
/** The notes exactly as the dialog opened them, for the merge on save. */
const baseline = ref<Note[]>(opened.notes.map((note) => ({ ...note })));

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
 */
async function buildPatch(): Promise<Partial<TaskInput>> {
  const current = splitBody((await fetchTask(props.task.id)).body ?? "").notes;
  return {
    title: title.value,
    status: status.value,
    priority: priority.value,
    assignee: assignee.value,
    labels: splitList(labelList.value),
    depends_on: splitList(dependsOn.value),
    body: joinBody({
      content: content.value,
      notes: mergeNotes(baseline.value, notes.value, current),
    }),
  };
}

async function save(): Promise<void> {
  try {
    await patchTask(props.task.id, await buildPatch());
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
    await patchTask(props.task.id, await buildPatch());
    const task = await addNote(props.task.id, text);
    noteDraft.value = "";

    const written = splitBody(task.body ?? "");
    content.value = written.content;
    notes.value = written.notes.map((note) => ({ ...note }));
    baseline.value = written.notes.map((note) => ({ ...note }));

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

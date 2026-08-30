<script setup lang="ts">
/**
 * The task dialog: one task, live, in two columns.
 *
 * TQ-0069 turned this from a form into a card. Two things follow from that, and
 * everything else here is one of the two.
 *
 * **The dialog holds no draft of the task.** Every field is drawn from the task
 * the board last read, so the whole dialog follows the file: an agent claiming
 * the task, appending a note or rewriting the body is visible here while it is
 * open, with no adoption rules and no per-field dirty flags, because there is
 * nothing local for an incoming change to overwrite. The one exception is the
 * editor the user has open, and there is at most one of those.
 *
 * **A field is written when its editor closes.** There is no Save, no Cancel
 * and no whole-task patch: `Store.Patch` takes a partial, so a title that was
 * edited sends a title and nothing else. That is what makes the point above
 * safe — a save of every field is only ever correct if the dialog holds a
 * current copy of every field, which is exactly what it stopped doing.
 *
 * The third rule is the ticket's, and it is a narrowing: a write whose field
 * moved on disk since the editor opened is **refused**, not merged. The dialog
 * says which field, writes nothing at all, and keeps the user's text on screen;
 * what to do about the collision is theirs, in the VCS. The arithmetic is in
 * ../edit.ts, and the one thing it is careful about is that a body is two
 * fields in one string — `tq note` appending a note is not a change to the
 * paragraph somebody is rewriting, and must not refuse it.
 */
import { computed, onMounted, ref } from "vue";

import { addNote, describe, fetchTask, patchTask, type TaskInput } from "../api";
import { defaultPriority, pendingDependencies, priorityOptions, type Task } from "../board";
import { commitContent, commitField, commitNote, type Commit } from "../edit";
import { formatTime } from "../format";
import { splitBody, type Note } from "../notes";
import { columns, index, openTaskMissing, priorities, refresh, toast } from "../state";
import InlineText from "./InlineText.vue";
import LabelChip from "./LabelChip.vue";
import Markdown from "./Markdown.vue";
import NotesPanel from "./NotesPanel.vue";
import TokenField from "./TokenField.vue";

const props = defineProps<{ task: Task }>();
const emit = defineEmits<{ close: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);

/** The body as the file holds it, split into the paragraph half the
 *  description shows and the notes the panel lists. */
const split = computed(() => splitBody(props.task.body ?? ""));

const priority = computed(() => props.task.priority || defaultPriority(priorities.value));
/**
 * The priorities the select has to offer: the project's, plus whatever this
 * task carries. A task filed before the vocabulary changed still holds a value
 * the project has dropped, and a select that cannot show it would say the task
 * is something it is not.
 */
const priorityChoices = computed(() => priorityOptions(priorities.value, [priority.value]));

const pending = computed(() => pendingDependencies(props.task, index.value, columns.value));
const timestamps = computed(
  () => `created ${formatTime(props.task.created)} · updated ${formatTime(props.task.updated)}`,
);

onMounted(() => dialog.value?.showModal());

/** Every path out of the dialog goes through the element's own close event, so
 *  Escape, the ✕ and a click outside end up in the same place. */
function dismiss(): void {
  dialog.value?.close();
}

/** The editors that can be open inside the dialog, for the rule below. */
const EDITORS = ".inline-editor, .note-editor, .token-input";

/**
 * Where the button went down, so a click can be told from a drag that ended
 * somewhere else. A `click` is dispatched at the *common ancestor* of the
 * mousedown and the mouseup, so selecting text in the description and letting
 * go past the edge of the sheet arrives looking exactly like a click on the
 * backdrop — and would throw the dialog away mid-selection.
 */
let pressedOutside = false;

function onMouseDown(event: MouseEvent): void {
  pressedOutside = event.target === dialog.value;
}

/**
 * A click outside the sheet closes the dialog.
 *
 * `<dialog>` paints its backdrop as part of the element rather than as a child,
 * so a click on it arrives with the dialog itself as the target — which is only
 * an unambiguous "outside" because the sheet fills the element and the element
 * carries no padding of its own.
 *
 * The exception is an editor being open, and it is the one thing this must not
 * get wrong. Losing focus writes an edit, and that write can be refused
 * (TQ-0069) — so closing on the same click would take the text a refusal exists
 * to preserve down with it, before the write has even come back. The first
 * click outside therefore lands on the editor and settles it; a second one
 * closes the dialog.
 */
function onClick(event: MouseEvent): void {
  const outside = pressedOutside && event.target === dialog.value;
  pressedOutside = false;
  if (outside && dialog.value?.querySelector(EDITORS) === null) dismiss();
}

// ── Saying the file moved ───────────────────────────────────────

/** A field with an editor open on it: what a refusal calls it, and how to read
 *  it off a task, so the notice can keep asking whether the file still agrees. */
interface Editable {
  what: string;
  of: (task: Task) => string;
}

const TITLE: Editable = { what: "title", of: (task) => task.title };
const ASSIGNEE: Editable = { what: "assignee", of: (task) => task.assignee ?? "" };
const DESCRIPTION: Editable = {
  what: "description",
  of: (task) => splitBody(task.body ?? "").content,
};

/** The editor the user has open, and the value the file held when it opened. */
const editing = ref<Editable | null>(null);
const opened = ref("");

function begin(field: Editable): void {
  editing.value = field;
  opened.value = field.of(props.task);
}

/**
 * The field the file has moved under an open editor, or "".
 *
 * Asked of the task the board holds rather than of the server, so the notice
 * appears while the user is still typing rather than when they finish — which
 * is the whole difference between a warning and a report. The write asks the
 * server again anyway, because the board's copy can be a moment behind and a
 * refusal has to be about the file.
 */
const moved = computed(() => {
  const field = editing.value;
  if (field === null) return "";
  return field.of(props.task) === opened.value ? "" : field.what;
});

// ── Writing ─────────────────────────────────────────────────────

/**
 * Says why nothing was written, and leaves the editor exactly as the user left
 * it, typing included: the point of refusing is that their text is the half
 * that would have been lost, so it is the last thing to throw away.
 *
 * Deliberately without a refresh. A task withheld from a scan is exactly what a
 * file being written twice looks like (TQ-0012, TQ-0040), which is the
 * situation a refusal is already in, and there is nothing here a refetch would
 * put right: every field the user is not in is already following the file.
 */
function refuse(what: string): void {
  toast(
    `Not saved: the ${what} of ${props.task.id} changed on disk while you were editing it. ` +
      `Nothing was written — copy what you need, then press Escape to see the file's version, ` +
      `and check the change in your VCS.`,
  );
}

/**
 * One plain field: read the file, decide, write.
 *
 * `was` is what the editor was opened with, or — for a select, which has no
 * editor to open — what the control was showing when it was used. The two are
 * the same question: somebody acted on a value, and the file either still holds
 * that value or it does not.
 *
 * The read is a round trip per commit, and it is the price of the ticket:
 * without it a refusal would be against the board's last listing rather than
 * against the file, and the whole point is that the file decides.
 */
async function writeField(
  field: Editable,
  was: string,
  edited: string,
  patch: Partial<TaskInput>,
): Promise<boolean> {
  try {
    const commit = commitField(was, edited, field.of(await fetchTask(props.task.id)));
    if (commit === "unchanged") return true;
    if (commit === "conflict") {
      refuse(field.what);
      return false;
    }
    await patchTask(props.task.id, patch);
    await refresh();
    return true;
  } catch (error) {
    toast(`Could not update ${props.task.id}: ${describe(error)}`);
    return false;
  }
}

/** The same, for the two halves of the body, which decide for themselves what
 *  a change on disk means to them. */
async function writeBody(what: string, decide: (current: ReturnType<typeof splitBody>) => Commit) {
  try {
    const commit = decide(splitBody((await fetchTask(props.task.id)).body ?? ""));
    if (commit.outcome === "conflict") {
      refuse(what);
      return false;
    }
    if (commit.outcome === "write") {
      await patchTask(props.task.id, { body: commit.body });
      await refresh();
    }
    return true;
  } catch (error) {
    toast(`Could not update ${props.task.id}: ${describe(error)}`);
    return false;
  }
}

async function saveTitle(title: string): Promise<boolean> {
  if (title.trim() === "") {
    toast(`Not saved: ${props.task.id} needs a title.`);
    return false;
  }
  return writeField(TITLE, opened.value, title, { title });
}

const saveAssignee = (assignee: string) =>
  writeField(ASSIGNEE, opened.value, assignee, { assignee });

const saveContent = (content: string) =>
  writeBody(DESCRIPTION.what, (current) => commitContent(opened.value, content, current));

const saveNote = (note: Note, text: string) =>
  writeBody("note", (current) => commitNote(note, text, current));

/**
 * A select the user has just used.
 *
 * The control is put back by hand when the write does not land, because there
 * is nothing local for Vue to re-render it from: it is bound to the task, the
 * task did not change, and the browser is holding a value the file never took.
 */
async function choose(field: Editable, event: Event, patch: (value: string) => Partial<TaskInput>) {
  const select = event.target as HTMLSelectElement;
  const chosen = select.value;
  const was = field.of(props.task);
  if (!(await writeField(field, was, chosen, patch(chosen)))) select.value = was;
}

const chooseStatus = (event: Event) =>
  choose({ what: "status", of: (task) => task.status }, event, (status) => ({ status }));

const choosePriority = (event: Event) =>
  choose(
    { what: "priority", of: (task) => task.priority || defaultPriority(priorities.value) },
    event,
    (value) => ({ priority: value }),
  );

/**
 * A list, compared as a whole and written as a whole.
 *
 * Serialised for the comparison rather than joined with a separator, because a
 * label is any string the project cares to use and nothing stops one holding
 * whatever separator was picked.
 */
function writeList(
  what: string,
  of: (task: Task) => string[],
  values: string[],
  patch: Partial<TaskInput>,
): Promise<boolean> {
  const field: Editable = { what, of: (task) => JSON.stringify(of(task)) };
  return writeField(field, field.of(props.task), JSON.stringify(values), patch);
}

const saveLabels = (labels: string[]) =>
  writeList("labels", (task) => task.labels ?? [], labels, { labels });

const saveDependencies = (depends_on: string[]) =>
  writeList("dependencies", (task) => task.depends_on ?? [], depends_on, { depends_on });

/** Appending is not an edit of anything, so it collides with nothing: a note
 *  goes on the end of whatever the file holds at the moment it lands. */
async function appendNote(text: string): Promise<boolean> {
  try {
    await addNote(props.task.id, text);
    await refresh();
    toast(`Note added to ${props.task.id}`, "info");
    return true;
  } catch (error) {
    toast(`Could not add a note to ${props.task.id}: ${describe(error)}`);
    return false;
  }
}
</script>

<template>
  <dialog
    id="task-dialog"
    ref="dialog"
    class="dialog task-dialog"
    @close="emit('close')"
    @mousedown="onMouseDown"
    @click="onClick"
  >
    <div class="task-sheet">
      <header class="task-head">
        <div class="task-head-text">
          <span id="task-dialog-id" class="task-id">{{ task.id }}</span>
          <InlineText
            id="task-title"
            heading
            label="Title"
            :value="task.title"
            :commit="saveTitle"
            @open="begin(TITLE)"
            @close="editing = null"
          />
          <select
            id="task-status"
            class="task-status"
            aria-label="Status"
            :value="task.status"
            @change="chooseStatus"
          >
            <option v-for="column in columns" :key="column.name" :value="column.name">
              {{ column.display_name }}
            </option>
          </select>
        </div>
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
        {{ task.id }} is no longer in the queue — it may have been deleted. Copy anything you still
        need here: a write has nothing left to land on.
      </p>

      <p id="task-changed" class="dialog-note moved" :hidden="moved === ''">
        The {{ moved }} of {{ task.id }} changed on disk while you were editing it. Nothing here has
        been written — copy what you need, press Escape to see the file's version, and check the
        change in your VCS.
      </p>

      <div class="task-columns">
        <section class="task-main">
          <h3 class="task-section">Description</h3>
          <InlineText
            id="task-body"
            multiline
            label="Description"
            :value="split.content"
            :commit="saveContent"
            @open="begin(DESCRIPTION)"
            @close="editing = null"
          >
            <Markdown v-if="split.content !== ''" :source="split.content" />
            <p v-else class="task-empty">No description yet — click here to write one.</p>
          </InlineText>

          <h3 class="task-section">Depends on</h3>
          <TokenField
            id="task-depends-on"
            label="a dependency"
            placeholder="TQ-0002"
            :values="task.depends_on ?? []"
            :commit="saveDependencies"
          >
            <template #default="{ value }">
              <span class="token-id">{{ value }}</span>
            </template>
          </TokenField>
          <p id="task-blocked" class="blocked-note" :hidden="pending.length === 0">
            Blocked by {{ pending.join(", ") }}
          </p>

          <NotesPanel :notes="split.notes" :commit="saveNote" :append="appendNote" />
        </section>

        <aside class="task-side">
          <h3 class="task-section">Priority</h3>
          <select
            id="task-priority"
            aria-label="Priority"
            :value="priority"
            @change="choosePriority"
          >
            <option
              v-for="option in priorityChoices"
              :key="option.name"
              :value="option.name"
              :title="
                option.configured ? option.name : `${option.name} — not in the project's priority set`
              "
            >
              {{ option.display }}
            </option>
          </select>

          <h3 class="task-section">Assignee</h3>
          <InlineText
            id="task-assignee"
            label="Assignee"
            placeholder="Unassigned"
            :value="task.assignee ?? ''"
            :commit="saveAssignee"
            @open="begin(ASSIGNEE)"
            @close="editing = null"
          />

          <h3 class="task-section">Labels</h3>
          <TokenField
            id="task-labels"
            label="a label"
            placeholder="backend"
            :values="task.labels ?? []"
            :commit="saveLabels"
          >
            <template #default="{ value }">
              <LabelChip :name="value" />
            </template>
          </TokenField>

          <p id="task-timestamps" class="timestamps">{{ timestamps }}</p>
        </aside>
      </div>
    </div>
  </dialog>
</template>

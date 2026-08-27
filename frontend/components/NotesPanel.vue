<script setup lang="ts">
/**
 * The notes panel of the task dialog: the notes a task carries, each editable
 * in place, and the box that appends a new one.
 *
 * Two of the bugs this component was written to fix live here.
 *
 * TQ-0027: the old panel rebuilt every note when an editor lost focus, so the
 * mousedown on a second note's pencil detached the button before mouseup and no
 * click was ever dispatched — the user had to click twice. The pencil therefore
 * acts on mousedown, and prevents the default focus change so the editor never
 * blurs underneath it. The click that follows is a no-op, which is what keeps
 * the keyboard path (Enter on a focused button, which fires no mousedown) alive.
 *
 * TQ-0019: the "append a note" box sits in the dialog's form, next to a submit
 * button, so Enter in it used to submit the form — saving the dialog and
 * silently discarding what was typed. Enter is handled here, and cancelled, so
 * it can never reach the form.
 */
import { nextTick, ref } from "vue";

import { formatTime } from "../format";
import type { Note } from "../notes";

const props = defineProps<{ notes: Note[]; draft: string }>();
const emit = defineEmits<{
  /** The user finished editing this note. */
  edit: [note: Note, text: string];
  "update:draft": [value: string];
  append: [];
}>();

const list = ref<HTMLUListElement | null>(null);
/**
 * The note being edited, or null.
 *
 * The note itself rather than its position in the list. The list is live while
 * the dialog is open — it takes in whatever the file holds (TQ-0084) — so a
 * position is a name that can come to mean a different note between one
 * keystroke and the next, and the edit would be committed onto that one. The
 * merge hands an edited note back unchanged, so its identity is the one thing
 * an adoption cannot move.
 */
const editing = ref<Note | null>(null);
const editor = ref("");

function beginEdit(note: Note): void {
  if (editing.value === note) return;
  // An edit already in progress is kept, the way losing focus keeps it.
  if (editing.value !== null) commit(editing.value);
  editing.value = note;
  editor.value = note.text;

  void nextTick(() => {
    const area = list.value?.querySelector("textarea.note-editor");
    if (area instanceof HTMLTextAreaElement) {
      area.focus();
      area.setSelectionRange(area.value.length, area.value.length);
    }
  });
}

/**
 * Ends the edit of one note — Enter and losing focus keep it, Escape drops it —
 * and either way the change is written with the dialog's Save, like every other
 * field.
 *
 * Which note matters, and is the other half of the TQ-0027 fix: swapping the
 * textarea back out for its paragraph blurs it, and that blur arrives *after*
 * the edit has already moved to another note. Acting on it would close the
 * editor that was just opened, which is the same "nothing happened" the
 * detached button used to produce.
 */
function finish(keep: boolean, note: Note): void {
  if (editing.value !== note) return;
  editing.value = null;
  if (keep) commit(note);
}

/** Hands an edit back to the dialog, unless it is empty or unchanged. */
function commit(note: Note): void {
  editing.value = null;
  const text = editor.value.trim();
  if (text !== "" && text !== note.text) emit("edit", note, text);
}

/**
 * Enter commits; Shift+Enter is a newline. Written out rather than as
 * `.enter.exact` because that also swallows Ctrl/Alt/Meta+Enter, which the
 * board this replaced treated as an ordinary Enter.
 */
function onEnter(event: KeyboardEvent, commit: () => void): void {
  if (event.shiftKey) return;
  event.preventDefault();
  commit();
}
</script>

<template>
  <section class="notes-section">
    <h3 class="notes-title">Notes</h3>

    <ul id="task-notes" ref="list" class="notes">
      <li v-if="notes.length === 0" class="notes-empty">No notes yet.</li>
      <li v-for="(note, position) in notes" :key="position" class="note">
        <div class="note-head">
          <time class="note-time">{{
            note.timestamp === "" ? "note" : formatTime(note.timestamp)
          }}</time>
          <button
            type="button"
            class="ghost icon"
            title="Edit this note"
            aria-label="Edit this note"
            @mousedown.prevent="beginEdit(note)"
            @click="beginEdit(note)"
          >
            ✎
          </button>
        </div>
        <textarea
          v-if="editing === note"
          v-model="editor"
          class="note-editor"
          rows="2"
          @keydown.enter="onEnter($event, () => finish(true, note))"
          @keydown.esc.prevent="finish(false, note)"
          @blur="finish(true, note)"
        ></textarea>
        <p v-else class="note-text">{{ note.text }}</p>
      </li>
    </ul>

    <div class="note-row">
      <input
        id="task-note"
        type="text"
        placeholder="Append a timestamped note…"
        autocomplete="off"
        :value="draft"
        @input="emit('update:draft', ($event.target as HTMLInputElement).value)"
        @keydown.enter.prevent="emit('append')"
      />
      <button id="task-note-add" type="button" class="ghost" @click="emit('append')">Add note</button>
    </div>
  </section>
</template>

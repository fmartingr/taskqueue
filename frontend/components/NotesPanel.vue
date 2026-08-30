<script setup lang="ts">
/**
 * The notes panel of the task dialog: the box that appends a note, and the
 * notes the file holds, each editable in place.
 *
 * The composer sits above the list rather than below it, which is what TQ-0069
 * drew and what a list that grows downwards wants. Both the composer and an
 * editor write when they close — there is no Save left to carry them — so the
 * two callbacks below are the whole of this component's contract with the
 * dialog: each answers whether the write landed, and a write that did not keeps
 * its text on screen.
 *
 * Four of the bugs this panel was written to fix live here.
 *
 * TQ-0027: the old panel rebuilt every note when an editor lost focus, so the
 * mousedown on a second note's pencil detached the button before mouseup and no
 * click was ever dispatched — the user had to click twice. The pencil therefore
 * acts on mousedown, and prevents the default focus change so the editor never
 * blurs underneath it. The click that follows is a no-op, which is what keeps
 * the keyboard path (Enter on a focused button, which fires no mousedown) alive.
 *
 * TQ-0019: the composer used to sit in the dialog's form, next to a submit
 * button, so Enter in it submitted the form — saving the dialog and silently
 * discarding what was typed. There is no form and no submit button any more,
 * which retires the bug rather than guarding against it; Enter is still handled
 * here because it has to mean "append".
 *
 * TQ-0054: both boxes are textareas — a note keeps the line breaks it is given,
 * and the guide tells agents to paste a command into one. Enter sends and
 * Shift+Enter is a newline, in the composer and in the editor alike.
 *
 * TQ-0084: the list is live and comes off the file, so the note under an open
 * editor is held by what it *said* rather than by the object it was, and by its
 * position least of all — a position is a name that can come to mean a
 * different note between one keystroke and the next. That is also exactly how
 * `commitNote` finds it in the file, so the panel and the write agree on which
 * note is which. A note that stops matching — reworded on disk, or gone
 * altogether — takes its editor out of the list rather than out of the page,
 * because the text in it is the one thing there is no getting back.
 */
import { computed, nextTick, ref, useTemplateRef } from "vue";

import { formatTime } from "../format";
import { noteLines, type Note } from "../notes";

const props = defineProps<{
  notes: Note[];
  /** Rewords one note and answers whether the write landed. */
  commit: (note: Note, text: string) => Promise<boolean>;
  /** Appends a note and answers whether the write landed. */
  append: (text: string) => Promise<boolean>;
}>();

const list = useTemplateRef<HTMLUListElement>("list");
const draft = ref("");
/** The note being edited, as it read when the editor opened. */
const editing = ref<Note | null>(null);
const editor = ref("");

/** True while a write is in flight, so a blur landing on top of an Enter
 *  cannot send the same text twice. */
let writing = false;

/** Whether these are the same note — the file's rule, not the panel's. */
function same(one: Note | null, other: Note): boolean {
  return one !== null && one.timestamp === other.timestamp && one.text === other.text;
}

/** Where in the list the note being edited is, or -1 when it is not there.
 *  By index rather than by a per-note test, so a queue that somehow holds the
 *  same note twice still shows one editor. */
const editingAt = computed(() => props.notes.findIndex((note) => same(editing.value, note)));

/** The editor is open on a note the list no longer has. */
const detached = computed(() => editing.value !== null && editingAt.value === -1);

/**
 * Writes one edit, unless it is empty or unchanged, and answers whether the
 * panel may close the editor.
 *
 * Normalised the way the file will hold it rather than merely trimmed: a
 * pasted block's shared indent is what makes it one block, and trimming would
 * take it off the first line alone (TQ-0054). noteLines is also what makes
 * "unchanged" mean it — the text it returns is what reading the note back out
 * of the file gives, so an edit that only moved whitespace stays a no-op
 * rather than becoming a write that changes nothing but the timestamp.
 */
async function write(note: Note, raw: string): Promise<boolean> {
  if (writing) return false;
  const text = noteLines(raw).join("\n");
  if (text === "" || text === note.text) return true;

  writing = true;
  try {
    return await props.commit(note, text);
  } finally {
    writing = false;
  }
}

/**
 * Opens an editor on a note, writing whatever the last one held first — and
 * staying where it is if that write was refused, because the text it refused
 * is the half worth keeping.
 */
async function beginEdit(note: Note): Promise<void> {
  if (same(editing.value, note)) return;
  if (editing.value !== null && !(await write(editing.value, editor.value))) return;

  editing.value = { ...note };
  editor.value = note.text;

  await nextTick();
  const area = list.value?.querySelector("textarea.note-editor");
  if (area instanceof HTMLTextAreaElement) {
    area.focus();
    area.setSelectionRange(area.value.length, area.value.length);
  }
}

/**
 * Ends the edit of one note — Enter and losing focus write it, Escape drops it.
 *
 * Which note matters, and is the other half of the TQ-0027 fix: swapping the
 * textarea back out for its paragraph blurs it, and that blur arrives *after*
 * the edit has already moved to another note. Acting on it would close the
 * editor that was just opened, which is the same "nothing happened" the
 * detached button used to produce.
 */
async function finish(keep: boolean, note: Note): Promise<void> {
  if (!same(editing.value, note)) return;
  if (!keep) {
    editing.value = null;
    return;
  }
  if (await write(note, editor.value)) editing.value = null;
}

async function send(): Promise<void> {
  // Trimmed to answer "is there a note here at all", and not otherwise: the
  // indent a pasted block shares is what makes it one block, and the store
  // normalises the text anyway (TQ-0054).
  const text = draft.value;
  if (text.trim() === "" || writing) return;

  writing = true;
  try {
    if (await props.append(text)) draft.value = "";
  } finally {
    writing = false;
  }
}

/**
 * Enter commits; Shift+Enter is a newline. Written out rather than as
 * `.enter.exact` because that also swallows Ctrl/Alt/Meta+Enter, which the
 * board this replaced treated as an ordinary Enter.
 */
function onEnter(event: KeyboardEvent, act: () => void): void {
  if (event.shiftKey) return;
  event.preventDefault();
  act();
}
</script>

<template>
  <section class="notes-section">
    <h3 class="task-section">Notes</h3>

    <div class="note-row">
      <textarea
        id="task-note"
        v-model="draft"
        class="note-draft"
        rows="2"
        placeholder="Write a note…"
        @keydown.enter="onEnter($event, send)"
      ></textarea>
      <button id="task-note-add" type="button" class="primary" @click="send">Send</button>
    </div>

    <ul id="task-notes" ref="list" class="notes">
      <li v-if="notes.length === 0 && !detached" class="notes-empty">No notes yet.</li>

      <li v-if="detached && editing !== null" class="note detached">
        <p id="task-note-detached" class="note-moved">
          The note you were editing is no longer in the file as you opened it. Nothing was written —
          copy what you need, then press Escape.
        </p>
        <textarea
          v-model="editor"
          class="note-editor"
          rows="2"
          @keydown.esc.prevent.stop="editing = null"
        ></textarea>
      </li>

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
          v-if="editingAt === position"
          v-model="editor"
          class="note-editor"
          rows="2"
          @keydown.enter="onEnter($event, () => finish(true, note))"
          @keydown.esc.prevent.stop="finish(false, note)"
          @blur="finish(true, note)"
        ></textarea>
        <p v-else class="note-text">{{ note.text }}</p>
      </li>
    </ul>
  </section>
</template>

<script setup lang="ts">
/**
 * A value that reads as text until it is clicked, and is an input while it is
 * being edited: the task's title, its assignee, and — as a textarea, over the
 * rendered Markdown — its description.
 *
 * This is the shape TQ-0069 asked for, and the reason is not decoration. A
 * dialog made of form controls says everything in it is a draft waiting for a
 * Save; a dialog made of text says the file is what is on screen, and that an
 * edit is something you go into deliberately. The writing follows: `commit` is
 * called when the editor closes, and the dialog writes that one field.
 *
 * A refused commit keeps the editor open with what the user typed, and says
 * nothing itself — the dialog owns the message, because it is the only thing
 * that knows what the field is called. Escape is the way out of a refusal that
 * will not clear: it drops the text and puts the file's value back on screen.
 */
import { nextTick, ref, useTemplateRef } from "vue";

const props = withDefaults(
  defineProps<{
    /** What the file holds. Shown whenever nothing is being edited, so the
     *  field follows the file for as long as nobody is in it. */
    value: string;
    /** The id the display carries. The editor carries `<id>-edit`, so a test
     *  and a stylesheet can name the two states apart. */
    id: string;
    /** For the label the editor is announced with. */
    label: string;
    /** A heading rather than a line of text: the title, and only the title. */
    heading?: boolean;
    /** A textarea rather than an input, with the buttons that go with one. */
    multiline?: boolean;
    /** What to show when the file holds nothing here. */
    placeholder?: string;
    /** Writes the value and answers whether it landed. False keeps the editor
     *  open, with the text that was refused still in it. */
    commit: (value: string) => Promise<boolean>;
  }>(),
  { heading: false, multiline: false, placeholder: "" },
);

const emit = defineEmits<{
  /** An editor opened on this field, or closed. The dialog uses the pair to
   *  say which field the file moved under, and nothing else. */
  open: [];
  close: [];
}>();

const editing = ref(false);
const draft = ref("");
const editor = useTemplateRef<HTMLInputElement | HTMLTextAreaElement>("editor");

/** True while `commit` is in flight, so a blur landing on top of an Enter
 *  cannot write the same value twice. */
let writing = false;

function begin(): void {
  if (editing.value) return;
  draft.value = props.value;
  editing.value = true;
  emit("open");

  void nextTick(() => {
    const area = editor.value;
    if (!area) return;
    area.focus();
    area.setSelectionRange(area.value.length, area.value.length);
  });
}

function close(): void {
  editing.value = false;
  emit("close");
}

/**
 * Ends the edit. Escape drops it; everything else — Enter, the button, the
 * caret leaving — writes it.
 *
 * Nothing is written when the text did not change, which is what keeps a click
 * into a field and back out again from touching the file's timestamp.
 */
async function finish(keep: boolean): Promise<void> {
  if (!editing.value || writing) return;
  if (!keep || draft.value === props.value) {
    close();
    return;
  }

  writing = true;
  try {
    if (await props.commit(draft.value)) close();
  } finally {
    writing = false;
  }
}

/** The mouse path out of an editor. It acts on mousedown and takes the default
 *  with it, so the blur that would otherwise write the draft never happens —
 *  the same reason the notes panel's pencil does (TQ-0027). */
function cancel(): void {
  if (!editing.value) return;
  close();
}
</script>

<template>
  <div class="inline-text" :class="{ multiline, editing }">
    <template v-if="editing">
      <textarea
        v-if="multiline"
        :id="`${id}-edit`"
        ref="editor"
        v-model="draft"
        class="inline-editor"
        rows="12"
        spellcheck="false"
        :aria-label="label"
        @keydown.esc.prevent.stop="cancel"
        @keydown.enter.ctrl.prevent="finish(true)"
        @keydown.enter.meta.prevent="finish(true)"
        @blur="finish(true)"
      ></textarea>
      <input
        v-else
        :id="`${id}-edit`"
        ref="editor"
        v-model="draft"
        class="inline-editor"
        type="text"
        autocomplete="off"
        :aria-label="label"
        @keydown.enter.prevent="finish(true)"
        @keydown.esc.prevent.stop="cancel"
        @blur="finish(true)"
      />
      <div v-if="multiline" class="inline-actions">
        <button type="button" class="primary" @mousedown.prevent @click="finish(true)">Save</button>
        <!-- Both handlers, and both load-bearing: mousedown is what stops the
             textarea blurring (which would write the draft this button exists
             to drop), and click is the only one a keyboard fires. -->
        <button type="button" class="ghost" @mousedown.prevent="cancel" @click="cancel">
          Cancel
        </button>
      </div>
    </template>

    <component
      :is="heading ? 'h2' : 'div'"
      v-else
      :id="id"
      class="inline-value"
      :class="{ empty: value === '' }"
      role="button"
      tabindex="0"
      :title="`Click to edit the ${label.toLowerCase()}`"
      @click="begin"
      @keydown.enter.prevent="begin"
    >
      <slot>{{ value === "" ? placeholder : value }}</slot>
    </component>
  </div>
</template>

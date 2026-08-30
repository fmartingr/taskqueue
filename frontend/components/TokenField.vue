<script setup lang="ts">
/**
 * A list the dialog edits one entry at a time: the labels, and the
 * dependencies.
 *
 * A comma-separated text field would have been fewer lines, and is what the
 * dialog used to have, but it makes every change to a list a rewrite of the
 * whole list — which under TQ-0069's rule is a collision with anyone who added
 * a label while it was open. One chip and one ✕ is also what the ticket drew.
 *
 * The chips are drawn from what the file holds, so a refused write simply never
 * appears: there is no local copy of the list to put back.
 */
import { nextTick, ref, useTemplateRef } from "vue";

const props = defineProps<{
  /** The entries as the file holds them. */
  values: string[];
  /** The id the row carries. The add button is `<id>-add` and the box it opens
   *  is `<id>-input`. */
  id: string;
  /** What one entry is, for the buttons' labels. */
  label: string;
  placeholder?: string;
  /** Writes the list and answers whether it landed. A refused add keeps what
   *  was typed, so it can be copied before it is thrown away. */
  commit: (values: string[]) => Promise<boolean>;
}>();

const adding = ref(false);
const draft = ref("");
const input = useTemplateRef<HTMLInputElement>("input");

/** True while a write is in flight: a blur landing on top of an Enter would
 *  otherwise send the same entry twice. */
let writing = false;

function begin(): void {
  adding.value = true;
  draft.value = "";
  void nextTick(() => input.value?.focus());
}

async function add(): Promise<void> {
  if (writing) return;
  const value = draft.value.trim();
  // An empty box is how the field is closed: clicking away from one is a
  // change of mind, not an entry.
  if (value === "" || props.values.includes(value)) {
    adding.value = false;
    return;
  }

  writing = true;
  try {
    if (await props.commit([...props.values, value])) adding.value = false;
  } finally {
    writing = false;
  }
}

async function remove(value: string): Promise<void> {
  if (writing) return;
  writing = true;
  try {
    await props.commit(props.values.filter((candidate) => candidate !== value));
  } finally {
    writing = false;
  }
}
</script>

<template>
  <div :id="id" class="tokens">
    <span v-for="value in values" :key="value" class="token">
      <slot :value="value">{{ value }}</slot>
      <button
        type="button"
        class="ghost icon token-remove"
        :title="`Remove ${value}`"
        :aria-label="`Remove ${value}`"
        @click="remove(value)"
      >
        ✕
      </button>
    </span>

    <input
      v-if="adding"
      :id="`${id}-input`"
      ref="input"
      v-model="draft"
      class="token-input"
      type="text"
      autocomplete="off"
      :placeholder="placeholder"
      :aria-label="`Add ${label}`"
      @keydown.enter.prevent="add"
      @keydown.esc.prevent.stop="adding = false"
      @blur="add"
    />
    <button
      v-else
      :id="`${id}-add`"
      type="button"
      class="ghost icon token-add"
      :title="`Add ${label}`"
      :aria-label="`Add ${label}`"
      @click="begin"
    >
      ＋
    </button>
  </div>
</template>

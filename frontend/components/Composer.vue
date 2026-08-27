<script setup lang="ts">
/**
 * The inline "add a card" composer. Enter files the card and stays open for the
 * next one, losing focus files it and closes, an empty draft is discarded, and
 * Escape cancels.
 *
 * The draft is this component's own state, which is the whole point of the
 * migration: the old board kept it in a global because every poll rebuilt the
 * board and would otherwise have taken the half-typed title away. Here the
 * textarea is mounted for as long as the composer is open, so a re-render
 * cannot touch it — and the caret and the selection stay where they were.
 *
 * That is what lets the board stay live while a composer is open (TQ-0084).
 * Its column keys its cards by ID and this composer sits outside that list, so
 * a refresh patches the cards around it and never remounts it. Nothing else
 * protects the draft: the guard that used to freeze the board for it is gone.
 */
import { onMounted, ref } from "vue";

import { describe } from "../api";
import type { Status } from "../board";
import { composing, quickAdd, toast } from "../state";

const props = defineProps<{ status: Status }>();

const input = ref<HTMLTextAreaElement | null>(null);
const draft = ref("");

/** Set while a create is in flight, and once the composer has been closed, so
 *  the blur that follows either one cannot file a second card. */
let settled = false;

onMounted(() => {
  input.value?.focus();
});

/**
 * Closes this composer, and only this one.
 *
 * The guard is not defensive: `file` awaits the create before closing, and by
 * the time it returns the user may have opened a composer in another column —
 * `composing` is shared, so an unguarded write here would close theirs instead,
 * and the blur that follows would file whatever they had typed so far.
 */
function close(): void {
  settled = true;
  draft.value = "";
  if (composing.value === props.status) composing.value = null;
}

async function file(keepOpen: boolean): Promise<void> {
  if (settled) return;

  const title = draft.value.trim();
  if (title === "") {
    close();
    return;
  }

  settled = true;
  draft.value = ""; // immediate feedback while the request is in flight
  try {
    await quickAdd(title, props.status);
    if (keepOpen) {
      settled = false;
      return;
    }
    close();
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
    // Hand the text back rather than losing what was typed — reopening this
    // column's composer first if the user has since closed it, since otherwise
    // the draft is restored into a component nothing is rendering.
    settled = false;
    if (composing.value === null) composing.value = props.status;
    if (composing.value === props.status) {
      draft.value = title;
      input.value?.focus();
    }
  }
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
  <div class="composer">
    <textarea
      ref="input"
      v-model="draft"
      class="composer-input"
      rows="2"
      placeholder="Title"
      @keydown.enter="onEnter($event, () => file(true))"
      @keydown.esc.prevent="close"
      @blur="file(false)"
    ></textarea>
  </div>
</template>

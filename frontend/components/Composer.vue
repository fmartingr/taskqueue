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

function close(): void {
  settled = true;
  draft.value = "";
  composing.value = null;
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
    // Hand the text back rather than losing what was typed.
    settled = false;
    draft.value = title;
    input.value?.focus();
  }
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
      @keydown.enter.exact.prevent="file(true)"
      @keydown.esc.prevent="close"
      @blur="file(false)"
    ></textarea>
  </div>
</template>

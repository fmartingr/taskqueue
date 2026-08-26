<script setup lang="ts">
/** How many notes a card's task carries, so the panel is worth opening. */
import { computed } from "vue";

const props = defineProps<{ count: number }>();

const title = computed(() => (props.count === 1 ? "1 note" : `${props.count} notes`));
</script>

<template>
  <span class="note-badge" :title="title"
    ><svg viewBox="0 0 16 16" width="12" height="12" aria-hidden="true">
      <path
        fill="currentColor"
        d="M2 3.5A1.5 1.5 0 0 1 3.5 2h9A1.5 1.5 0 0 1 14 3.5v6a1.5 1.5 0 0 1-1.5 1.5H6.6L3.7 13.7A.5.5 0 0 1 3 13.3V11h-.5A1.5 1.5 0 0 1 1 9.5v-6z"
      /></svg
    >{{ count }}</span
  >
</template>

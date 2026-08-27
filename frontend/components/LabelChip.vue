<script setup lang="ts">
/**
 * A label chip: the configured colour behind text picked to contrast with it,
 * or the neutral pill when the project does not declare this label.
 *
 * A scoped label is one pill in two halves: the colour carries the scope, and
 * the page's own surface carries the value with the colour as its text, under a
 * border in that same colour so the halves read as one object. That is what
 * keeps the half saying what kind of label it is on the board, where a display
 * name alone would drop it.
 *
 * The raw label is always the tooltip, because that is what the CLI and the
 * filters take — the halves are only what the board shows.
 */
import { computed } from "vue";

import { labelChip, labelHalves } from "../board";
import { labels } from "../state";

const props = defineProps<{ name: string }>();

const halves = computed(() => labelHalves(props.name, labels.value));
const chip = computed(() => labelChip(props.name, labels.value));
const scoped = computed(() => halves.value.scope !== "");

/**
 * A pill in one piece carries the colour itself; a pill in two carries only the
 * border that ties its halves together, and hands the colour to the halves.
 */
const pillStyle = computed(() => {
  const drawn = chip.value;
  if (drawn === null) return undefined;
  return scoped.value
    ? { borderColor: drawn.background }
    : { background: drawn.background, color: drawn.text };
});

const scopeStyle = computed(() =>
  chip.value === null ? undefined : { background: chip.value.background, color: chip.value.text },
);

const valueStyle = computed(() => (chip.value === null ? undefined : { color: chip.value.background }));
</script>

<template>
  <span class="label" :class="{ tinted: chip !== null, scoped }" :style="pillStyle" :title="name">
    <template v-if="scoped">
      <span class="label-scope" :style="scopeStyle">{{ halves.scope }}</span>
      <span class="label-value" :style="valueStyle">{{ halves.value }}</span>
    </template>
    <template v-else>{{ halves.value }}</template>
  </span>
</template>

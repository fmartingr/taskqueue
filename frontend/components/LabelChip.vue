<script setup lang="ts">
/**
 * A label chip: the configured colour behind text picked to contrast with it,
 * or the neutral pill when the project does not declare this label. The raw
 * label is always the tooltip, because that is what the CLI and the filters
 * take — the display name is only what the board shows.
 */
import { computed } from "vue";

import { labelChip, labelDisplay } from "../board";
import { labels } from "../state";

const props = defineProps<{ name: string }>();

const display = computed(() => labelDisplay(props.name, labels.value));
const chip = computed(() => labelChip(props.name, labels.value));
</script>

<template>
  <span
    class="label"
    :class="{ tinted: chip !== null }"
    :style="chip ? { background: chip.background, color: chip.text } : undefined"
    :title="name"
    >{{ display }}</span
  >
</template>

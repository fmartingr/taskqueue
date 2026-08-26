<script setup lang="ts">
/**
 * A card's priority badge, drawn from the project's vocabulary the way a label
 * chip is drawn from its own: the configured colour behind text picked to
 * contrast with it, or the neutral pill for a value the project no longer
 * declares. The stored value is always the tooltip, since that is what the CLI
 * and the filters take.
 */
import { computed } from "vue";

import { defaultPriority, priorityChip, priorityDisplay } from "../board";
import { priorities } from "../state";

const props = defineProps<{ priority?: string }>();

const name = computed(() => props.priority || defaultPriority(priorities.value));
const display = computed(() => priorityDisplay(name.value, priorities.value));
const chip = computed(() => priorityChip(name.value, priorities.value));
</script>

<template>
  <span
    class="badge"
    :class="{ tinted: chip !== null }"
    :style="chip ? { background: chip.background, color: chip.text } : undefined"
    :title="name"
    >{{ display }}</span
  >
</template>

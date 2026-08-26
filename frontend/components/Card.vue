<script setup lang="ts">
/**
 * One task on the board. It is the drag source — the column is the drop target
 * — and clicking it opens the task dialog.
 */
import { computed } from "vue";

import { pendingDependencies, type Task } from "../board";
import { splitBody } from "../notes";
import { columns, dragging, index, openTaskID } from "../state";
import LabelChip from "./LabelChip.vue";
import NoteBadge from "./NoteBadge.vue";
import PriorityBadge from "./PriorityBadge.vue";

const props = defineProps<{ task: Task }>();

const pending = computed(() => pendingDependencies(props.task, index.value, columns.value));
const noteCount = computed(() => splitBody(props.task.body ?? "").notes.length);
const hasMeta = computed(
  () => !!props.task.assignee || (props.task.labels ?? []).length > 0 || noteCount.value > 0,
);

function onDragStart(event: DragEvent): void {
  dragging.value = props.task.id;
  event.dataTransfer?.setData("text/plain", props.task.id);
  if (event.dataTransfer) event.dataTransfer.effectAllowed = "move";
}

function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    openTaskID.value = props.task.id;
  }
}
</script>

<template>
  <article
    class="card"
    :class="{ blocked: pending.length > 0, dragging: dragging === task.id }"
    :data-id="task.id"
    draggable="true"
    tabindex="0"
    @dragstart="onDragStart"
    @dragend="dragging = null"
    @click="openTaskID = task.id"
    @keydown="onKeydown"
  >
    <div class="card-top">
      <span class="task-id">{{ task.id }}</span>
      <PriorityBadge :priority="task.priority" />
    </div>
    <p class="card-title">{{ task.title }}</p>

    <div v-if="hasMeta" class="card-meta">
      <span v-if="task.assignee" class="assignee">{{ task.assignee }}</span>
      <LabelChip v-for="label in task.labels ?? []" :key="label" :name="label" />
      <NoteBadge v-if="noteCount > 0" :count="noteCount" />
    </div>

    <p v-if="pending.length > 0" class="blocked-note">Blocked by {{ pending.join(", ") }}</p>
  </article>
</template>

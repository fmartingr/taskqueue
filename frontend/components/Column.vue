<script setup lang="ts">
/**
 * One status column: its cards, its composer, and the drop target for its
 * status. Native drag and drop, so the column only has to say yes to a drag
 * passing over it and read the ID back out of the drop.
 */
import { computed, ref } from "vue";

import type { Status } from "../board";
import { composing, dragging, moveTask, visible } from "../state";
import Card from "./Card.vue";
import Composer from "./Composer.vue";

const props = defineProps<{ status: Status }>();

const cards = computed(() => visible.value.filter((task) => task.status === props.status));
const over = ref(false);

function onDragLeave(event: DragEvent): void {
  // dragleave also fires when the pointer moves onto a card inside the column;
  // only drop the highlight when it really left.
  const to = event.relatedTarget;
  const column = event.currentTarget;
  if (to instanceof Node && column instanceof Node && column.contains(to)) return;
  over.value = false;
}

function onDrop(event: DragEvent): void {
  over.value = false;
  const id = event.dataTransfer?.getData("text/plain") || dragging.value;
  if (id) void moveTask(id, props.status);
}
</script>

<template>
  <section
    class="column"
    :class="{ 'drop-target': over }"
    :data-status="status"
    @dragover.prevent="over = true"
    @dragleave="onDragLeave"
    @drop.prevent="onDrop"
  >
    <header class="column-header">
      <h2>{{ status }}</h2>
      <span class="count">{{ cards.length }}</span>
    </header>

    <div class="column-tasks">
      <Card v-for="task in cards" :key="task.id" :task="task" />
    </div>

    <Composer v-if="composing === status" :status="status" />
    <button v-else type="button" class="composer-open" @click="composing = status">
      + Add a card
    </button>
  </section>
</template>

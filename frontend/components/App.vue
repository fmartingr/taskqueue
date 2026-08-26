<script setup lang="ts">
/**
 * The whole page: the top bar, the board, the status line and the two dialogs.
 *
 * Both dialogs are mounted only while they are open, which is what makes their
 * state theirs: closing one throws its fields away, and opening one starts from
 * the task (or from the project's defaults) rather than from whatever the last
 * open left behind.
 */
import { computed } from "vue";

import { creating, openTaskID, statusLine, tasks } from "../state";
import Board from "./Board.vue";
import CreateDialog from "./CreateDialog.vue";
import FilterBar from "./FilterBar.vue";
import TaskDialog from "./TaskDialog.vue";
import Toasts from "./Toasts.vue";

const open = computed(() => tasks.value.find((task) => task.id === openTaskID.value));
</script>

<template>
  <header class="topbar">
    <h1 class="brand">tq</h1>
    <FilterBar />
    <button id="new-task" type="button" class="primary" @click="creating = true">New task</button>
  </header>

  <Board />

  <footer class="statusbar">
    <span id="status-line">{{ statusLine }}</span>
  </footer>

  <Toasts />

  <TaskDialog v-if="open" :task="open" @close="openTaskID = null" />
  <CreateDialog v-if="creating" @close="creating = false" />
</template>

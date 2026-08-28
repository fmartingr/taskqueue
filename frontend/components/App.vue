<script setup lang="ts">
/**
 * The whole page: the top bar, the board, the status line and the two dialogs.
 *
 * Both dialogs are mounted only while they are open, which is what makes their
 * state theirs: closing one throws its fields away, and opening one starts from
 * the task (or from the project's defaults) rather than from whatever the last
 * open left behind.
 *
 * The task dialog is mounted on `openTask`, which state.ts holds rather than
 * derives, so a refresh that no longer lists the task cannot unmount the dialog
 * and throw away what was being typed in it. The dialog says the task is gone
 * itself.
 */
import { creating, openTask, openTaskID, statusLine } from "../state";
import Board from "./Board.vue";
import CreateDialog from "./CreateDialog.vue";
import FilterBar from "./FilterBar.vue";
import SearchBar from "./SearchBar.vue";
import TaskDialog from "./TaskDialog.vue";
import Toasts from "./Toasts.vue";
</script>

<template>
  <header class="topbar">
    <h1 class="brand">tq</h1>
    <SearchBar />
    <button id="new-task" type="button" class="primary" @click="creating = true">New task</button>
    <FilterBar />
  </header>

  <Board />

  <footer class="statusbar">
    <span id="status-line">{{ statusLine }}</span>
  </footer>

  <Toasts />

  <!-- Keyed by the task: `openTask` is a ref rather than a find, so it could be
       pointed straight at another task without passing through nothing, and an
       instance reused across that switch would keep the first task's fields and
       save them onto the second. -->
  <TaskDialog v-if="openTask" :key="openTask.id" :task="openTask" @close="openTaskID = null" />
  <CreateDialog v-if="creating" @close="creating = false" />
</template>

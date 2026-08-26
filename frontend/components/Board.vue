<script setup lang="ts">
/** The four columns. Which four is board.ts's STATUSES, for now: making them
 *  the project's own is TQ-0035. */
import { STATUSES } from "../board";
import Column from "./Column.vue";
</script>

<template>
  <main id="board" class="board">
    <Column v-for="status in STATUSES" :key="status" :status="status" />
  </main>
</template>

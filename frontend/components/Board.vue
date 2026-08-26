<script setup lang="ts">
/**
 * The board's columns, in the order the project declares them.
 *
 * The count is a CSS variable rather than a fixed `repeat(4, …)`, because a
 * project can declare as many columns as it likes and the grid has to follow.
 */
import { columns } from "../state";
import Column from "./Column.vue";
</script>

<template>
  <main id="board" class="board" :style="{ '--column-count': columns.length }">
    <Column v-for="column in columns" :key="column.name" :status="column.name" />
  </main>
</template>

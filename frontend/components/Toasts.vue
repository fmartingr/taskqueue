<script setup lang="ts">
/** What went wrong, and the occasional confirmation that something went right. */
import { toasts } from "../state";
</script>

<template>
  <div id="toasts" class="toasts" aria-live="polite">
    <div v-for="item in toasts" :key="item.id" class="toast" :class="item.kind">
      {{ item.message }}
    </div>
  </div>
</template>

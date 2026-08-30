<script setup lang="ts">
/**
 * A task body, rendered.
 *
 * The one place in the board that puts markup it built into the page, and the
 * only reason it can is that `renderMarkdown` escapes the document before it
 * emits anything — there is no HTML from the file left in what arrives here.
 * Its cases are in ../markdown.test.ts, including every way a body could try
 * to become markup.
 */
import { computed } from "vue";

import { renderMarkdown } from "../markdown";

const props = defineProps<{ source: string }>();

const html = computed(() => renderMarkdown(props.source));
</script>

<template>
  <div class="markdown" v-html="html"></div>
</template>

<script setup lang="ts">
/**
 * One dependency in the task dialog: the ID, the title, the status, and a way
 * into that task.
 *
 * An ID on its own answers neither of the questions somebody has about a
 * dependency — what is it, and is it done — and the board already holds both:
 * the listing it drew the cards from is the same one this reads (TQ-0105).
 *
 * The status comes before the title and is drawn to a fixed width, so a list
 * of them is one column to read down and the titles all start at the same
 * place.
 *
 * A dependency the listing does not carry is drawn as text rather than as a
 * link, and says so. It is not a lookup that failed: a missing dependency
 * blocks its dependent for good, and it is the row where that is found out.
 * There is also nothing to open — pointing the dialog at an ID the board has
 * no task for would leave it showing the task it was on.
 */
import type { Dependency } from "../board";

defineProps<{
  dependency: Dependency;
  /** Opens the dependency's own task. The dialog owns this, because whether a
   *  click may navigate at all depends on the editors open in it. */
  open: (id: string) => void;
}>();
</script>

<template>
  <button
    v-if="dependency.task"
    type="button"
    class="ghost dep"
    :title="`Open ${dependency.id}`"
    @click="open(dependency.id)"
  >
    <span class="token-id">{{ dependency.id }}</span>
    <span class="dep-status" :class="{ pending: dependency.pending }">{{ dependency.status }}</span>
    <span class="dep-title">{{ dependency.task.title }}</span>
  </button>

  <span v-else class="dep gone">
    <span class="token-id">{{ dependency.id }}</span>
    <span class="dep-status pending">missing</span>
    <span class="dep-title">not in the queue</span>
  </span>
</template>

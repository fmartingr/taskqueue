<script setup lang="ts">
/**
 * The filter bar. Status is the one closed set the board still spells out
 * itself; priorities and labels come from the project's vocabulary in
 * GET /api/config, because hard-coding either would offer values the store
 * refuses and hide the ones it wants.
 */
import { computed, ref, watch } from "vue";

import {
  groupLabels,
  labelsInUse,
  priorityOptions,
  STATUSES,
  type LabelGroup,
  type PriorityOption,
} from "../board";
import { filters, labels, priorities, tasks } from "../state";

// The filter's own value is offered even when the project has dropped it, or
// when nothing carries it any more: dropping the option would leave the bar
// reading "any" while the board went on hiding everything.
const priorityChoices = computed(() => priorityOptions(priorities.value, [filters.priority]));

const desiredGroups = computed(() => {
  const inUse = labelsInUse(tasks.value);
  if (filters.label) inUse.push(filters.label);
  return groupLabels(labels.value, inUse);
});

/**
 * The label options as they are currently rendered, which is not always the
 * options the tasks currently justify.
 *
 * Nothing is rebuilt while the select has focus. The poll runs every few
 * seconds and an agent filing a task with a new label is the normal case here:
 * replacing the options under an expanded dropdown collapses it mid-choice.
 * Blurring picks the skipped rebuild back up.
 */
const shownGroups = ref<LabelGroup[]>(desiredGroups.value);
const holding = ref(false);

watch(desiredGroups, (groups) => {
  if (!holding.value) shownGroups.value = groups;
});

function releaseLabelOptions(): void {
  holding.value = false;
  shownGroups.value = desiredGroups.value;
}

/** Labels with no prefix are the select's own options; the rest are grouped. */
const looseLabels = computed(() => shownGroups.value.find((group) => group.prefix === "")?.labels ?? []);
const labelGroups = computed(() => shownGroups.value.filter((group) => group.prefix !== ""));

const priorityTitle = (option: PriorityOption) =>
  option.configured ? option.name : `${option.name} — not in the project's priority set`;

const labelTitle = (name: string, configured: boolean) =>
  configured ? name : `${name} — not in the project's label set`;

function reset(): void {
  filters.status = "";
  filters.priority = "";
  filters.assignee = "";
  filters.label = "";
  filters.ready = false;
}
</script>

<template>
  <div class="filters">
    <label>
      Status
      <select id="filter-status" v-model="filters.status">
        <option value="">any</option>
        <option v-for="status in STATUSES" :key="status" :value="status">{{ status }}</option>
      </select>
    </label>

    <label>
      Priority
      <select id="filter-priority" v-model="filters.priority">
        <option value="">any</option>
        <option
          v-for="option in priorityChoices"
          :key="option.name"
          :value="option.name"
          :title="priorityTitle(option)"
        >{{ option.display }}</option>
      </select>
    </label>

    <label>
      Assignee
      <input
        id="filter-assignee"
        v-model="filters.assignee"
        type="search"
        placeholder="anyone"
        autocomplete="off"
      />
    </label>

    <label>
      Label
      <select
        id="filter-label"
        v-model="filters.label"
        @focus="holding = true"
        @blur="releaseLabelOptions"
      >
        <option value="">any</option>
        <option
          v-for="label in looseLabels"
          :key="label.name"
          :value="label.name"
          :title="labelTitle(label.name, label.configured)"
        >{{ label.display }}</option>
        <optgroup v-for="group in labelGroups" :key="group.prefix" :label="group.prefix">
          <option
            v-for="label in group.labels"
            :key="label.name"
            :value="label.name"
            :title="labelTitle(label.name, label.configured)"
          >{{ label.display }}</option>
        </optgroup>
      </select>
    </label>

    <label class="checkbox">
      <input id="filter-ready" v-model="filters.ready" type="checkbox" />
      Ready only
    </label>

    <button id="filter-reset" type="button" class="ghost" @click="reset">Reset</button>
  </div>
</template>

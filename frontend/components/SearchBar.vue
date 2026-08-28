<script setup lang="ts">
/**
 * The search bar: one line of text that edits the same filters the bar below it
 * does (TQ-0068).
 *
 * It complements the filter bar rather than replacing it. Both write into the
 * one `filters` object, so typing `priority=urgent` moves the priority select
 * and clearing that select rewrites the query — there is no second state to
 * fall out of step, and no mode to be in.
 *
 * The query is only reformatted when the two have actually drifted apart, which
 * is what keeps a formatter from rewriting a half-typed term under the cursor.
 *
 * Every rule the line obeys is in search.ts, which knows nothing about Vue and
 * has its own unit tests; what is left here is the input, the menu and the keys.
 */
import { computed, nextTick, ref, watch } from "vue";

import { labelDisplay, labelsInUse } from "../board";
import {
  applyCompletion,
  completeQuery,
  equalFilters,
  formatQuery,
  parseQuery,
  type Sources,
} from "../search";
import { columns, filters, labels, priorities, tasks } from "../state";

const input = ref<HTMLInputElement | null>(null);
const query = ref(formatQuery(filters));
const caret = ref(0);
const active = ref(0);
const focused = ref(false);
/** Set by Escape, cleared by the next keystroke: the menu can be sent away
 *  without the query it was suggesting for being sent away with it. */
const dismissed = ref(false);

watch(query, (line) => {
  const parsed = parseQuery(line);
  if (equalFilters(parsed, filters)) return;
  Object.assign(filters, parsed);
});

watch(filters, () => {
  if (equalFilters(parseQuery(query.value), filters)) return;
  query.value = formatQuery(filters);
});

/** Every label the project declares or a task carries — the same two sources
 *  the filter bar's select draws on, so neither offers what the other hides. */
const labelNames = computed(() =>
  [...new Set([...Object.keys(labels.value), ...labelsInUse(tasks.value)])]
    .filter((name) => name !== "")
    .sort(),
);

/** Assignees are nobody's vocabulary: they exist because a task carries one. */
const assignees = computed(() =>
  [...new Set(tasks.value.map((task) => task.assignee ?? ""))].filter((name) => name !== "").sort(),
);

const sources = computed<Sources>(() => ({
  status: columns.value.map((column) => ({ value: column.name, display: column.display_name })),
  priority: priorities.value.map((priority) => ({
    value: priority.name,
    display: priority.display_name,
  })),
  label: labelNames.value.map((name) => ({ value: name, display: labelDisplay(name, labels.value) })),
  assignee: assignees.value.map((name) => ({ value: name })),
}));

const completion = computed(() => completeQuery(query.value, caret.value, sources.value));

const showing = computed(
  () => focused.value && !dismissed.value && completion.value.suggestions.length > 0,
);

watch(completion, () => {
  active.value = 0;
});

function syncCaret(): void {
  const field = input.value;
  if (field) caret.value = field.selectionStart ?? field.value.length;
}

function onInput(event: Event): void {
  const field = event.target as HTMLInputElement;
  query.value = field.value;
  caret.value = field.selectionStart ?? field.value.length;
  dismissed.value = false;
}

async function accept(at: number): Promise<void> {
  const suggestion = completion.value.suggestions[at];
  if (suggestion === undefined) return;

  const next = applyCompletion(query.value, completion.value, suggestion);
  query.value = next.query;
  caret.value = next.caret;
  dismissed.value = false;

  // The DOM value is written on the next tick, and a selection set before it
  // would be set on the old text.
  await nextTick();
  const field = input.value;
  if (field === null) return;
  field.focus();
  field.setSelectionRange(next.caret, next.caret);
}

/**
 * The keys the menu owns, and only while it has something to show. Everything
 * else — Tab above all — is left to the browser: this is a suggestion list, not
 * a focus trap, and tabbing out of it has to reach the filter bar.
 */
function onKeydown(event: KeyboardEvent): void {
  if (event.key === "Escape") {
    if (!showing.value) return;
    dismissed.value = true;
    // Stopped as well as prevented: nothing above should read this Escape as a
    // reason to close something the user was not closing.
    event.preventDefault();
    event.stopPropagation();
    return;
  }

  if (event.key === "ArrowDown" || event.key === "ArrowUp") {
    dismissed.value = false; // an arrow is how a dismissed menu is asked back
    const count = completion.value.suggestions.length;
    if (count === 0) return;
    event.preventDefault();
    const step = event.key === "ArrowDown" ? 1 : -1;
    active.value = (active.value + step + count) % count;
    return;
  }

  if (event.key === "Enter" && showing.value) {
    event.preventDefault();
    void accept(active.value);
  }
}
</script>

<template>
  <div class="search">
    <label class="search-field">
      Search
      <input
        id="search-query"
        ref="input"
        :value="query"
        type="text"
        role="combobox"
        class="search-input"
        placeholder="text, or priority=urgent"
        autocomplete="off"
        spellcheck="false"
        aria-autocomplete="list"
        aria-controls="search-suggestions"
        :aria-expanded="showing"
        :aria-activedescendant="showing ? `search-option-${active}` : undefined"
        @input="onInput"
        @keyup="syncCaret"
        @click="syncCaret"
        @keydown="onKeydown"
        @focus="focused = true"
        @blur="focused = false"
      />
    </label>

    <ul v-if="showing" id="search-suggestions" class="search-menu" role="listbox">
      <li
        v-for="(suggestion, at) in completion.suggestions"
        :id="`search-option-${at}`"
        :key="suggestion.insert"
        class="search-option"
        :class="{ active: at === active }"
        role="option"
        :aria-selected="at === active"
        @mousedown.prevent="accept(at)"
        @mouseenter="active = at"
      >
        <span class="search-option-label">{{ suggestion.label }}</span>
        <span v-if="suggestion.detail" class="search-option-detail">{{ suggestion.detail }}</span>
      </li>
    </ul>
  </div>
</template>

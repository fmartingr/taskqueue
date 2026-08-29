<script setup lang="ts">
/**
 * The search bar: the one line of text the whole board is narrowed from
 * (TQ-0068, TQ-0098).
 *
 * It is the only writer of `filters`, so the line is the input and `filters` is
 * what it parses to — one state with one editor, and nothing to fall out of step
 * with. Every field the removed filter bar held a control for is a term here:
 * `status=`, `priority=`, `label=`, `assignee=` and the bare `ready`.
 *
 * The line is also the address: it is read out of `?q=` on load and written
 * back with `replaceState`, so a filtered board is something to send someone
 * and a reload keeps what was typed. One parameter, no router.
 *
 * Every rule the line obeys is in search.ts, which knows nothing about Vue and
 * has its own unit tests; what is left here is the input, the menu and the keys.
 */
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from "vue";

import { labelDisplay, labelsInUse } from "../board";
import {
  applyCompletion,
  canonicalValues,
  completeQuery,
  parseQuery,
  queryFromURL,
  sameFilters,
  urlWithQuery,
  type Sources,
} from "../search";
import { columns, filters, labels, priorities, tasks } from "../state";

/** The one key that puts the cursor here, GitHub style. It is claimed only when
 *  nothing else is being typed into — see `onShortcut`. */
const FOCUS_KEY = "/";

const input = ref<HTMLInputElement | null>(null);
const menu = ref<HTMLUListElement | null>(null);
const query = ref(queryFromURL(window.location.href));
const caret = ref(0);
const active = ref(0);
const focused = ref(false);
/** Set by Escape, cleared by the next keystroke: the menu can be sent away
 *  without the query it was suggesting for being sent away with it. */
const dismissed = ref(false);

/** Every label the project declares or a task carries: a configured label
 *  nothing uses is still part of the vocabulary, and a label in use that
 *  nothing declares still has to be filterable. */
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

/**
 * What the line currently says, in the project's own spelling.
 *
 * It depends on the vocabularies as well as the line, which is why it is a
 * computed rather than something the input handler does once: the board reads
 * its configuration asynchronously, so a query restored from the address bar is
 * parsed before there is anything to canonicalise it against.
 */
const parsed = computed(() => canonicalValues(parseQuery(query.value), sources.value));

// The one write to `filters`, which is what makes it derived: it ends up
// holding exactly what `parsed` says, spelling included.
//
// `sameFilters` rather than `equalFilters` is what promises that. The
// vocabularies arrive after the first parse, so the correction `canonicalValues`
// makes is usually a change of case and nothing else — and `equalFilters`
// ignores case, so it would call the corrected set equal and skip the write,
// leaving `filters` on whatever was typed. There is a guard at all because
// `parsed` is rebuilt whenever the vocabularies move rather than only when the
// line does, and `parseQuery` hands back fresh `text` and `excluded` arrays each
// time: unguarded, every listing would write to `filters` for a query nobody
// touched.
watch(
  parsed,
  (next) => {
    if (sameFilters(next, filters)) return;
    Object.assign(filters, next);
  },
  { immediate: true },
);

// The address follows the line — typed, completed or cleared.
watch(query, (line) => {
  window.history.replaceState(null, "", urlWithQuery(window.location.href, line));
});

const completion = computed(() => completeQuery(query.value, caret.value, sources.value));

const showing = computed(
  () => focused.value && !dismissed.value && completion.value.suggestions.length > 0,
);

watch(completion, () => {
  active.value = 0;
});

/** Keeps the highlighted row on screen: the menu scrolls, and a keyboard
 *  selection that walks past its edge would otherwise be invisible. */
watch(active, async (at) => {
  await nextTick();
  const row = menu.value?.children[at];
  if (row instanceof HTMLElement) row.scrollIntoView({ block: "nearest" });
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

/** Puts the cursor at the end of the line, which is where someone who asked for
 *  the box wants to carry on typing. */
function focusInput(): void {
  const field = input.value;
  if (field === null) return;
  field.focus();
  const end = field.value.length;
  field.setSelectionRange(end, end);
  caret.value = end;
}

/** The × in the box: the search's own affordance, where the hand already is.
 *  The line holds the whole filter set, so emptying it puts the whole board
 *  back — every term at once, whatever was narrowing it. */
function clear(): void {
  query.value = "";
  dismissed.value = false;
  void nextTick(focusInput);
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
 * a focus trap, and tabbing out of it has to reach the rest of the page.
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

/** Whether the keystroke is going into something that takes text, in which case
 *  `/` is a character and nothing else. `isContentEditable` covers a rich editor
 *  this board does not have yet and would not want to steal a slash from. */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  const tag = target.tagName;
  return tag === "INPUT" || tag === "TEXTAREA" || tag === "SELECT" || target.isContentEditable;
}

/**
 * `/` puts the cursor in the box, from anywhere on the board.
 *
 * It stands down for anything that is already taking text — the column
 * composers, the body and note editors, the dialog fields — and for an open
 * dialog whatever has focus inside it, because a modal is its own context and a
 * shortcut that reaches through one is a bug rather than a convenience.
 */
function onShortcut(event: KeyboardEvent): void {
  if (event.key !== FOCUS_KEY || event.isComposing) return;
  if (event.ctrlKey || event.metaKey || event.altKey || event.defaultPrevented) return;
  if (isTyping(event.target)) return;
  if (document.querySelector("dialog[open]") !== null) return;
  event.preventDefault();
  focusInput();
}

onMounted(() => document.addEventListener("keydown", onShortcut));
onBeforeUnmount(() => document.removeEventListener("keydown", onShortcut));
</script>

<template>
  <div class="search">
    <div class="search-box">
      <input
        id="search-query"
        ref="input"
        :value="query"
        type="text"
        role="combobox"
        class="search-input"
        placeholder="text, -not, or priority=urgent  (/)"
        autocomplete="off"
        spellcheck="false"
        aria-label="Search"
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

      <button
        v-if="query !== ''"
        id="search-clear"
        type="button"
        class="search-clear"
        title="Clear the search"
        aria-label="Clear the search"
        @click="clear"
      >×</button>
    </div>

    <ul v-if="showing" id="search-suggestions" ref="menu" class="search-menu" role="listbox">
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

<script setup lang="ts">
/**
 * The "New task" dialog. It is mounted when it opens and thrown away when it
 * closes, so each open starts from the project's defaults rather than from a
 * form that has to be reset back to its markup.
 */
import { computed, onMounted, ref } from "vue";

import { createTask, describe } from "../api";
import { defaultPriority, priorityOptions, STATUSES } from "../board";
import { splitList } from "../format";
import { priorities, refresh, toast } from "../state";

const emit = defineEmits<{ close: [] }>();

const dialog = ref<HTMLDialogElement | null>(null);
const titleField = ref<HTMLInputElement | null>(null);

const title = ref("");
const status = ref<string>("todo");
const priority = ref(defaultPriority(priorities.value));
const assignee = ref("");
const labelList = ref("");
const dependsOn = ref("");
const body = ref("");

const priorityChoices = computed(() => priorityOptions(priorities.value, []));

onMounted(() => {
  dialog.value?.showModal();
  titleField.value?.focus();
});

function dismiss(): void {
  dialog.value?.close();
}

async function submit(): Promise<void> {
  const wanted = title.value.trim();
  if (wanted === "") return;

  try {
    const task = await createTask({
      title: wanted,
      status: status.value,
      priority: priority.value,
      assignee: assignee.value,
      labels: splitList(labelList.value),
      depends_on: splitList(dependsOn.value),
      body: body.value,
    });
    dismiss();
    await refresh();
    toast(`Created ${task.id}`, "info");
  } catch (error) {
    toast(`Could not create the task: ${describe(error)}`);
  }
}
</script>

<template>
  <dialog id="create-dialog" ref="dialog" class="dialog" @close="emit('close')">
    <form id="create-form" method="dialog" @submit.prevent="submit">
      <header class="dialog-header">
        <span class="task-id">New task</span>
        <button
          type="button"
          class="ghost close"
          data-close="create-dialog"
          aria-label="Close"
          @click="dismiss"
        >
          ✕
        </button>
      </header>

      <label>
        Title
        <input id="create-title" ref="titleField" v-model="title" name="title" type="text" required />
      </label>

      <div class="grid">
        <label>
          Status
          <select id="create-status" v-model="status" name="status">
            <option v-for="option in STATUSES" :key="option" :value="option">{{ option }}</option>
          </select>
        </label>
        <label>
          Priority
          <select id="create-priority" v-model="priority" name="priority">
            <option
              v-for="option in priorityChoices"
              :key="option.name"
              :value="option.name"
              :title="option.name"
            >{{ option.display }}</option>
          </select>
        </label>
        <label>
          Assignee
          <input id="create-assignee" v-model="assignee" name="assignee" type="text" autocomplete="off" />
        </label>
        <label>
          Labels
          <input
            id="create-labels"
            v-model="labelList"
            name="labels"
            type="text"
            placeholder="backend, auth"
            autocomplete="off"
          />
        </label>
        <label>
          Depends on
          <input
            id="create-depends-on"
            v-model="dependsOn"
            name="depends_on"
            type="text"
            placeholder="TQ-0002"
            autocomplete="off"
          />
        </label>
      </div>

      <label>
        Body (Markdown)
        <textarea id="create-body" v-model="body" name="body" rows="8" spellcheck="false"></textarea>
      </label>

      <footer class="dialog-footer">
        <span class="spacer"></span>
        <button type="button" class="ghost" data-close="create-dialog" @click="dismiss">Cancel</button>
        <button type="submit" class="primary">Create</button>
      </footer>
    </form>
  </dialog>
</template>

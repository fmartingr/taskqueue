/**
 * tq Kanban board.
 *
 * The board fetches JSON from the same REST API the CLI writes through, renders
 * it with Vue, and polls so that tasks created or moved by an agent show up on
 * their own. Vue is bundled into the output by frontend/build.ts, so the page
 * still loads exactly one script and fetches nothing else.
 */

import { createApp } from "vue";

import App from "./components/App.vue";
import { start } from "./state";

createApp(App).mount("#app");
void start();

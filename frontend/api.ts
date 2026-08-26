/**
 * The REST API the board talks to.
 *
 * It is the same API the CLI writes through, so nothing here is a second
 * source of truth: every call reads from or writes to the Markdown files, and
 * the response is what the store actually has on disk.
 *
 * Framework-agnostic on purpose — no Vue in this file. `state.ts` owns the
 * state; this owns the wire.
 */

import type { ColumnSet, LabelSet, PrioritySet, Task } from "./board";

/** The fields POST /api/tasks and PATCH /api/tasks/{id} accept. */
export interface TaskInput {
  title: string;
  status?: string;
  priority?: string;
  assignee?: string;
  labels?: string[];
  depends_on?: string[];
  body?: string;
}

/**
 * A task file the server could not read, and why. It is skipped rather than
 * failing the listing, so this is the only place it is ever mentioned: the
 * board is what tells someone a file is missing from it (TQ-0011).
 */
export interface UnreadableFile {
  file: string;
  reason: string;
}

/** GET /api/status. */
export interface ServerStatus {
  ok: boolean;
  task_count: number;
  task_dir: string;
  version: string;
  unreadable: UnreadableFile[];
}

/** GET /api/config: the project marker as the server resolved it. */
export interface ProjectConfig {
  version: number;
  path: string;
  task_dir: string;
  file: string;
  labels: LabelSet;
  priorities: PrioritySet;
  columns: ColumnSet;
}

export async function api<T>(path: string, method = "GET", body?: unknown): Promise<T> {
  const init: RequestInit = { method };
  if (body !== undefined) {
    init.headers = { "Content-Type": "application/json" };
    init.body = JSON.stringify(body);
  }

  const response = await fetch(path, init);
  if (!response.ok) {
    throw new Error(await errorMessage(response));
  }
  return (await response.json()) as T;
}

async function errorMessage(response: Response): Promise<string> {
  try {
    const payload = (await response.json()) as { error?: string };
    if (payload.error) return payload.error;
  } catch {
    // The body was not JSON; fall back to the status line.
  }
  return `${response.status} ${response.statusText}`;
}

export const fetchStatus = () => api<ServerStatus>("/api/status");
export const fetchConfig = () => api<ProjectConfig>("/api/config");
export const fetchTasks = () => api<Task[]>("/api/tasks");
export const fetchTask = (id: string) => api<Task>(`/api/tasks/${id}`);
export const createTask = (input: TaskInput) => api<Task>("/api/tasks", "POST", input);
export const patchTask = (id: string, patch: Partial<TaskInput>) =>
  api<Task>(`/api/tasks/${id}`, "PATCH", patch);
export const addNote = (id: string, text: string) =>
  api<Task>(`/api/tasks/${id}/notes`, "POST", { text });

/** The message to put in a toast for whatever a rejected promise carried. */
export function describe(error: unknown): string {
  return error instanceof Error ? error.message : String(error);
}

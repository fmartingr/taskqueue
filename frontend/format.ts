/**
 * The conversions between what a field holds, what the API takes and what the
 * page says. Kept out of the components because more than one of them needs
 * each of these and none of them owns one.
 */

/** "backend, auth" — the comma-separated text fields the dialogs offer. */
export function splitList(value: string): string[] {
  return value
    .split(",")
    .map((item) => item.trim())
    .filter((item) => item.length > 0);
}

/** An RFC 3339 timestamp as a local time, or the raw value if it is not one. */
export function formatTime(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString();
}

/** "3 tasks", "1 task": a number of tasks, where naming them would not fit. */
export function taskCount(count: number): string {
  return `${count} task${count === 1 ? "" : "s"}`;
}

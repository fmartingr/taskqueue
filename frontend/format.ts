/**
 * The two conversions the dialogs do between what a field holds and what the
 * API takes. Kept out of the components because both dialogs need them and
 * neither one owns them.
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

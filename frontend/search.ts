/**
 * The search bar's query language, taken away from the DOM.
 *
 * One line of text stands for the same thing the filter bar holds: a `Filters`.
 * `parseQuery` reads the line into one, `formatQuery` writes one back out, and
 * the component keeps the two in step — so a query and the selects are never
 * two sources of truth, they are one state with two editors.
 *
 * `completeQuery` is the other half: given a caret, it says which slice of the
 * query is being typed and what could go there. The values come from the
 * project's own vocabularies rather than from anything here, because a
 * suggestion the store would refuse is worse than no suggestion at all.
 *
 * Like board.ts and notes.ts this knows nothing about Vue, and has its own unit
 * tests in search.test.ts.
 *
 * The syntax, in full:
 *
 *   auth                     free text: a substring of the id, title or body
 *   status=todo              a structured term; the key is case-insensitive
 *   assignee="agent api"     double quotes hold a value with spaces in it
 *   ready                    a bare word, the one key that needs no value
 *   ready=false              …and its explicit form
 *   "priority=urgent"        quoted, so it is free text rather than a term
 */

import type { Filters } from "./board";

/** The keys that take a value, in the order a formatted query lists them. */
const VALUE_KEYS = ["status", "priority", "label", "assignee"] as const;

/** Every key a query can set. `ready` is last because it is the odd one: a
 *  boolean, and the only key that may be written without a value. */
export const SEARCH_KEYS = [...VALUE_KEYS, "ready"] as const;

export type SearchKey = (typeof SEARCH_KEYS)[number];

/** What each key filters on, shown beside it in the suggestion list. */
export const KEY_HINTS: Record<SearchKey, string> = {
  status: "column",
  priority: "level",
  label: "whole label",
  assignee: "substring",
  ready: "unblocked only",
};

/** The one key whose value may be left out: `ready` alone means `ready=true`. */
const READY = "ready";

const TRUE_WORDS = new Set(["", "true", "yes", "y", "1", "on"]);

/** A filter set that constrains nothing — what an empty query parses to. */
export const NO_FILTERS: Filters = {
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
  text: "",
};

/**
 * One whitespace-separated term of a query, and where it sits in the line.
 *
 * The offsets are what autocomplete replaces: a suggestion accepted swaps this
 * span for a whole term, so the rest of the query — before and after — is
 * untouched.
 */
export interface Token {
  start: number;
  /** One past the last character, so `query.slice(start, end)` is the term. */
  end: number;
  /** Exactly as typed, quotes included. */
  raw: string;
  /** The key, lowercased, or "" for free text. */
  key: SearchKey | "";
  /** The value with its quotes removed; the whole term for free text. */
  value: string;
}

function isSearchKey(name: string): name is SearchKey {
  return (SEARCH_KEYS as readonly string[]).includes(name);
}

/** Quotes are delimiters, never content: they are dropped wherever they are. */
function unquote(text: string): string {
  return text.replaceAll('"', "");
}

/** The offset of the `=` that splits a term, or -1 when there is none outside
 *  quotes — which is what keeps `"priority=urgent"` free text. */
function splitAt(raw: string): number {
  let quoted = false;
  for (let at = 0; at < raw.length; at++) {
    const character = raw[at];
    if (character === '"') quoted = !quoted;
    else if (character === "=" && !quoted) return at;
  }
  return -1;
}

/** Whether a term opens with a quote, the form that makes a key or a bare
 *  `ready` ordinary text again. */
function isQuoted(raw: string): boolean {
  return raw.startsWith('"');
}

function readToken(query: string, start: number, end: number): Token {
  const raw = query.slice(start, end);
  const at = splitAt(raw);
  if (at > 0) {
    const key = unquote(raw.slice(0, at)).toLowerCase();
    if (isSearchKey(key)) return { start, end, raw, key, value: unquote(raw.slice(at + 1)) };
  }
  // The bare form of the one boolean key. Quoted, it is text like any other.
  if (!isQuoted(raw) && raw.toLowerCase() === READY) {
    return { start, end, raw, key: READY, value: "true" };
  }
  return { start, end, raw, key: "", value: unquote(raw) };
}

/**
 * Splits a query into terms. Whitespace separates them, except inside double
 * quotes, which is the whole reason this is a scan rather than a `split`.
 *
 * An unbalanced quote runs to the end of the line, so a value being typed —
 * `assignee="agent ` — is one term while it is half-written rather than two.
 */
export function tokenize(query: string): Token[] {
  const tokens: Token[] = [];
  let start = -1;
  let quoted = false;

  for (let at = 0; at <= query.length; at++) {
    const character = query[at];
    if (character === '"') quoted = !quoted;
    const boundary = at === query.length || (!quoted && /\s/.test(character ?? ""));
    if (boundary) {
      if (start >= 0) tokens.push(readToken(query, start, at));
      start = -1;
      continue;
    }
    if (start < 0) start = at;
  }
  return tokens;
}

/** Whether a `ready` term is on. The key alone, and every ordinary word for
 *  yes, mean on; anything else — `ready=false` above all — means off. */
function parseReady(value: string): boolean {
  return TRUE_WORDS.has(value.trim().toLowerCase());
}

/**
 * Reads a query into the filter set it stands for.
 *
 * Every term the query does not carry comes back empty, because the query *is*
 * the filter state: deleting `status=todo` has to clear the status filter, not
 * leave the last one standing.
 *
 * A key repeated keeps the last one — there is one slot per field, and the
 * newest thing typed is the one meant. An unknown key is not an error: it is
 * free text, so `oidc=` in a title still finds the task. Free text found in
 * several places is joined back with single spaces and matched as one phrase.
 */
export function parseQuery(query: string): Filters {
  const filters: Filters = { ...NO_FILTERS };
  const text: string[] = [];

  for (const token of tokenize(query)) {
    switch (token.key) {
      case "":
        if (token.value !== "") text.push(token.value);
        break;
      case "ready":
        filters.ready = parseReady(token.value);
        break;
      default:
        filters[token.key] = token.value;
    }
  }

  filters.text = text.join(" ");
  return filters;
}

/** Wraps a value the tokenizer would otherwise split, or lose entirely. */
function quoteValue(value: string): string {
  const clean = unquote(value);
  return clean === "" || /\s/.test(clean) ? `"${clean}"` : clean;
}

/** Free text needs quoting only when it would read back as something else —
 *  a word carrying an `=`, or the bare word `ready`. */
function quoteText(text: string): string {
  const clean = unquote(text);
  return tokenize(clean).some((token) => token.key !== "") ? `"${clean}"` : clean;
}

/**
 * Writes a filter set back out as a query.
 *
 * The canonical order is free text first, then the keys in `SEARCH_KEYS` order.
 * It is only ever reached for by the component when the query and the filters
 * have actually drifted apart — a select moved — so what the user typed is
 * never rewritten under their cursor while they are typing it.
 */
export function formatQuery(filters: Filters): string {
  const parts: string[] = [];
  const text = filters.text.trim();
  if (text !== "") parts.push(quoteText(text));

  for (const key of VALUE_KEYS) {
    const value = filters[key].trim();
    if (value !== "") parts.push(`${key}=${quoteValue(value)}`);
  }
  if (filters.ready) parts.push(READY);

  return parts.join(" ");
}

/** Whether two filter sets say the same thing — how the component knows the
 *  query and the selects still agree, and that neither has to be rewritten. */
export function equalFilters(a: Filters, b: Filters): boolean {
  return (
    a.status === b.status &&
    a.priority === b.priority &&
    a.assignee === b.assignee &&
    a.label === b.label &&
    a.ready === b.ready &&
    a.text === b.text
  );
}

// ── Autocomplete ────────────────────────────────────────────────

/** One value a key can take, as the project declares it. */
export interface Option {
  value: string;
  /** The project's display name, when it has one that differs. */
  display?: string;
}

/**
 * Where suggested values come from. Every one of them is real: the columns and
 * priorities the project declares, the labels it declares or its tasks carry,
 * and the assignees on the tasks themselves. `ready` is the exception, being
 * the one closed set the board owns.
 */
export type Sources = Record<Exclude<SearchKey, "ready">, Option[]>;

const READY_OPTIONS: Option[] = [{ value: "true" }, { value: "false" }];

export interface Suggestion {
  kind: "key" | "value";
  /** What the list shows. */
  label: string;
  /** Secondary text: the display name of a value, or what a key filters on. */
  detail: string;
  /** The whole term the query gets when this is accepted. */
  insert: string;
}

/**
 * What could go where the caret is, and the span it would replace.
 *
 * An empty `suggestions` is the ordinary case rather than a failure: it is what
 * happens while free text is being typed, and it is what closes the menu.
 */
export interface Completion {
  start: number;
  end: number;
  suggestions: Suggestion[];
}

function keySuggestions(prefix: string): Suggestion[] {
  const wanted = unquote(prefix).toLowerCase();
  return SEARCH_KEYS.filter((key) => key.startsWith(wanted)).map((key) => ({
    kind: "key" as const,
    label: key === READY ? key : `${key}=`,
    detail: KEY_HINTS[key],
    insert: key === READY ? key : `${key}=`,
  }));
}

function valueSuggestions(key: SearchKey, prefix: string, sources: Sources): Suggestion[] {
  const options = key === READY ? READY_OPTIONS : sources[key];
  const wanted = prefix.trim().toLowerCase();
  const matches = options.filter(
    (option) =>
      option.value.toLowerCase().includes(wanted) || (option.display ?? "").toLowerCase().includes(wanted),
  );

  // What the value starts with first: a prefix is what someone typing means,
  // and a substring match is the wider net cast behind it.
  const ranked = [
    ...matches.filter((option) => option.value.toLowerCase().startsWith(wanted)),
    ...matches.filter((option) => !option.value.toLowerCase().startsWith(wanted)),
  ];

  return ranked.map((option) => ({
    kind: "value" as const,
    label: option.value,
    detail: option.display && option.display !== option.value ? option.display : "",
    insert: `${key}=${quoteValue(option.value)}`,
  }));
}

/**
 * What to offer for the term the caret is in.
 *
 * The caret decides which term is being edited, and the whole term is what a
 * suggestion replaces — so a value accepted halfway through a half-typed one
 * leaves a well-formed term rather than the two halves spliced together.
 */
export function completeQuery(query: string, caret: number, sources: Sources): Completion {
  const position = Math.max(0, Math.min(caret, query.length));
  const token = tokenize(query).find(
    (candidate) => position >= candidate.start && position <= candidate.end,
  );
  if (token === undefined) {
    return { start: position, end: position, suggestions: keySuggestions("") };
  }

  const at = splitAt(token.raw);
  const key = at > 0 ? unquote(token.raw.slice(0, at)).toLowerCase() : "";
  if (isSearchKey(key)) {
    return { start: token.start, end: token.end, suggestions: valueSuggestions(key, token.value, sources) };
  }
  return { start: token.start, end: token.end, suggestions: keySuggestions(token.raw) };
}

/**
 * The query a suggestion accepted leaves, and where the caret lands in it.
 *
 * A key stops at its `=`, so the value suggestions for it come up on the very
 * same keystroke. Anything that finishes a term gets a space after it, unless
 * the query already has one there.
 */
export function applyCompletion(
  query: string,
  completion: Completion,
  suggestion: Suggestion,
): { query: string; caret: number } {
  const before = query.slice(0, completion.start);
  const after = query.slice(completion.end);
  const finished = !suggestion.insert.endsWith("=");
  const spaced = finished && !/^\s/.test(after);
  const inserted = spaced ? `${suggestion.insert} ` : suggestion.insert;
  return { query: before + inserted + after, caret: before.length + inserted.length };
}

/**
 * The search bar's query language, taken away from the DOM.
 *
 * One line of text stands for a whole `Filters`: `parseQuery` reads the line
 * into one, and that is the only direction there is. The line is the input and
 * the filter set is what it parses to, so there is nothing to keep in step
 * (TQ-0098).
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
 *   auth                     free text: a word the id, title or body carries
 *   auth token               two words, both of which have to be carried
 *   "auth token"             quoted, so the phrase has to be carried whole
 *   status=todo              a structured term; the key is case-insensitive
 *   assignee="agent api"     double quotes hold a value with spaces in it
 *   ready                    a bare word, the one key that needs no value
 *   ready=false              …and its explicit form
 *   -auth                    negated: the word must not appear
 *   -"auth token"            negated phrase
 *   -priority=low            negated term: that value is excluded
 *   -ready                   the same as ready=false
 *   "priority=urgent"        quoted, so it is free text rather than a term
 *   "-auth"                  quoted, so the dash is a character rather than a no
 *
 * Values are matched case-insensitively, so a hand-typed `priority=Urgent`
 * finds what the autocomplete's `priority=urgent` finds.
 */

import type { ExcludedTerm, Filters, TextTerm } from "./board";

/**
 * A key that constrains a field of a task, as opposed to the board's own
 * readiness rule. Only these can be excluded, since only these have a value.
 *
 * Taken from `ExcludedTerm` rather than declared again, so the list below and
 * what board.ts filters on cannot drift apart.
 */
export type ValueKey = ExcludedTerm["key"];

/** The keys that take a value, in the order the suggestion menu offers them. */
const VALUE_KEYS: readonly ValueKey[] = ["status", "priority", "label", "assignee"];

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

/** What turns a term into its opposite, written in front of it. */
export const NOT = "-";

const TRUE_WORDS = new Set(["", "true", "yes", "y", "1", "on"]);

/**
 * A filter set that constrains nothing — what an empty query parses to.
 *
 * Frozen, arrays included. It is a shared constant, and `{ ...NO_FILTERS }`
 * copies the object while *aliasing* the two arrays, so one push into a copy's
 * `text` would quietly change what "constrains nothing" means everywhere else.
 * Frozen, that push throws where it is written instead of somewhere else later.
 */
export const NO_FILTERS: Readonly<Filters> = Object.freeze({
  status: "",
  priority: "",
  assignee: "",
  label: "",
  ready: false,
  text: frozenEmpty<TextTerm>(),
  excluded: frozenEmpty<ExcludedTerm>(),
});

/** An empty array nothing can push into, still typed as the mutable array the
 *  shape asks for — the shape is what every other filter set has to satisfy. */
function frozenEmpty<T>(): T[] {
  const empty: T[] = [];
  Object.freeze(empty);
  return empty;
}

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
  /** Exactly as typed, quotes and any leading `-` included. */
  raw: string;
  /** True when the term opened with an unquoted `-`. */
  negated: boolean;
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

/** Whether a term opens with a quote, the form that makes a key, a bare `ready`
 *  or a leading `-` ordinary text again. */
function isQuoted(raw: string): boolean {
  return raw.startsWith('"');
}

/** The `-` in front of a term, and the term without it. A quoted term has none:
 *  `"-x"` searches for a dash, and that is the only way to search for one. */
function signOf(raw: string): { negated: boolean; body: string } {
  if (!raw.startsWith(NOT)) return { negated: false, body: raw };
  return { negated: true, body: raw.slice(NOT.length) };
}

function readToken(query: string, start: number, end: number): Token {
  const raw = query.slice(start, end);
  const { negated, body } = signOf(raw);

  const at = splitAt(body);
  if (at > 0) {
    const key = unquote(body.slice(0, at)).toLowerCase();
    if (isSearchKey(key)) return { start, end, raw, negated, key, value: unquote(body.slice(at + 1)) };
  }
  // The bare form of the one boolean key. Quoted, it is text like any other.
  if (!isQuoted(body) && body.toLowerCase() === READY) {
    return { start, end, raw, negated, key: READY, value: "true" };
  }
  return { start, end, raw, negated, key: "", value: unquote(body) };
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
 * A positive key repeated keeps the last one — there is one slot per field, and
 * the newest thing typed is the one meant. Negated keys are a list instead:
 * excluding two labels at once is a sensible thing to ask for. An unknown key is
 * not an error: it is free text, so `oidc=` in a title still finds the task.
 *
 * Free text is a term per bare word, all of which have to match — what a search
 * box is expected to do. A quoted run stays one phrase, which is how a phrase is
 * asked for now that a space no longer means one.
 */
export function parseQuery(query: string): Filters {
  const filters: Filters = { ...NO_FILTERS, text: [], excluded: [] };

  for (const token of tokenize(query)) {
    const value = token.value.trim();
    if (token.key === "") {
      // An empty term is nothing at all — including the lone `-` that a
      // negation is halfway through being typed as.
      if (value !== "") filters.text.push({ value, negated: token.negated });
      continue;
    }
    if (token.key === READY) {
      const on = parseReady(token.value);
      filters.ready = token.negated ? !on : on;
      continue;
    }
    if (token.negated) {
      if (value !== "") filters.excluded.push({ key: token.key, value });
      continue;
    }
    filters[token.key] = value;
  }

  return filters;
}

/** Wraps a value the tokenizer would otherwise split, or lose entirely. */
function quoteValue(value: string): string {
  const clean = unquote(value);
  return clean === "" || /\s/.test(clean) ? `"${clean}"` : clean;
}

/** Two values that constrain the board identically. Case is not one of the ways
 *  they can differ, because nothing matches case-sensitively (see board.ts). */
function sameValue(a: string, b: string): boolean {
  return a.trim().toLowerCase() === b.trim().toLowerCase();
}

function sameText(a: TextTerm[], b: TextTerm[]): boolean {
  return (
    a.length === b.length &&
    a.every((term, at) => term.negated === b[at]!.negated && sameValue(term.value, b[at]!.value))
  );
}

function sameExcluded(a: ExcludedTerm[], b: ExcludedTerm[]): boolean {
  return (
    a.length === b.length &&
    a.every((term, at) => term.key === b[at]!.key && sameValue(term.value, b[at]!.value))
  );
}

/** Whether two filter sets hide the same cards. Case is not one of the ways
 *  they can differ, for the reason `sameValue` gives above. */
export function equalFilters(a: Filters, b: Filters): boolean {
  return (
    sameValue(a.status, b.status) &&
    sameValue(a.priority, b.priority) &&
    sameValue(a.assignee, b.assignee) &&
    sameValue(a.label, b.label) &&
    a.ready === b.ready &&
    sameText(a.text, b.text) &&
    sameExcluded(a.excluded, b.excluded)
  );
}

/**
 * Whether two filter sets are the same down to the spelling — the guard on the
 * one write the search bar makes to the shared filter set.
 *
 * Two predicates, deliberately not one. `equalFilters` above ignores case, which
 * is right for "do these two hide the same cards" and wrong here: the whole
 * point of `canonicalValues` is to replace a hand-typed value with the project's
 * own spelling, and a guard that called the correction equal would skip it and
 * leave the filter set on what was typed. Only the fields `canonicalValues`
 * rewrites are compared exactly — a trimmed assignee would otherwise be tidied
 * up under whoever was typing it.
 */
export function sameFilters(a: Filters, b: Filters): boolean {
  return equalFilters(a, b) && CANONICAL_KEYS.every((key) => a[key] === b[key]);
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
export type Sources = Record<ValueKey, Option[]>;

const READY_OPTIONS: Option[] = [{ value: "true" }, { value: "false" }];

/**
 * The fields the project declares an exact list of, and so the only ones worth
 * correcting the case of. The assignee filter is a substring of a freeform name:
 * there is nothing to be canonical about, and rewriting it would tidy up what
 * someone was still typing.
 */
const CANONICAL_KEYS = ["status", "priority", "label"] as const;

/**
 * Replaces a mis-cased value with the project's own spelling of it, so the
 * filter set holds the vocabulary's own words rather than whatever was typed.
 *
 * The query line itself is left exactly as typed: correcting it under the cursor
 * is the one thing this must not do.
 */
export function canonicalValues(filters: Filters, sources: Sources): Filters {
  const corrected: Filters = { ...filters };
  for (const key of CANONICAL_KEYS) {
    corrected[key] =
      sources[key].find((option) => sameValue(option.value, filters[key]))?.value ?? filters[key];
  }
  return corrected;
}

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

/** The suggestions carry the `-` of the term being completed, so accepting one
 *  finishes the negation rather than quietly dropping it. */
function keySuggestions(prefix: string, sign: string): Suggestion[] {
  const wanted = unquote(prefix).toLowerCase();
  return SEARCH_KEYS.filter((key) => key.startsWith(wanted)).map((key) => ({
    kind: "key" as const,
    label: sign + (key === READY ? key : `${key}=`),
    detail: KEY_HINTS[key],
    insert: sign + (key === READY ? key : `${key}=`),
  }));
}

function valueSuggestions(
  key: SearchKey,
  prefix: string,
  sources: Sources,
  sign: string,
): Suggestion[] {
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
    insert: `${sign}${key}=${quoteValue(option.value)}`,
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
    return { start: position, end: position, suggestions: keySuggestions("", "") };
  }

  // A lone `-` is not a term yet, but it is a negation being typed, so the keys
  // come up for it with their dash already on.
  const sign = token.raw.startsWith(NOT) ? NOT : "";
  const body = token.raw.slice(sign.length);

  const at = splitAt(body);
  const key = at > 0 ? unquote(body.slice(0, at)).toLowerCase() : "";
  if (isSearchKey(key)) {
    return {
      start: token.start,
      end: token.end,
      suggestions: valueSuggestions(key, token.value, sources, sign),
    };
  }
  return { start: token.start, end: token.end, suggestions: keySuggestions(body, sign) };
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

// ── The address bar ─────────────────────────────────────────────

/**
 * The query is kept in the URL rather than in storage (TQ-0068): a filtered
 * board is then something to send someone, and surviving a reload is the same
 * mechanism rather than a second one. The board has no router, and this is not
 * the start of one — it is one parameter, read once and written with
 * `replaceState` so typing does not fill the back button with keystrokes.
 *
 * Both halves take and return strings so they stay testable without a browser;
 * the component is what touches `location` and `history`.
 */
export const QUERY_PARAM = "q";

/** The query an address carries, or "" when it carries none. */
export function queryFromURL(url: string): string {
  try {
    return new URL(url).searchParams.get(QUERY_PARAM) ?? "";
  } catch {
    return "";
  }
}

/**
 * The same address with the query written into it, as the relative reference
 * `replaceState` wants. An empty query drops the parameter rather than leaving
 * `?q=` behind, so a board with nothing typed into it has a clean address.
 */
export function urlWithQuery(url: string, query: string): string {
  let next: URL;
  try {
    next = new URL(url);
  } catch {
    return url;
  }
  if (query.trim() === "") next.searchParams.delete(QUERY_PARAM);
  else next.searchParams.set(QUERY_PARAM, query);
  return next.pathname + next.search + next.hash;
}

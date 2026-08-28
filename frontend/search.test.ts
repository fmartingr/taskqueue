import { describe, expect, test } from "bun:test";
import type { Filters } from "./board";
import {
  applyCompletion,
  completeQuery,
  equalFilters,
  formatQuery,
  parseQuery,
  tokenize,
  NO_FILTERS,
  type Sources,
} from "./search";

const filters = (overrides: Partial<Filters>): Filters => ({ ...NO_FILTERS, ...overrides });

/** Stands in for a project's vocabularies and the assignees its tasks carry. */
const SOURCES: Sources = {
  status: [
    { value: "inbox", display: "Inbox" },
    { value: "todo", display: "To do" },
    { value: "in-progress", display: "In Progress" },
    { value: "done", display: "Done" },
  ],
  priority: [
    { value: "urgent", display: "Urgent" },
    { value: "high", display: "High" },
    { value: "normal", display: "Normal" },
  ],
  label: [
    { value: "bug", display: "Bug" },
    { value: "component/api", display: "Component | API" },
  ],
  assignee: [{ value: "agent-api" }, { value: "agent ui" }],
};

describe("tokenize", () => {
  test("splits on whitespace and records where each term is", () => {
    const tokens = tokenize("auth status=todo");
    expect(tokens.map((token) => [token.start, token.end, token.key, token.value])).toEqual([
      [0, 4, "", "auth"],
      [5, 16, "status", "todo"],
    ]);
  });

  test("a quoted value keeps its spaces", () => {
    expect(tokenize('assignee="agent ui"')).toEqual([
      { start: 0, end: 19, raw: 'assignee="agent ui"', key: "assignee", value: "agent ui" },
    ]);
  });

  test("an unbalanced quote runs to the end, so a half-typed value is one term", () => {
    const tokens = tokenize('assignee="agent u');
    expect(tokens).toHaveLength(1);
    expect(tokens[0]!.value).toBe("agent u");
  });

  test("runs of whitespace are not terms", () => {
    expect(tokenize("   auth   ").map((token) => token.value)).toEqual(["auth"]);
    expect(tokenize("")).toEqual([]);
  });
});

describe("parseQuery", () => {
  test("an empty query constrains nothing", () => {
    expect(parseQuery("")).toEqual(NO_FILTERS);
    expect(parseQuery("   ")).toEqual(NO_FILTERS);
  });

  test("reads each structured key", () => {
    expect(parseQuery("status=todo priority=urgent label=bug assignee=agent-api")).toEqual(
      filters({ status: "todo", priority: "urgent", label: "bug", assignee: "agent-api" }),
    );
  });

  test("keys are case-insensitive; values are taken verbatim", () => {
    expect(parseQuery("PRIORITY=Urgent")).toEqual(filters({ priority: "Urgent" }));
  });

  test("bare ready means ready, and ready=false means not", () => {
    expect(parseQuery("ready").ready).toBe(true);
    expect(parseQuery("ready=true").ready).toBe(true);
    expect(parseQuery("ready=yes").ready).toBe(true);
    expect(parseQuery("ready=").ready).toBe(true);
    expect(parseQuery("ready=false").ready).toBe(false);
    expect(parseQuery("ready=no").ready).toBe(false);
  });

  test("everything else is free text, joined as one phrase", () => {
    expect(parseQuery("global search bar").text).toBe("global search bar");
  });

  test("an unknown key is free text, kept whole", () => {
    expect(parseQuery("owner=nobody")).toEqual(filters({ text: "owner=nobody" }));
  });

  test("a key with no value is no constraint", () => {
    expect(parseQuery("priority=")).toEqual(NO_FILTERS);
  });

  test("a key repeated keeps the last one", () => {
    expect(parseQuery("priority=high priority=urgent").priority).toBe("urgent");
  });

  test("quotes hold a value with spaces", () => {
    expect(parseQuery('assignee="agent ui"').assignee).toBe("agent ui");
  });

  test("a quoted term is text, whatever it looks like", () => {
    expect(parseQuery('"priority=urgent"')).toEqual(filters({ text: "priority=urgent" }));
    expect(parseQuery('"ready"')).toEqual(filters({ text: "ready" }));
  });

  test("text and terms mix in any order", () => {
    expect(parseQuery("status=todo oidc ready")).toEqual(
      filters({ status: "todo", text: "oidc", ready: true }),
    );
  });
});

describe("formatQuery", () => {
  test("an empty filter set is an empty query", () => {
    expect(formatQuery(NO_FILTERS)).toBe("");
  });

  test("writes text first, then the keys in order", () => {
    const query = formatQuery(
      filters({ text: "oidc", status: "todo", priority: "urgent", label: "bug", ready: true }),
    );
    expect(query).toBe("oidc status=todo priority=urgent label=bug ready");
  });

  test("quotes a value with spaces in it", () => {
    expect(formatQuery(filters({ assignee: "agent ui" }))).toBe('assignee="agent ui"');
  });

  test("quotes text that would read back as a term", () => {
    expect(formatQuery(filters({ text: "ready" }))).toBe('"ready"');
    expect(formatQuery(filters({ text: "status=todo" }))).toBe('"status=todo"');
  });

  test("round-trips every filter set back to itself", () => {
    const sets = [
      NO_FILTERS,
      filters({ status: "todo", ready: true }),
      filters({ text: "two words", assignee: "agent ui" }),
      filters({ text: "ready", priority: "urgent" }),
      filters({ text: "status=todo" }),
      filters({ label: "component/api", text: "oidc login" }),
    ];
    for (const set of sets) expect(parseQuery(formatQuery(set))).toEqual(set);
  });
});

describe("equalFilters", () => {
  test("compares every field", () => {
    expect(equalFilters(NO_FILTERS, { ...NO_FILTERS })).toBe(true);
    expect(equalFilters(NO_FILTERS, filters({ text: "a" }))).toBe(false);
    expect(equalFilters(NO_FILTERS, filters({ ready: true }))).toBe(false);
    expect(equalFilters(filters({ status: "todo" }), filters({ status: "done" }))).toBe(false);
  });
});

describe("completeQuery", () => {
  const complete = (query: string, caret = query.length) => completeQuery(query, caret, SOURCES);
  const labels = (query: string, caret?: number) =>
    complete(query, caret).suggestions.map((suggestion) => suggestion.label);

  test("an empty query offers every key", () => {
    expect(labels("")).toEqual(["status=", "priority=", "label=", "assignee=", "ready"]);
  });

  test("a prefix narrows the keys", () => {
    expect(labels("pri")).toEqual(["priority="]);
    expect(labels("re")).toEqual(["ready"]);
  });

  test("free text that matches no key suggests nothing", () => {
    expect(labels("oidc")).toEqual([]);
  });

  test("a fixed key offers its values, from the project", () => {
    expect(labels("status=")).toEqual(["inbox", "todo", "in-progress", "done"]);
    expect(labels("priority=")).toEqual(["urgent", "high", "normal"]);
    expect(labels("label=")).toEqual(["bug", "component/api"]);
    expect(labels("assignee=")).toEqual(["agent-api", "agent ui"]);
    expect(labels("ready=")).toEqual(["true", "false"]);
  });

  test("values are matched on the value or its display name, prefixes first", () => {
    // "done" and "todo" both carry "do"; the one that starts with it comes first.
    expect(labels("status=do")).toEqual(["done", "todo"]);
    expect(labels("label=API")).toEqual(["component/api"]);
  });

  test("a value with spaces is suggested quoted", () => {
    const suggestions = complete("assignee=agent").suggestions;
    expect(suggestions.map((suggestion) => suggestion.insert)).toEqual([
      "assignee=agent-api",
      'assignee="agent ui"',
    ]);
  });

  test("the span it replaces is the whole term the caret is in", () => {
    const completion = complete("oidc status=to ready", 14);
    expect([completion.start, completion.end]).toEqual([5, 14]);
    expect(completion.suggestions.map((suggestion) => suggestion.label)).toEqual(["todo"]);
  });

  test("a caret in whitespace starts a new term", () => {
    const completion = complete("oidc ", 5);
    expect([completion.start, completion.end]).toEqual([5, 5]);
    expect(completion.suggestions).toHaveLength(5);
  });
});

describe("applyCompletion", () => {
  const accept = (query: string, caret = query.length, at = 0) => {
    const completion = completeQuery(query, caret, SOURCES);
    return applyCompletion(query, completion, completion.suggestions[at]!);
  };

  test("a key stops at its equals, so the values come up next", () => {
    expect(accept("pri")).toEqual({ query: "priority=", caret: 9 });
  });

  test("a value finishes the term and leaves a space to type in", () => {
    expect(accept("priority=urg")).toEqual({ query: "priority=urgent ", caret: 16 });
  });

  test("it replaces only its own term", () => {
    expect(accept("oidc status=to ready", 14)).toEqual({
      query: "oidc status=todo ready",
      caret: 16,
    });
  });

  test("bare ready is accepted whole", () => {
    expect(accept("rea")).toEqual({ query: "ready ", caret: 6 });
  });
});
